package handler

import (
	"io"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"github.com/1abobik1/upload_file_service/internal/apperrors"
)

// чтение чанков и их объединение в единый срез
func readUploadStream(stream interface {
	Recv() (*pb.UploadRequest, error)
}) ([]byte, error) {
	var data []byte
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filename := req.GetFilename(); filename != "" {
			return nil, apperrors.ErrFilenameProvidedTwice
		}
		data = append(data, req.GetChunk()...)
	}
	return data, nil
}

// чтение чанков и их объединения в единый срез
func readUpdateStream(stream interface {
	Recv() (*pb.UpdateFileRequest, error)
}) ([]byte, error) {
	var data []byte
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if fileID := req.GetFileId(); fileID != "" {
			return nil, apperrors.ErrFileIDProvidedTwice
		}
		data = append(data, req.GetChunk()...)
	}
	return data, nil
}
