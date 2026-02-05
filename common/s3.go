package common

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var S3Client *s3.Client
var PresignClient *s3.PresignClient

func InitS3() {
	// Load basic config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("AWS_REGION")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		)),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	// Configure for LocalStack (if running locally)
	endpoint := os.Getenv("AWS_ENDPOINT")
	if endpoint != "" {
		S3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // Required for LocalStack
		})
	} else {
		// Real AWS
		S3Client = s3.NewFromConfig(cfg)
	}

	// Initialize Presign Client
	PresignClient = s3.NewPresignClient(S3Client)
}

func GetS3() *s3.Client {
	return S3Client
}

// GetPresignedURL generates a temporary URL to download a file
func GetPresignedURL(key string) (string, error) {
	if PresignClient == nil {
		return "", fmt.Errorf("presign client not initialized")
	}
	request, err := PresignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 1 * time.Hour // Link valid for 1 hour
	})
	if err != nil {
		return "", err
	}
	return request.URL, nil
}
