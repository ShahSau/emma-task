package common

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var S3Client *s3.Client

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
	// We check if AWS_ENDPOINT is set (e.g., http://localstack:4566)
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

	fmt.Println("S3 Client Initialized")
}

func GetS3() *s3.Client {
	return S3Client
}
