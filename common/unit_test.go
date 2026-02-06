package common

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitS3AndPresign(t *testing.T) {
	origRegion := os.Getenv("AWS_REGION")
	origKey := os.Getenv("AWS_ACCESS_KEY_ID")
	origSecret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	origBucket := os.Getenv("S3_BUCKET")
	origEndpoint := os.Getenv("AWS_ENDPOINT")

	defer func() {
		os.Setenv("AWS_REGION", origRegion)
		os.Setenv("AWS_ACCESS_KEY_ID", origKey)
		os.Setenv("AWS_SECRET_ACCESS_KEY", origSecret)
		os.Setenv("S3_BUCKET", origBucket)
		os.Setenv("AWS_ENDPOINT", origEndpoint)
	}()

	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_ACCESS_KEY_ID", "TEST_KEY_ID")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "TEST_SECRET_KEY")
	os.Setenv("S3_BUCKET", "my-test-bucket")
	os.Unsetenv("AWS_ENDPOINT")

	assert.NotPanics(t, func() {
		InitS3()
	})

	assert.NotNil(t, GetS3())
	assert.NotNil(t, PresignClient)

	key := "exports/users.csv"
	url, err := GetPresignedURL(key)

	assert.NoError(t, err)
	assert.NotEmpty(t, url)

	assert.Contains(t, url, "my-test-bucket", "URL should contain the bucket name")
	assert.Contains(t, url, "exports/users.csv", "URL should contain the file key")
	assert.Contains(t, url, "https://", "URL should be HTTPS")
	assert.Contains(t, url, "X-Amz-Signature", "URL should be signed")
}

func TestGetPresignedURL_NotInitialized(t *testing.T) {
	oldClient := PresignClient
	defer func() { PresignClient = oldClient }()

	PresignClient = nil

	url, err := GetPresignedURL("some-file.txt")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "presign client not initialized")
	assert.Empty(t, url)
}

func TestInitS3_LocalStack(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_ENDPOINT", "http://localhost:4566")
	os.Setenv("S3_BUCKET", "local-bucket")

	// Run Init
	assert.NotPanics(t, func() {
		InitS3()
	})

	url, err := GetPresignedURL("test.txt")
	assert.NoError(t, err)

	isLocal := strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")
	assert.True(t, isLocal, "LocalStack URL should point to localhost")
	assert.Contains(t, url, "local-bucket")
}
