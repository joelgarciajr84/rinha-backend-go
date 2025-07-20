package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"rinha/adapter"
	"rinha/handler"
	"rinha/model"
	"rinha/utils"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelInfo)

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        512,
			MaxIdleConnsPerHost: 128,
			IdleConnTimeout:     120 * time.Second,
			MaxConnsPerHost:     512,
			DialContext: (&net.Dialer{
				Timeout:   time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
		Timeout: 5 * time.Second,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: utils.GetEnvOrDefault("REDIS_ADDR", "localhost:6379"),
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("Redis failed", "err", err)
		os.Exit(1)
	}

	retryQueue := make(chan model.PaymentRequestProcessor, 10000)
	adapter := adapter.NewPaymentProcessorAdapter(
		client,
		rdb,
		utils.GetEnvOrDefault("PAYMENT_PROCESSOR_URL_DEFAULT", "http://localhost:8001"),
		utils.GetEnvOrDefault("PAYMENT_PROCESSOR_URL_FALLBACK", "http://localhost:8002"),
		retryQueue,
		900,
	)

	handler := handler.NewPaymentHandler(adapter)

	app := fiber.New(fiber.Config{
		JSONEncoder: sonic.Marshal,
		JSONDecoder: sonic.Unmarshal,
	})

	app.Post("/payments", handler.Process)
	app.Get("/payments-summary", handler.Summary)
	app.Post("/purge-payments", handler.Purge)

	adapter.StartWorkers()
	adapter.EnableHealthCheck(utils.GetEnvOrDefault("MONITOR_HEALTH", "true"))

	port := utils.GetEnvOrDefault("PORT", "9999")
	banner := `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣤⣶⣶⣶⣤⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⣿⠟⠋⠙⠻⣿⣿⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⡇⠀🐔⠀ ⢸⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠘⣿⣿⣄⣀⣤⣾⣿⠟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⠛⠛⠋⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀

💥 RINHA DE BACKEND 2025 💥
🥊 Lutador: Golang Beast v1
🧠 Estratégia: Fallback, retry e muita mutexada
📡 Arena aberta na porta ` + port + `

👉 Que comece o massacre dos milisegundos!
`

	slog.Info(banner)

	slog.Info("server running", "port", port)
	if err := app.Listen(":" + port); err != nil {
		slog.Error("server failed", "err", err)
	}
}
