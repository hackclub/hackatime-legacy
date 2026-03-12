package services

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hackclub/hackatime/config"
)

type ObjectStorageService struct {
	config        *config.Config
	s3Client      *s3.Client
	presignClient *s3.PresignClient
	uploader      *manager.Uploader
}

func NewObjectStorageService() *ObjectStorageService {
	cfg := config.Get()

	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.ObjectStorage.Endpoint),
		Region:       cfg.ObjectStorage.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.ObjectStorage.AccessKeyId, cfg.ObjectStorage.SecretAccessKey, ""),
	})

	return &ObjectStorageService{
		config:        cfg,
		s3Client:      s3Client,
		presignClient: s3.NewPresignClient(s3Client),
		uploader:      manager.NewUploader(s3Client),
	}
}

func (s *ObjectStorageService) Upload(key string, data io.Reader, contentType string) error {
	_, err := s.uploader.Upload(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.config.ObjectStorage.Bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *ObjectStorageService) GetDownloadURL(key string, expiresAt time.Time) (string, error) {
	expiresIn := time.Until(expiresAt)
	if expiresIn <= 0 {
		return "", nil
	}

	req, err := s.presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.config.ObjectStorage.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (s *ObjectStorageService) Delete(key string) error {
	_, err := s.s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.ObjectStorage.Bucket),
		Key:    aws.String(key),
	})
	return err
}
