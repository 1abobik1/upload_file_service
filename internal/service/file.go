package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"github.com/1abobik1/upload_file_service/internal/apperrors"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MetaFilename  = "Filename"
	MetaCreatedAt = "Createdat"
	MetaUpdatedAt = "Updatedat"
)

type MinIOStorageI interface {
	PutObject(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, objectSize int64, metadata map[string]string) error
	GetObject(ctx context.Context, bucket string, objectName string) (io.ReadCloser, error)
	StatObject(ctx context.Context, bucket string, objectName string) (map[string]string, time.Time, int64, error)
	ListObjects(ctx context.Context, bucket string) <-chan minio.ObjectInfo
	PresignedGetObject(ctx context.Context, bucket string, objectName string, expiry time.Duration) (*url.URL, error)
}

type FileService struct {
	storage MinIOStorageI
	bucket  string
}

func NewFileService(storage MinIOStorageI, bucket string) *FileService {
	return &FileService{
		storage: storage,
		bucket:  bucket,
	}
}

func (s *FileService) Upload(ctx context.Context, filename string, data []byte) (string, uint64, error) {
	const op = "location internal/service/Upload()"

	contentType := http.DetectContentType(data)
	ext := getExtensionFromMIME(contentType)

	// на случай, если клиент по какой-то причине не указал расширение файла в названии
	originalExt := filepath.Ext(filename)
	if originalExt == "" && ext != "" {
		filename = fmt.Sprintf("%s%s", filename, ext)
	}

	fileID := generateFileID(filename, ext)

	now := time.Now().Format(time.RFC3339)
	err := s.storage.PutObject(
		ctx,
		s.bucket,
		fileID,
		contentType,
		bytes.NewReader(data),
		int64(len(data)),
		map[string]string{
			MetaFilename:  filename,
			MetaCreatedAt: now,
			MetaUpdatedAt: now,
		},
	)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to put object", op)
		return "", 0, fmt.Errorf("%w", errors.Join(apperrors.ErrStorageFailure, err))
	}

	return fileID, uint64(len(data)), nil
}

func (s *FileService) Update(ctx context.Context, fileID string, data []byte) (string, uint64, error) {
	const op = "location internal/service/Update()"

	metadata, _, _, err := s.storage.StatObject(ctx, s.bucket, fileID)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to get file metadata", op)
		if minioErr, ok := err.(minio.ErrorResponse); ok && minioErr.Code == "NoSuchKey" {
			return "", 0, apperrors.ErrFileNotFound
		}
		return "", 0, fmt.Errorf("failed to get file metadata: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	metadata[MetaUpdatedAt] = now

	contentType := http.DetectContentType(data)

	err = s.storage.PutObject(
		ctx,
		s.bucket,
		fileID,
		contentType,
		bytes.NewReader(data),
		int64(len(data)),
		metadata,
	)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to update object", op)
		return "", 0, fmt.Errorf("%w", errors.Join(apperrors.ErrStorageFailure, err))
	}

	return fileID, uint64(len(data)), nil
}

func (s *FileService) DownloadLink(ctx context.Context, fileID string) (string, error) {
	const op = "location internal/service/DownloadLink()"

	_, _, _, err := s.storage.StatObject(ctx, s.bucket, fileID)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to get file metadata", op)
		if minioErr, ok := err.(minio.ErrorResponse); ok && minioErr.Code == "NoSuchKey" {
			return "", apperrors.ErrFileNotFound
		}
		return "", fmt.Errorf("failed to get file metadata: %w", err)
	}

	url, err := s.storage.PresignedGetObject(ctx, s.bucket, fileID, time.Hour)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to generate presigned URL", op)
		return "", fmt.Errorf("%w", errors.Join(apperrors.ErrStorageFailure, err))
	}

	return url.String(), nil
}

func (s *FileService) ListFiles(ctx context.Context) ([]*pb.FileInfo, error) {
	const op = "location internal/service/ListFiles()"

	var files []*pb.FileInfo

	for obj := range s.storage.ListObjects(ctx, s.bucket) {
		if obj.Err != nil {
			logrus.WithError(obj.Err).Warnf("%s: skipping object due to error", op)
			continue
		}

		metadata, _, size, err := s.storage.StatObject(ctx, s.bucket, obj.Key)
		if err != nil {
			logrus.WithError(err).Warnf("%s: failed to stat object %s", op, obj.Key)
			continue
		}

		createdAt, _ := time.Parse(time.RFC3339, metadata[MetaCreatedAt])
		updatedAt, _ := time.Parse(time.RFC3339, metadata[MetaUpdatedAt])

		files = append(files, &pb.FileInfo{
			FileId:    obj.Key,
			Filename:  metadata[MetaFilename],
			CreatedAt: timestamppb.New(createdAt),
			UpdatedAt: timestamppb.New(updatedAt),
			Size:      uint64(size),
		})
	}

	return files, nil
}

func (s *FileService) DownloadZip(ctx context.Context, fileIDs []string) (io.ReadCloser, error) {
	const op = "location internal/service/DownloadZip()"

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for _, fileID := range fileIDs {
		obj, err := s.storage.GetObject(ctx, s.bucket, fileID)
		if err != nil {
			logrus.WithError(err).Warnf("%s: failed to get object %s, skipping", op, fileID)
			continue
		}

		metadata, _, _, err := s.storage.StatObject(ctx, s.bucket, fileID)
		if err != nil {
			obj.Close()
			continue
		}

		filename := metadata[MetaFilename]
		if filename == "" || filename == " " {
			filename = fileID
		}

		writer, err := zipWriter.Create(filename)
		if err != nil {
			logrus.WithError(err).Warnf("%s: failed to create zip entry for %s, skipping", op, fileID)
			obj.Close()
			continue
		}

		if _, err := io.Copy(writer, obj); err != nil {
			logrus.WithError(err).Warnf("%s: error copying data for %s", op, fileID)
			obj.Close()
			continue
		}
		obj.Close()
	}

	if err := zipWriter.Close(); err != nil {
		logrus.WithError(err).Errorf("%s: failed to close zip writer", op)
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func getExtensionFromMIME(contentType string) string {
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0] // Берём первое подходящее расширение
}

func generateFileID(baseName string, ext string) string {
	return fmt.Sprintf("%s-%s%s",
		strings.TrimSuffix(baseName, filepath.Ext(baseName)),
		uuid.New().String(),
		ext,
	)
}
