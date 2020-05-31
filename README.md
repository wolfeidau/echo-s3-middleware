# echo-s3-middleware

This [echo](https://echo.labstack.com/) middleware provides a static file store backed by S3.

[![GitHub Actions status](https://github.com/wolfeidau/echo-s3-middleware/workflows/Go/badge.svg?branch=master)](https://github.com/wolfeidau/echo-s3-middleware/actions?query=workflow%3AGo) 
[![Go Report Card](https://goreportcard.com/badge/github.com/wolfeidau/echo-s3-middleware)](https://goreportcard.com/report/github.com/wolfeidau/echo-s3-middleware) 
[![Documentation](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/wolfeidau/echo-s3-middleware)

# Example 

```go
e := echo.New()
e.Pre(echomiddleware.AddTrailingSlash()) // required to ensure trailing slash is appended

fs := s3middleware.New(s3middleware.FilesConfig{
  Region: "us-east-1",    // can also be assigned using AWS_REGION environment variable
  SPA: true,              // enable fallback which will try Index if the first path is not found
  Index: "login.html",
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