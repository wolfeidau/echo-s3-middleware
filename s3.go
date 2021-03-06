package s3middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
)

var (
	errNotFound = errors.New("file not found")
)

// FilesConfig defines the config for the middleware
type FilesConfig struct {
	// Skipper defines a function to skip middleware
	echomiddleware.Skipper
	// Region The region used to configure the aws client
	Region string
	// HeaderXRequestID Name of the request id header to include in callbacks, defaults to echo.HeaderXRequestID
	HeaderXRequestID string
	// Enable SPA mode by forwarding all not-found requests to root so that
	// SPA (single-page application) can handle the routing.
	SPA bool
	// Index file for serving a directory in SPA mode.
	Index string
	// Summary provides a callback which provide a summary of what was successfully processed by s3
	Summary func(ctx context.Context, evt map[string]interface{})
	// OnErr is called if there is an issue processing the s3 request
	OnErr func(ctx context.Context, err error)
	// CacheHeaders is called prior to writing enabling customisation of cache control headers
	CacheHeaders func(ctx context.Context, fileInfo FileInfo) string
	// S3API the s3 service used to download assets
	S3API s3iface.S3API
}

// FileInfo provided to callbacks to enable cache header selection
type FileInfo struct {
	ID            string
	Bucket        string
	Key           string
	Name          string
	Etag          string
	LastModified  time.Time
	ContentLength int64
}

// FilesStore manages the s3 client
type FilesStore struct {
	config FilesConfig
}

// New create a new FilesStore backed by s3
func New(config FilesConfig) *FilesStore {
	return &FilesStore{config: config}
}

// StaticBucket new static file server using the supplied s3 bucket
func (fs *FilesStore) StaticBucket(s3Bucket string) echo.MiddlewareFunc {

	if fs.config.Skipper == nil {
		fs.config.Skipper = echomiddleware.DefaultSkipper
	}

	if fs.config.Summary == nil {
		fs.config.Summary = func(context.Context, map[string]interface{}) {} // NOOP
	}

	if fs.config.OnErr == nil {
		fs.config.OnErr = func(context.Context, error) {} // NOOP
	}

	if fs.config.HeaderXRequestID == "" {
		fs.config.HeaderXRequestID = echo.HeaderXRequestID
	}

	if fs.config.Index == "" {
		fs.config.Index = "index.html"
	}

	if fs.config.CacheHeaders == nil {
		fs.config.CacheHeaders = CacheNothing
	}

	if fs.config.S3API == nil {
		fs.config.S3API = buildS3API(fs.config)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if fs.config.Skipper(c) {
				return next(c)
			}

			ctx := c.Request().Context()

			id := c.Request().Header.Get(fs.config.HeaderXRequestID)
			if id == "" {
				id = c.Response().Header().Get(fs.config.HeaderXRequestID)
			}

			if c.Request().Method != "GET" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request method: %s path: %s", c.Request().Method, c.Request().URL.Path))
			}

			paths := fs.buildPaths(c)

			for _, path := range paths {
				contentType, body, err := fs.file(c, s3Bucket, id, path)
				if err == errNotFound {
					continue // try the next path
				}
				if err != nil {
					fs.config.OnErr(ctx, errors.Wrapf(err, "failed to process s3 request path: %s id: %s", path, id))
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to process request")
				}
				defer body.Close()
				return c.Stream(http.StatusOK, contentType, body)
			}

			// neither path was found
			return echo.NewHTTPError(http.StatusNotFound, "document not found:", c.Request().URL.Path)

		}
	}
}

func (fs *FilesStore) file(c echo.Context, s3Bucket, id, name string) (string, io.ReadCloser, error) {
	ctx := c.Request().Context()

	start := time.Now()
	res, err := fs.config.S3API.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				return "", nil, errNotFound
			}
		}
		return "", nil, err
	}

	stop := time.Now()

	fs.config.Summary(ctx, map[string]interface{}{
		"id":            id,
		"bucket":        s3Bucket,
		"key":           name,
		"etag":          aws.StringValue(res.ETag),
		"last_modified": aws.TimeValue(res.LastModified).Format(time.RFC3339),
		"contentlength": aws.Int64Value(res.ContentLength),
		"latency":       stop.Sub(start),
		"latency_human": stop.Sub(start).String(),
	})

	// force browsers to avoid caching this data
	c.Response().Header().Set("Cache-Control", fs.config.CacheHeaders(ctx, FileInfo{
		ID:            id,
		Name:          name,
		Bucket:        s3Bucket,
		Etag:          aws.StringValue(res.ETag),
		LastModified:  aws.TimeValue(res.LastModified),
		ContentLength: aws.Int64Value(res.ContentLength),
	}))

	// add this information to help with troubleshooting
	c.Response().Header().Set("ETag", aws.StringValue(res.ETag))
	c.Response().Header().Set("Last-Modified", aws.TimeValue(res.LastModified).Format(time.RFC3339))

	// we rely on s3 for content type of objects
	// return c.Stream(http.StatusOK, aws.StringValue(res.ContentType), res.Body)
	return aws.StringValue(res.ContentType), res.Body, nil
}

func (fs *FilesStore) buildPaths(c echo.Context) []string {

	// if we let "/" key through to s3 it will return a xml directory listing for a GetObject call.
	if c.Request().URL.Path == "/" {
		return []string{filepath.Join("/", fs.config.Index)}
	}

	p := []string{c.Request().URL.Path}

	if fs.config.SPA {
		p = append(p, filepath.Join("/", fs.config.Index))
	}

	return p
}

// CacheNothing default cache header function which caches nothing
func CacheNothing(ctx context.Context, fileInfo FileInfo) string {
	return "no-store, no-cache, must-revalidate, post-check=0, pre-check=0"
}

func buildS3API(config FilesConfig) s3iface.S3API {
	awsCfg := buildAwsConfig(config) // update the region / profile

	sess := session.Must(session.NewSession(awsCfg))
	return s3.New(sess)
}

func buildAwsConfig(config FilesConfig) *aws.Config {
	awsCfg := &aws.Config{}

	if config.Region != "" {
		awsCfg = awsCfg.WithRegion(config.Region)
	}

	return awsCfg
}
