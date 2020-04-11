# echo-s3-middleware

This [echo](https://echo.labstack.com/) middleware provides a static file store backed by S3.

# Configuration

This echo middleware has a few configuration options which are passed to the s3 client.

* **Region** - This region used to access AWS.
* **Profile** - This profile used to access AWS.

**Note:** The normal `AWS_PROFILE` and `AWS_REGION` variables are supported, these are detected by the [AWS Go SDK](https://aws.amazon.com/sdk-for-go/) out of the box.

So with a configuration of the following:

```go
e := echo.New()
e.Pre(echomiddleware.AddTrailingSlash()) // required to ensure trailing slash is appended

fs := s3middleware.New(s3middleware.RedirectConfig{
  Region: "us-east-1",
  Profile: "someprofile",
})

// serve static files from the supplied bucket
e.Use(fs.StaticBucket("somebucket"))
```

# License

This application is released under Apache 2.0 license and is copyright [Mark Wolfe](https://www.wolfe.id.au).