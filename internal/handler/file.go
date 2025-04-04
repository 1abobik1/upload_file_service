package handler

import (
	"context"
	"io"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"github.com/1abobik1/upload_file_service/internal/apperrors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileService interface {
	Upload(ctx context.Context, filename string, data []byte) (string, uint64, error)
	DownloadLink(ctx context.Context, fileID string) (string, error)
	ListFiles(ctx context.Context) ([]*pb.FileInfo, error)
	DownloadZip(ctx context.Context, fileIDs []string) (io.ReadCloser, error)
	Update(ctx context.Context, fileID string, data []byte) (string, uint64, error)
}

type FileHandler struct {
	pb.UnimplementedFileServiceServer
	service FileService
}

func NewFileHandler(svc FileService) *FileHandler {
	return &FileHandler{service: svc}
}

func (h *FileHandler) Upload(stream pb.FileService_UploadServer) error {
	const op = "location internal/handler/Upload()"

	firstChunk, err := stream.Recv()
	if err != nil {
		logrus.WithError(err).Errorf("Failed to receive first chunk in %s", op)
		return status.Error(codes.InvalidArgument, "failed to receive first chunk")
	}

	filename := firstChunk.GetFilename()
	if filename == "" {
		logrus.Errorf("%s: Filename is empty in first chunk", op)
		return status.Error(codes.InvalidArgument, "filename is required in first chunk")
	}

	data, err := readUploadStream(stream)
	if err != nil {
		logrus.WithError(err).Errorf("%s: Error receiving update chunk", op)
		return status.Errorf(codes.Internal, "failed to read file data: %v", err)
	}

	fileID, size, err := h.service.Upload(stream.Context(), filename, data)
	if err != nil {
		logrus.WithError(err).Errorf("%s: Failed to upload file in service", op)
		return apperrors.MapErrorToStatus(err)
	}

	logrus.Infof("%s: Uploaded file: %s (%d bytes) as ID: %s", op, filename, size, fileID)

	return stream.SendAndClose(&pb.UploadResponse{
		FileId: fileID,
		Size:   size,
	})
}

func (h *FileHandler) UpdateFile(stream pb.FileService_UpdateFileServer) error {
	const op = "location internal/handler/UpdateFile()"

	firstChunk, err := stream.Recv()
	if err != nil {
		logrus.WithError(err).Errorf("Failed to receive first chunk in %s", op)
		return status.Error(codes.InvalidArgument, "failed to receive first chunk")
	}

	fileID := firstChunk.GetFileId()
	if fileID == "" {
		logrus.Errorf("FileID is empty in %s first chunk", op)
		return status.Error(codes.InvalidArgument, "file_id is required in first chunk")
	}

	data, err := readUpdateStream(stream)
	if err != nil {
		logrus.WithError(err).Errorf("%s: Error receiving update chunk", op)
		return status.Errorf(codes.Internal, "failed to read file data: %v", err)
	}

	newFileID, newSize, err := h.service.Update(stream.Context(), fileID, data)
	if err != nil {
		logrus.WithError(err).Errorf("%s: Update failed", op)
		return apperrors.MapErrorToStatus(err)
	}

	logrus.Infof("%s: Updated file: %s → %s (%d bytes)", op, fileID, newFileID, newSize)

	return stream.SendAndClose(&pb.UpdateFileResponse{
		FileId:  newFileID,
		NewSize: newSize,
	})
}

func (h *FileHandler) GetDownloadLink(ctx context.Context, req *pb.DownloadLinkRequest) (*pb.DownloadLinkResponse, error) {
	const op = "location internal/handler/GetDownloadLink()"

	if req.GetFileId() == "" {
		logrus.Errorf("%s: file_id is required", op)
		return nil, status.Error(codes.InvalidArgument, "file_id is required")
	}

	url, err := h.service.DownloadLink(ctx, req.GetFileId())
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to generate URL for file %s", op, req.GetFileId())
		return nil, apperrors.MapErrorToStatus(err)
	}

	logrus.Infof("%s: generated URL for file %s", op, req.GetFileId())
	return &pb.DownloadLinkResponse{Url: url}, nil
}

func (h *FileHandler) ListFiles(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	const op = "location internal/handler/ListFiles()"

	files, err := h.service.ListFiles(ctx)
	if err != nil {
		logrus.WithError(err).Errorf("%s: failed to list files", op)
		return nil, apperrors.MapErrorToStatus(err)
	}

	logrus.Infof("%s: returned %d files", op, len(files))
	return &pb.ListResponse{Files: files}, nil
}

func (h *FileHandler) DownloadZip(req *pb.DownloadZipRequest, stream pb.FileService_DownloadZipServer) error {
	const op = "location internal/handler/DownloadZip()"

	if len(req.GetFileIds()) == 0 {
		logrus.Errorf("%s: at least one file_id is required", op)
		return status.Error(codes.InvalidArgument, "at least one file_id is required")
	}

	readCloser, err := h.service.DownloadZip(stream.Context(), req.GetFileIds())
	if err != nil {
		logrus.WithError(err).Errorf("%s: zip creation failed", op)
		return apperrors.MapErrorToStatus(err)
	}
	defer readCloser.Close()

	// Отправляем чанки по 1MB
	buf := make([]byte, 1<<20)
	for {
		n, err := readCloser.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.WithError(err).Errorf("%s: read error", op)
			return status.Error(codes.Internal, "read error: "+err.Error())
		}

		if err := stream.Send(&pb.DownloadZipResponse{Chunk: buf[:n]}); err != nil {
			logrus.WithError(err).Errorf("%s: failed to send chunk", op)
			return err
		}
	}

	logrus.Infof("%s: zip streaming completed successfully", op)
	return nil
}
