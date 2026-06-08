package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store implements Store using S3-compatible object storage (AWS S3, MinIO, etc.)
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// S3Config holds S3 connection parameters
type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Prefix          string
	UseSSL          bool
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	ctx := context.TODO()
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.Region = cfg.Region
			if cfg.Endpoint != "" {
				scheme := "http"
				if cfg.UseSSL {
					scheme = "https"
				}
				o.BaseEndpoint = aws.String(fmt.Sprintf("%s://%s", scheme, cfg.Endpoint))
				o.UsePathStyle = true
			}
		},
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func (s *S3Store) objectKey(key string) string {
	if s.prefix != "" {
		return path.Join(s.prefix, key)
	}
	return key
}

func (s *S3Store) Put(key string, data io.Reader) error {
	body, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read data: %w", err)
	}
	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.objectKey(key)),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
	})
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

func (s *S3Store) Get(key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return out.Body, nil
}

func (s *S3Store) Delete(key string) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

func (s *S3Store) Exists(key string) bool {
	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	return err == nil
}
