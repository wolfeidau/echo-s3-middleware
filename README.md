# echo-s3-middleware

This [echo](https://echo.labstack.com/) middleware provides a static file store backed by S3.

[![GitHub Actions status](https://github.com/wolfeidau/echo-s3-middleware/workflows/Go/badge.svg?branch=master)](https://github.com/wolfeidau/echo-s3-middleware/actions?query=workflow%3AGo)
[![Go Report Card](https://goreportcard.com/badge/github.com/wolfeidau/echo-s3-middleware)](https://goreportcard.com/report/github.com/wolfeidau/echo-s3-middleware)
[![Documentation](https://godoc.org/github.com/wolfeidau/echo-s3-middleware?status.svg)](https://godoc.org/github.com/wolfeidau/echo-s3-middleware)

# Configuration

This echo middleware has a few configuration options which are passed to the s3 client.

* **Region** - Optional region used to access AWS.
* **Profile** - Optional profile used to access AWS.
* **HeaderXRequestID** - Name of the request id header to include in callbacks, defaults to echo.HeaderXRequestID
* **Summary** - This provides a callback which provide a summary of what was successfully processed by s3.
* **OnErr** - This provides a callback which is invoked if there is an issue processing the s3 request.

**Note:** The normal `AWS_PROFILE` and `AWS_REGION` variables are supported, these are detected by the [AWS Go SDK](https://aws.amazon.com/sdk-for-go/) out of the box.

So with a configuration of the following:

```go
e := echo.New()
e.Pre(echomiddleware.AddTrailingSlash()) // required to ensure trailing slash is appended

fs := s3middleware.New(s3middleware.FilesConfig{
  Region: "us-east-1",    // can also be assigned using AWS_REGION environment variable
  Profile: "someprofile", // can also be assigned using AWS_PROFILE environment variable
  Summary: func(ctx context.Context, data map[string]interface{}) {
    log.Printf("processed s3 request: %+v", data)
  },
  OnErr: func(ctx context.Context, err error) {
    log.Printf("failed to process s3 request: %+v", err)
  },
})

// serve static files from the supplied bucket
e.Use(fs.StaticBucket("somebucket"))
```

# License

This code was authored by [Mark Wolfe](https://www.wolfe.id.au) and licensed under the [Apache 2.0 license](http://www.apache.org/licenses/LICENSE-2.0).