package services

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hackclub/hackatime/config"
)

type ObjectStorageService struct {
	config   *config.Config
	s3Client *s3.Client
}

func NewObjectStorageService() *ObjectStorageService {
	cfg := config.Get()

	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.ObjectStorage.Endpoint),
		Region:       cfg.ObjectStorage.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.ObjectStorage.AccessKeyId, cfg.ObjectStorage.SecretAccessKey, ""),
	})

	return &ObjectStorageService{
		config:   cfg,
		s3Client: s3Client,
	}
}

func (s *ObjectStorageService) Upload(key string, data io.ReadSeeker, contentType string) (string, error) {
	_, err := s.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.config.ObjectStorage.Bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/%s", s.config.ObjectStorage.Endpoint, s.config.ObjectStorage.Bucket, key)
	return url, nil
}
