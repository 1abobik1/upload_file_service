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

	firstChunk, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive first chunk")
	}

	filename := firstChunk.GetFilename()
	if filename == "" {
		return status.Error(codes.InvalidArgument, "filename is required in first chunk")
	}

	data, err := readUploadStream(stream)
	if err != nil {
		return apperrors.MapErrorToStatus(err)
	}

	fileID, size, err := h.service.Upload(stream.Context(), filename, data)
	if err != nil {
		return apperrors.MapErrorToStatus(err)
	}

	return stream.SendAndClose(&pb.UploadResponse{
		FileId: fileID,
		Size:   size,
	})
}

func (h *FileHandler) UpdateFile(stream pb.FileService_UpdateFileServer) error {

	firstChunk, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive first chunk")
	}

	fileID := firstChunk.GetFileId()
	if fileID == "" {
		return status.Error(codes.InvalidArgument, "file_id is required in first chunk")
	}

	data, err := readUpdateStream(stream)
	if err != nil {
		return apperrors.MapErrorToStatus(err)
	}

	newFileID, newSize, err := h.service.Update(stream.Context(), fileID, data)
	if err != nil {
		return apperrors.MapErrorToStatus(err)
	}

	return stream.SendAndClose(&pb.UpdateFileResponse{
		FileId:  newFileID,
		NewSize: newSize,
	})
}

func (h *FileHandler) GetDownloadLink(ctx context.Context, req *pb.DownloadLinkRequest) (*pb.DownloadLinkResponse, error) {

	if req.GetFileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "file_id is required")
	}

	url, err := h.service.DownloadLink(ctx, req.GetFileId())
	if err != nil {
		return nil, apperrors.MapErrorToStatus(err)
	}

	return &pb.DownloadLinkResponse{Url: url}, nil
}

func (h *FileHandler) ListFiles(ctx context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	files, err := h.service.ListFiles(ctx)
	if err != nil {
		return nil, apperrors.MapErrorToStatus(err)
	}

	return &pb.ListResponse{Files: files}, nil
}

func (h *FileHandler) DownloadZip(req *pb.DownloadZipRequest, stream pb.FileService_DownloadZipServer) error {
	const op = "location internal/handler/DownloadZip()"

	if len(req.GetFileIds()) == 0 {
		return status.Error(codes.InvalidArgument, "at least one file_id is required")
	}

	readCloser, err := h.service.DownloadZip(stream.Context(), req.GetFileIds())
	if err != nil {
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

	return nil
}
