package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"github.com/1abobik1/upload_file_service/internal/handler"
	"github.com/1abobik1/upload_file_service/internal/service"
	"github.com/1abobik1/upload_file_service/tests/integration/testutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// -------------------- Тесты API --------------------

// TestGRPCHandler проверяет метод Upload через gRPC стрим
func TestGRPCHandler(t *testing.T) {
	store := testutils.SetupMinIO(t)
	defer testutils.CleanupMinIO(t, store)

	svc := service.NewFileService(store, store.Bucket)
	h := handler.NewFileHandler(svc)

	t.Run("Upload file", func(t *testing.T) {
		stream := &mockUploadStream{
			requests: []*pb.UploadRequest{
				{Data: &pb.UploadRequest_Filename{Filename: "test.txt"}},
				{Data: &pb.UploadRequest_Chunk{Chunk: []byte("data")}},
			},
			ctx: context.Background(),
		}

		err := h.Upload(stream)
		require.NoError(t, err)
		require.NotEmpty(t, stream.response.GetFileId(), "Ответ должен содержать fileID")
	})
}

// TestDownloadLink проверяет получение URL для скачивания файла
func TestDownloadLink(t *testing.T) {
	store := testutils.SetupMinIO(t)
	defer testutils.CleanupMinIO(t, store)

	svc := service.NewFileService(store, store.Bucket)
	h := handler.NewFileHandler(svc)

	ctx := context.Background()
	filename := "download_test.txt"
	content := []byte("download test content")

	fileID, size, err := svc.Upload(ctx, filename, content)
	require.NoError(t, err)
	require.NotEmpty(t, fileID)
	require.Equal(t, uint64(len(content)), size)

	req := &pb.DownloadLinkRequest{FileId: fileID}
	resp, err := h.GetDownloadLink(ctx, req)
	require.NoError(t, err)
	require.Contains(t, resp.GetUrl(), fileID, "URL для скачивания должен содержать fileID")
}

// TestListFiles проверяет получение списка загруженных файлов
func TestListFiles(t *testing.T) {
	store := testutils.SetupMinIO(t)
	defer testutils.CleanupMinIO(t, store)

	svc := service.NewFileService(store, store.Bucket)
	h := handler.NewFileHandler(svc)

	ctx := context.Background()
	_, _, err := svc.Upload(ctx, "file1.txt", []byte("content1"))
	require.NoError(t, err)
	_, _, err = svc.Upload(ctx, "file2.txt", []byte("content2"))
	require.NoError(t, err)

	listReq := &pb.ListRequest{}
	listResp, err := h.ListFiles(ctx, listReq)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listResp.GetFiles()), 2, "Должно быть не менее 2 файлов")
}

// TestDownloadZip проверяет формирование zip-архива из загруженных файлов
func TestDownloadZip(t *testing.T) {
	store := testutils.SetupMinIO(t)
	defer testutils.CleanupMinIO(t, store)

	svc := service.NewFileService(store, store.Bucket)
	h := handler.NewFileHandler(svc)
	ctx := context.Background()

	expectedSubstr1 := "file1.txt"
	expectedSubstr2 := "file2.txt"

	fileID1, _, err := svc.Upload(ctx, expectedSubstr1, []byte("content1"))
	require.NoError(t, err)
	fileID2, _, err := svc.Upload(ctx, expectedSubstr2, []byte("content2"))
	require.NoError(t, err)

	zipReq := &pb.DownloadZipRequest{FileIds: []string{fileID1, fileID2}}
	stream := &mockDownloadZipStream{ctx: ctx}

	err = h.DownloadZip(zipReq, stream)
	require.NoError(t, err)

	zipData := stream.buf.Bytes()
	require.NotEmpty(t, zipData, "Zip-данные не должны быть пустыми")

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	require.NoError(t, err)

	var foundFile1, foundFile2 bool
	for _, f := range zipReader.File {
		t.Logf("Найден файл в архиве: %s", f.Name)
		// проверка, что имя файла содержит ожидаемую подстроку
		if strings.Contains(f.Name, expectedSubstr1) {
			foundFile1 = true
			rc, err := f.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(rc)
			rc.Close()
			require.NoError(t, err)
			require.Equal(t, "content1", string(data))
		}
		if strings.Contains(f.Name, expectedSubstr2) {
			foundFile2 = true
			rc, err := f.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(rc)
			rc.Close()
			require.NoError(t, err)
			require.Equal(t, "content2", string(data))
		}
	}

	if !foundFile1 || !foundFile2 {
		var names []string
		for _, f := range zipReader.File {
			names = append(names, f.Name)
		}
		t.Logf("Файлы в архиве: %v", names)
	}
	require.True(t, foundFile1, "Архив должен содержать "+expectedSubstr1)
	require.True(t, foundFile2, "Архив должен содержать "+expectedSubstr2)
}

// TestUploadPhoto проверяет загрузку реальной фотографии
func TestUploadPhoto(t *testing.T) {
	store := testutils.SetupMinIO(t)
	defer testutils.CleanupMinIO(t, store)

	svc := service.NewFileService(store, store.Bucket)
	ctx := context.Background()

	photoPath := "photos_tests/test_photo.jpg"
	photoData, err := os.ReadFile(photoPath)
	require.NoError(t, err)
	require.NotEmpty(t, photoData)

	fileID, size, err := svc.Upload(ctx, "test_photo.jpg", photoData)
	require.NoError(t, err)
	require.Greater(t, size, uint64(0))
	require.NotEmpty(t, fileID)

	downloadLink, err := svc.DownloadLink(ctx, fileID)
	require.NoError(t, err)
	require.Contains(t, downloadLink, fileID, "Ссылка для скачивания должна содержать fileID")
}

