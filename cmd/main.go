package main

import (
	"context"
	"fmt"
	"log"
	"main/internal/api/handler"
	"main/internal/api/middleware"
	"main/internal/config"
	"main/internal/db"
	"main/internal/idempotency"
	"main/internal/payments"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error fatal: %v", err)
	}

}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no se pudo cargar la configuración: %w", err)
	}
	log.Println("Configuración cargada correctamente.")

	ctx := context.Background()
	pool, err := db.NewPostgresConnectionPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	log.Println("Conexión a la base de datos establecida exitosamente.")
	defer pool.Close()

	redisClient, err := db.NewRedisConnectionPool(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a Redis: %w", err)
	}
	log.Println("Conexión a Redis establecida exitosamente.")
	defer redisClient.Close()

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	redisStore := idempotency.NewRedisStore(redisClient)
	sqlStore := payments.NewSQLStore(pool)
	paymentService := payments.NewPaymentService(sqlStore)

	h := handler.NewPaymentHandler(paymentService)

	mux.Handle("POST /api/v1/payments",
		middleware.IdempotencyMiddleware(redisStore)(http.HandlerFunc(h.CreateTransfer)),
	)

	log.Printf("Servidor escuchando en :%s", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ Apagando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("error en shutdown: %w", err)
	}

	log.Println("Servidor apagado correctamente.")
	return nil
}
