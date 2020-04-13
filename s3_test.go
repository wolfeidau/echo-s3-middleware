package s3middleware

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/mock/gomock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/wolfeidau/echo-s3-middleware/mocks"
)

func TestStatic(t *testing.T) {
	assert := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3svc := mocks.NewMockS3API(ctrl)

	r := ioutil.NopCloser(strings.NewReader("hello world"))

	s3svc.EXPECT().GetObjectWithContext(gomock.Any(), &s3.GetObjectInput{Bucket: aws.String("testbucket"), Key: aws.String("/index.html")}).Return(&s3.GetObjectOutput{Body: r}, nil)

	fs := FilesStore{s3svc: s3svc}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderXRequestID, "xglvVA0A9ONQCjJACsvY0rC1f7ypPi7g")
	rec := httptest.NewRecorder()
	e.Use(fs.StaticBucket("testbucket"))

	e.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code)
}

func TestStatic_NotFound(t *testing.T) {
	assert := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3svc := mocks.NewMockS3API(ctrl)

	s3svc.EXPECT().GetObjectWithContext(gomock.Any(), &s3.GetObjectInput{Bucket: aws.String("testbucket"), Key: aws.String("/not.html")}).
		Return(nil, awserr.New(s3.ErrCodeNoSuchKey, "testing not found", errors.New("test")))

	fs := FilesStore{s3svc: s3svc}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/not.html", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.Use(fs.StaticBucket("testbucket"))

	e.ServeHTTP(rec, req)

	// Assertions
	assert.Equal(http.StatusNotFound, rec.Code)
}

func TestStatic_InternalServerError(t *testing.T) {
	assert := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s3svc := mocks.NewMockS3API(ctrl)

	s3svc.EXPECT().GetObjectWithContext(gomock.Any(), &s3.GetObjectInput{Bucket: aws.String("testbucket"), Key: aws.String("/not.html")}).
		Return(nil, awserr.New(s3.ErrCodeNoSuchBucket, "testing internal error", errors.New("test")))

	fs := FilesStore{s3svc: s3svc}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/not.html", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.Use(fs.StaticBucket("testbucket"))

	e.ServeHTTP(rec, req)

	// Assertions
	assert.Equal(http.StatusInternalServerError, rec.Code)
}

func TestBuildAWSConfig(t *testing.T) {
	assert := require.New(t)

	awsCfg := buildAwsConfig(FilesConfig{})
	assert.Equal(&aws.Config{}, awsCfg)

	awsCfg = buildAwsConfig(FilesConfig{Region: "us-east-1"})
	assert.Equal(aws.String("us-east-1"), awsCfg.Region)
}
