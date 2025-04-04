package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type GRPCConfig struct {
	Port                    string        `env:"GRPC_PORT" env-required:"true"`
	MaxConcurrentStreams    int           `env:"GRPC_MAX_CONCURRENT_STREAMS" env-required:"true"`
	FileOpsConcurrencyLimit int           `env:"GRPC_CLIENT_FILE_OPS_CONCURRENCY_LIMIT"  env-required:"true"`
	ListConcurrencyLimit    int           `env:"GRPC_CLIENT_LIST_CONCURRENCY_LIMIT"  env-required:"true"`
	ShutdownTimeout         time.Duration `env:"GRPC_SHUTDOWN_TIMEOUT"  env-required:"true"`
}

type MinIOConfig struct {
	Endpoint          string `env:"MINIO_PORT" env-required:"true"`
	MinIoRootUser     string `env:"MINIO_ROOT_USER"  env-required:"true"`
	MinIoRootPassword string `env:"MINIO_ROOT_PASSWORD"  env-required:"true"`
	Bucket            string `env:"MINIO_BUCKET" env-required:"true"`
	UseSSL            bool   `env:"MINIO_USE_SSL" env-required:"true"`
}

type Config struct {
	GRPC  GRPCConfig
	MinIO MinIOConfig
}

func MustLoad() *Config {
	path := getConfigPath()

	// Загружаем .env файл если он существует
	if _, err := os.Stat(path); err == nil {
		if err := godotenv.Load(path); err != nil {
			panic(fmt.Sprintf("error loading .env file: %v", err))
		}
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic(fmt.Sprintf("failed to read env vars: %v", err))
	}

	return &cfg
}

func getConfigPath() string {
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}

	var res string
	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res != "" {
		return res
	}

	return ".env" // default path
}
