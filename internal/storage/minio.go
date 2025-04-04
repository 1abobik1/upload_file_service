package storage

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	Client *minio.Client
	Bucket string
}

func NewTestMinIOStorage(client *minio.Client, bucket string) *MinIOStorage {
	return &MinIOStorage{
		Client: client,
		Bucket: bucket,
	}
}

func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	// Проверка существования бакета
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}
	}

	return &MinIOStorage{
		Client: client,
		Bucket: bucket,
	}, nil
}

func (s *MinIOStorage) PutObject(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, objectSize int64, metadata map[string]string) error {
	_, err := s.Client.PutObject(
		ctx,
		bucket,
		objectName,
		reader,
		objectSize,
		minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: metadata,
		},
	)
	return err
}

func (s *MinIOStorage) GetObject(ctx context.Context, bucket string, objectName string) (io.ReadCloser, error) {
	obj, err := s.Client.GetObject(
		ctx,
		bucket,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, err
	}

	// Проверка существования объекта
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, err
	}

	return obj, nil
}

func (s *MinIOStorage) StatObject(ctx context.Context, bucket string, objectName string) (map[string]string, time.Time, int64, error) {
	info, err := s.Client.StatObject(
		ctx,
		bucket,
		objectName,
		minio.StatObjectOptions{},
	)
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	return info.UserMetadata, info.LastModified, info.Size, nil
}

func (s *MinIOStorage) ListObjects(ctx context.Context, bucket string) <-chan minio.ObjectInfo {
	return s.Client.ListObjects(
		ctx,
		bucket,
		minio.ListObjectsOptions{
			Recursive: true,
		},
	)
}

func (s *MinIOStorage) PresignedGetObject(ctx context.Context, bucket string, objectName string, expiry time.Duration) (*url.URL, error) {
	return s.Client.PresignedGetObject(
		ctx,
		bucket,
		objectName,
		expiry,
		nil,
	)
}
