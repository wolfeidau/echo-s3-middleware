package s3middleware

import (
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
)

// FilesConfig defines the config for the middleware
type FilesConfig struct {
	// Skipper defines a function to skip middleware
	echomiddleware.Skipper
	// Profile The profile used to configure the aws client
	Profile string
	// Region The region used to configure the aws client
	Region string
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

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if fs.config.Skipper(c) {
				return next(c)
			}

			path := c.Request().URL.Path

			if c.Request().Method != "GET" {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid request method:", c.Request().Method, "for path:", path)
			}

			res, err := fs.s3svc.GetObject(&s3.GetObjectInput{
				Bucket: aws.String(s3Bucket),
				Key:    aws.String(path),
			})
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					case s3.ErrCodeNoSuchKey:
						return echo.NewHTTPError(http.StatusNotFound, "document not found:", path)
					}
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "invalid request method:", c.Request().Method, "for path:", path)
			}

			c.Response().Header().Set("ETag", aws.StringValue(res.ETag))
			c.Response().Header().Set("Last-Modified", aws.TimeValue(res.LastModified).Format(time.RFC3339))

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