func TestUpdateFile(t *testing.T) {
    store := testutils.SetupMinIO(t)
    defer testutils.CleanupMinIO(t, store)

    svc := service.NewFileService(store, store.Bucket)
    h := handler.NewFileHandler(svc)
    ctx := context.Background()

    originalContent := []byte("original content")
    fileID, _, err := svc.Upload(ctx, "test_update.txt", originalContent)
    require.NoError(t, err)

    files, err := svc.ListFiles(ctx)
    require.NoError(t, err)
    for _, f := range files {
        if f.FileId == fileID {
            break
        }
    }

    t.Run("Successful update", func(t *testing.T) {
        newContent := []byte("updated content with new data")
        
        stream := &mockUpdateStream{
            requests: []*pb.UpdateFileRequest{
                {Data: &pb.UpdateFileRequest_FileId{FileId: fileID}},
                {Data: &pb.UpdateFileRequest_Chunk{Chunk: newContent}},
            },
            ctx: ctx,
        }

        err := h.UpdateFile(stream)
        require.NoError(t, err)
        require.Equal(t, fileID, stream.response.GetFileId())
        require.Equal(t, uint64(len(newContent)), stream.response.GetNewSize())

        reader, err := store.GetObject(ctx, store.Bucket, fileID)
        require.NoError(t, err)
        defer reader.Close()

        data, err := io.ReadAll(reader)
        require.NoError(t, err)
        require.Equal(t, newContent, data)
    })

    t.Run("Update non-existent file", func(t *testing.T) {
        invalidID := "nonexistent-file-id"
        stream := &mockUpdateStream{
            requests: []*pb.UpdateFileRequest{
                {Data: &pb.UpdateFileRequest_FileId{FileId: invalidID}},
                {Data: &pb.UpdateFileRequest_Chunk{Chunk: []byte("data")}},
            },
            ctx: ctx,
        }

        err := h.UpdateFile(stream)
        require.Error(t, err)
        st, ok := status.FromError(err)
        require.True(t, ok)
        require.Equal(t, codes.NotFound, st.Code())
    })
}

// -------------------- Моки --------------------

// mockDownloadZipStream эмулирует gRPC стрим для DownloadZip
type mockDownloadZipStream struct {
	ctx context.Context
	buf bytes.Buffer
	pb.FileService_DownloadZipServer
}

func (m *mockDownloadZipStream) Send(resp *pb.DownloadZipResponse) error {
	m.buf.Write(resp.Chunk)
	return nil
}

func (m *mockDownloadZipStream) Context() context.Context {
	return m.ctx
}

func (m *mockDownloadZipStream) SendMsg(msg interface{}) error {
	return nil
}

func (m *mockDownloadZipStream) RecvMsg(msg interface{}) error {
	return nil
}

// mockUploadStream для тестирования метода Upload
type mockUploadStream struct {
	requests []*pb.UploadRequest
	response *pb.UploadResponse
	ctx      context.Context
	pb.FileService_UploadServer 
}

func (m *mockUploadStream) Recv() (*pb.UploadRequest, error) {
	if len(m.requests) == 0 {
		return nil, io.EOF
	}
	req := m.requests[0]
	m.requests = m.requests[1:]
	return req, nil
}

func (m *mockUploadStream) SendAndClose(resp *pb.UploadResponse) error {
	m.response = resp
	return nil
}

func (m *mockUploadStream) Context() context.Context {
	return m.ctx
}

func (m *mockUploadStream) RecvMsg(msg interface{}) error {
	req, err := m.Recv()
	if err != nil {
		return err
	}
	clonedReq := proto.Clone(req).(*pb.UploadRequest)
	proto.Merge(msg.(proto.Message), clonedReq)
	return nil
}

func (m *mockUploadStream) SendMsg(msg interface{}) error {
	resp, ok := msg.(*pb.UploadResponse)
	if !ok {
		return status.Error(codes.Internal, "failed to cast response")
	}
	return m.SendAndClose(resp)
}

// mockUpdateStream для тестирования метода UpdateFile
type mockUpdateStream struct {
    requests []*pb.UpdateFileRequest
    response *pb.UpdateFileResponse
    ctx      context.Context
    pb.FileService_UpdateFileServer
}

func (m *mockUpdateStream) Recv() (*pb.UpdateFileRequest, error) {
    if len(m.requests) == 0 {
        return nil, io.EOF
    }
    req := m.requests[0]
    m.requests = m.requests[1:]
    return req, nil
}

func (m *mockUpdateStream) SendAndClose(resp *pb.UpdateFileResponse) error {
    m.response = resp
    return nil
}

func (m *mockUpdateStream) Context() context.Context {
    return m.ctx
}

func (m *mockUpdateStream) RecvMsg(msg interface{}) error {
    req, err := m.Recv()
    if err != nil {
        return err
    }
    clonedReq := proto.Clone(req).(*pb.UpdateFileRequest)
    proto.Merge(msg.(proto.Message), clonedReq)
    return nil
}

func (m *mockUpdateStream) SendMsg(msg interface{}) error {
    resp, ok := msg.(*pb.UpdateFileResponse)
    if !ok {
        return status.Error(codes.Internal, "failed to cast response")
    }
    return m.SendAndClose(resp)
}