# Importar el .env directamente (Make lo lee como variables propias)
include .env

# Variables
APP_NAME=motor-de-pago-api

# Objetivo por defecto
all: build

# Compilar el proyecto
build:
	go build -o $(APP_NAME) ./cmd/main.go

# Ejecutar el proyecto
run:
	go run ./cmd/main.go

# Ejecutar el linter
lint:
	golangci-lint run


clean:
	del /q /f $(APP_NAME).exe

migrate-up:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir db/migrations postgres "$(DATABASE_URL)" down