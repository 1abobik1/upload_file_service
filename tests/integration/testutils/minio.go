package testutils

import (
	"testing"
	"time"

	"github.com/1abobik1/upload_file_service/internal/storage"
	"github.com/stretchr/testify/require"
)

func SetupMinIO(t *testing.T) *storage.MinIOStorage {
	t.Helper()

	cfg := struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		Bucket    string
		UseSSL    bool
	}{
		Endpoint:  "test-minio:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test-bucket-" + time.Now().Format("20060102-150405"),
		UseSSL:    false,
	}

	// Инициализация клиента
	store, err := storage.NewMinIOStorage(
		cfg.Endpoint,
		cfg.AccessKey,
		cfg.SecretKey,
		cfg.Bucket,
		cfg.UseSSL,
	)
	require.NoError(t, err)

	return store
}

func CleanupMinIO(t *testing.T, store *storage.MinIOStorage) {
	t.Helper()
}
