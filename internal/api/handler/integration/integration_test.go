package integration_test

import (
	"context"
	"database/sql"
	"log"
	"main/internal/api/handler"
	"main/internal/api/middleware"
	"main/internal/db"
	"main/internal/idempotency"
	"main/internal/payments"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testServer *httptest.Server
	testPool   *pgxpool.Pool
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second),
		),
	)

	if err != nil {
		log.Fatalf("Error en creación de postgres: %v", err)
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	if err != nil {
		log.Fatalf("Error en conexión a postgres: %v", err)
	}

	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp").WithStartupTimeout(30*time.Second),
		),
	)

	if err != nil {
		log.Fatalf("Error levantando Redis: %v", err)
	}

	redisAddr, err := redisContainer.Endpoint(ctx, "")
	if err != nil {
		log.Fatalf("Error obteniendo endpoint de Redis: %v", err)
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error abriendo conexión para migraciones: %v", err)
	}

	err = goose.SetDialect("postgres")
	if err != nil {
		log.Fatalf("Error configurando goose: %v", err)
	}

	path := filepath.Join(
		"..",
		"..",
		"..",
		"..",
		"db",
		"migrations",
	)

	err = goose.Up(sqlDB, path)
	if err != nil {
		log.Fatalf("Error corriendo migraciones: %v", err)
	}

	pool, err := db.NewPostgresConnectionPool(ctx, connStr)
	if err != nil {
		log.Fatalf("Error creando pool: %v", err)
	}

	testPool = pool

	redisClient, err := db.NewRedisConnectionPool(redisAddr)
	if err != nil {
		log.Fatalf("Error creando cliente Redis: %v", err)
	}

	redisStore := idempotency.NewRedisStore(redisClient)
	sqlStore := payments.NewSQLStore(pool)
	paymentService := payments.NewPaymentService(sqlStore)
	h := handler.NewPaymentHandler(paymentService)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/payments",
		middleware.IdempotencyMiddleware(redisStore)(http.HandlerFunc(h.CreateTransfer)),
	)

	testServer = httptest.NewServer(mux)

	code := m.Run()

	testServer.Close()
	_ = postgresContainer.Terminate(ctx)
	_ = redisContainer.Terminate(ctx)

	os.Exit(code)
}
