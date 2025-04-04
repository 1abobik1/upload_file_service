package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	pb "github.com/1abobik1/proto-upload-service/gen/go/upload_service/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultChunkSize = 64 * 1024 // 64KB
	pathToNewFile    = "update_photo.jpg"
)

func main() {
	serverAddr := flag.String("server", "localhost:50051", "gRPC server address")
	filePath := flag.String("file", "", "Path to file to upload")
	insecureFlag := flag.Bool("insecure", false, "Use insecure connection")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("Please specify file path with -file flag")
	}

	var opts []grpc.DialOption
	if *insecureFlag {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		config := &tls.Config{InsecureSkipVerify: true}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	fileID, err := uploadFile(client, *filePath)
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Printf("File uploaded successfully. File ID: %s\n", fileID)

	if err := getDownloadLink(client, fileID); err != nil {
		log.Fatalf("GetDownloadLink failed: %v", err)
	}

	if err := listFiles(client); err != nil {
		log.Fatalf("ListFiles failed: %v", err)
	}

	err = updateFile(client, pathToNewFile)
	if err != nil {
		log.Fatalf("UpdateFile failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	listResp, err := client.ListFiles(ctx, &pb.ListRequest{})
	if err != nil {
		log.Fatalf("ListFiles error: %v", err)
	}
	if len(listResp.Files) == 0 {
		log.Fatalf("No files available to download")
	}

	// берём первые 3 fileID (если их меньше, берём меньше 3)
	n := 3
	if len(listResp.Files) < 3 {
		n = len(listResp.Files)
	}
	var fileIDs []string
	fmt.Println("Files on server:")
	for i := 0; i < n; i++ {
		file := listResp.Files[i]
		fmt.Printf("ID: %s, Name: %s, Size: %d\n", file.FileId, file.Filename, file.Size)
		fileIDs = append(fileIDs, file.FileId)
	}

	// Скачиваем zip-архив по полученным fileID
	outputZip := "download.zip"
	if err := downloadZip(client, fileIDs, outputZip); err != nil {
		log.Fatalf("DownloadZip error: %v", err)
	}
	fmt.Printf("Zip archive downloaded successfully to %s\n", outputZip)
}

// uploadFile выполняет потоковую загрузку файла на сервер и возвращает fileID.
func uploadFile(client pb.FileServiceClient, filePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stream, err := client.Upload(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create upload stream: %v", err)
	}

	filename := filepath.Base(filePath)
	if err := stream.Send(&pb.UploadRequest{
		Data: &pb.UploadRequest_Filename{Filename: filename},
	}); err != nil {
		return "", fmt.Errorf("failed to send filename: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, defaultChunkSize)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read file: %v", err)
		}
		if n == 0 {
			break
		}

		if err := stream.Send(&pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{Chunk: buffer[:n]},
		}); err != nil {
			return "", fmt.Errorf("failed to send chunk: %v", err)
		}
	}

	response, err := stream.CloseAndRecv()
	if err != nil {
		return "", fmt.Errorf("failed to receive response: %v", err)
	}
	return response.FileId, nil
}

func updateFile(client pb.FileServiceClient, pathToNewData string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	listResp, err := client.ListFiles(ctx, &pb.ListRequest{})
	if err != nil {
		return fmt.Errorf("failed to list files: %v", err)
	}
	if len(listResp.Files) == 0 {
		return fmt.Errorf("no files available to update")
	}

	fileID := listResp.Files[0].FileId
	fmt.Printf("Updating file with ID: %s\n", fileID)

	file, err := os.Open(pathToNewData)
	if err != nil {
		return fmt.Errorf("failed to open new file: %v", err)
	}
	defer file.Close()

	stream, err := client.UpdateFile(ctx)
	if err != nil {
		return fmt.Errorf("failed to start UpdateFile stream: %v", err)
	}

	if err := stream.Send(&pb.UpdateFileRequest{
		Data: &pb.UpdateFileRequest_FileId{FileId: fileID},
	}); err != nil {
		return fmt.Errorf("failed to send fileID: %v", err)
	}

	buf := make([]byte, defaultChunkSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading file: %v", err)
		}
		if n == 0 {
			break
		}

		if err := stream.Send(&pb.UpdateFileRequest{
			Data: &pb.UpdateFileRequest_Chunk{Chunk: buf[:n]},
		}); err != nil {
			return fmt.Errorf("failed to send chunk: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to receive update response: %v", err)
	}

	fmt.Printf("File updated successfully. New file ID: %s, New size: %d bytes\n", resp.FileId, resp.NewSize)
	return nil
}

// getDownloadLink запрашивает и выводит presigned URL для скачивания файла по fileID.
func getDownloadLink(client pb.FileServiceClient, fileID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := &pb.DownloadLinkRequest{FileId: fileID}
	resp, err := client.GetDownloadLink(ctx, req)
	if err != nil {
		return fmt.Errorf("GetDownloadLink error: %v", err)
	}
	fmt.Printf("Download Link for file %s: %s\n", fileID, resp.Url)
	return nil
}

// listFiles получает список файлов с сервера и выводит их информацию.
func listFiles(client pb.FileServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := &pb.ListRequest{}
	resp, err := client.ListFiles(ctx, req)
	if err != nil {
		return fmt.Errorf("ListFiles error: %v", err)
	}

	fmt.Println("Files on server:")
	for _, file := range resp.Files {
		createdAt := file.CreatedAt.AsTime().Format("2006-01-02 15:04:05") // YYYY-MM-DD HH:MM:SS
		updatedAt := file.UpdatedAt.AsTime().Format("2006-01-02 15:04:05")

		fmt.Printf("ID: %s, Name: %s, Size: %d bytes, Created: %s, UpdatedAt: %s\n",
			file.FileId, file.Filename, file.Size, createdAt, updatedAt)
	}
	return nil
}

func downloadZip(client pb.FileServiceClient, fileIDs []string, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req := &pb.DownloadZipRequest{FileIds: fileIDs}
	stream, err := client.DownloadZip(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call DownloadZip: %v", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error receiving chunk: %v", err)
		}
		if _, err := outFile.Write(resp.Chunk); err != nil {
			return fmt.Errorf("error writing chunk: %v", err)
		}
	}
	return nil
}