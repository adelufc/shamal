package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"shamal/service/internal/httpapi"
	"shamal/service/internal/platform"
	"shamal/service/internal/seed"
)

func main() {
	loadDotEnv()
	cfg := platform.LoadConfig()
	ctx := context.Background()
	pool, err := platform.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := platform.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}
	if err := seed.Run(ctx, pool); err != nil {
		log.Fatal(err)
	}
	if err := seed.EnsureCommercial(ctx, pool); err != nil {
		log.Fatal(err)
	}
	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	rdb := redis.NewClient(options)
	api := httpapi.New(pool, rdb, cfg)
	go syncNeighborhoods(ctx, api)
	log.Printf("shamal service listening on http://%s", cfg.HTTPAddr)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func syncNeighborhoods(ctx context.Context, api *httpapi.API) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := api.Redis.BRPop(ctx, 2*time.Second, "queue:geo.sync").Result()
		if err == nil && len(result) == 2 {
			_ = api.Geo.CompleteJob(ctx, result[1])
		}
		ids, err := api.Geo.RecordedJobs(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		for _, id := range ids {
			_ = api.Geo.CompleteJob(ctx, id)
		}
	}
}

func loadDotEnv() {
	for _, path := range []string{".env", "../.env"} {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			key, value, _ := strings.Cut(line, "=")
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		file.Close()
	}
}
