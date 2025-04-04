package apperrors

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrFileNotFound          = errors.New("file not found")
	ErrInvalidFileFormat     = errors.New("invalid file format")
	ErrStorageFailure        = errors.New("storage failure")
	ErrPermissionDenied      = errors.New("permission denied")
	ErrFilenameProvidedTwice = errors.New("filename must only be provided in the first chunk")
	ErrFileIDProvidedTwice   = errors.New("file_id must only be provided in the first chunk")
)

// MapErrorToStatus преобразует ошибки в безопасные gRPC-ответы
func MapErrorToStatus(err error) error {
	switch {
	case errors.Is(err, ErrFileNotFound):
		return status.Error(codes.NotFound, "file not found")
	case errors.Is(err, ErrInvalidFileFormat):
		return status.Error(codes.InvalidArgument, "invalid file format")
	case errors.Is(err, ErrStorageFailure):
		return status.Error(codes.Internal, "internal storage error")
	case errors.Is(err, ErrFilenameProvidedTwice):
		return status.Error(codes.InvalidArgument, "filename must only be provided in the first chunk")
	case errors.Is(err, ErrFileIDProvidedTwice):
		return status.Error(codes.InvalidArgument, "file_id must only be provided in the first chunk")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
