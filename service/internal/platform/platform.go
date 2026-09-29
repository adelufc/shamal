package platform

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	RedisURL        string
	AllowApprox     bool
	Storage         string
	StorageDir      string
	S3Endpoint      string
	S3AccessKey     string
	S3SecretKey     string
	S3PublicBucket  string
	S3PrivateBucket string
	PublicOrigin    string
}

func LoadConfig() Config {
	return Config{
		HTTPAddr:        getenv("SHAMAL_HTTP_ADDR", "127.0.0.1:4318"),
		DatabaseURL:     getenv("SHAMAL_DATABASE_URL", "postgres://shamal:shamal_local_dev@127.0.0.1:55432/shamal?sslmode=disable"),
		RedisURL:        getenv("SHAMAL_REDIS_URL", "redis://127.0.0.1:16379/0"),
		AllowApprox:     getenv("SHAMAL_ALLOW_APPROXIMATE_BOUNDARY", "true") == "true",
		Storage:         getenv("SHAMAL_STORAGE", "local"),
		StorageDir:      getenv("SHAMAL_STORAGE_DIR", ".data/objects"),
		S3Endpoint:      getenv("SHAMAL_S3_ENDPOINT", "127.0.0.1:9444"),
		S3AccessKey:     getenv("SHAMAL_S3_ACCESS_KEY", "shamal"),
		S3SecretKey:     getenv("SHAMAL_S3_SECRET_KEY", "shamal_local_dev"),
		S3PublicBucket:  getenv("SHAMAL_S3_PUBLIC_BUCKET", "shamal-public"),
		S3PrivateBucket: getenv("SHAMAL_S3_PRIVATE_BUCKET", "shamal-private"),
		PublicOrigin:    getenv("SHAMAL_PUBLIC_ORIGIN", "http://127.0.0.1:4317"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type Error struct {
	Code     string
	HTTP     int
	Fields   map[string]string
	Internal error
}

func (e *Error) Error() string {
	if e.Internal != nil {
		return e.Code + ": " + e.Internal.Error()
	}
	return e.Code
}

func E(code string, http int) *Error {
	return &Error{Code: code, HTTP: http}
}

func As(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return nil
}

func Page(pageRaw, sizeRaw string) (int, int) {
	page := 1
	size := 25
	if n, err := strconv.Atoi(pageRaw); err == nil && n > 0 {
		page = n
	}
	if n, err := strconv.Atoi(sizeRaw); err == nil && n > 0 {
		size = n
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, url)
}
