package s3middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
)

// FilesConfig defines the config for the middleware
type FilesConfig struct {
	// Skipper defines a function to skip middleware
	echomiddleware.Skipper
	// Profile The profile used to configure the aws client
	Profile string
	// Region The region used to configure the aws client
	Region string
	// HeaderXRequestID Name of the request id header to include in callbacks, defaults to echo.HeaderXRequestID
	HeaderXRequestID string

	// Summary provides a callback which provide a summary of what was successfully processed by s3
	Summary func(ctx context.Context, evt map[string]interface{})
	// OnErr is called if there is an issue processing the s3 request
	OnErr func(ctx context.Context, err error)
}

// FilesStore manages the s3 client
type FilesStore struct {
	s3svc  s3iface.S3API
	config FilesConfig
}

// New create a new FilesStore backed by s3
func New(config FilesConfig) *FilesStore {

	awsCfg := buildAwsConfig(config) // update the region / profile

	sess := session.Must(session.NewSession(awsCfg))
	return &FilesStore{s3svc: s3.New(sess), config: config}
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

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if fs.config.Skipper(c) {
				return next(c)
			}

			ctx := c.Request().Context()
			path := c.Request().URL.Path

			id := c.Request().Header.Get(fs.config.HeaderXRequestID)
			if id == "" {
				id = c.Response().Header().Get(fs.config.HeaderXRequestID)
			}

			if c.Request().Method != "GET" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid request method: %s path: %s", c.Request().Method, path))
			}

			start := time.Now()
			res, err := fs.s3svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s3Bucket),
				Key:    aws.String(path),
			})
			stop := time.Now()

			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					case s3.ErrCodeNoSuchKey:
						return echo.NewHTTPError(http.StatusNotFound, "document not found:", path)
					}
				}
				fs.config.OnErr(ctx, errors.Wrapf(err, "failed to process s3 request path: %s id: %s", path, id))
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to process request")
			}

			fs.config.Summary(ctx, map[string]interface{}{
				"id":            id,
				"bucket":        s3Bucket,
				"key":           path,
				"etag":          aws.StringValue(res.ETag),
				"last_modified": aws.TimeValue(res.LastModified).Format(time.RFC3339),
				"contentlength": aws.Int64Value(res.ContentLength),
				"latency":       stop.Sub(start),
				"latency_human": stop.Sub(start).String(),
			})

			// force browsers to avoid caching this data
			c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0")

			// add this information to help with troubleshooting
			c.Response().Header().Set("ETag", aws.StringValue(res.ETag))
			c.Response().Header().Set("Last-Modified", aws.TimeValue(res.LastModified).Format(time.RFC3339))

			// we rely on s3 for content type of objects
			return c.Stream(http.StatusOK, aws.StringValue(res.ContentType), res.Body)
		}
	}
}

func buildAwsConfig(config FilesConfig) *aws.Config {
	awsCfg := &aws.Config{}

	if config.Profile != "" {
		awsCfg = awsCfg.WithCredentials(credentials.NewSharedCredentials("", config.Profile))
	}

	if config.Region != "" {
		awsCfg = awsCfg.WithRegion(config.Region)
	}

	return awsCfg
}
