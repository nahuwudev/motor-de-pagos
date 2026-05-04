# Variables
APP_NAME=motor-de-pago-api

# Objetivo por defecto (se ejecuta si solo escribís "make")
all: build

# Compilar el proyecto
build:
	go build -o $(APP_NAME) main.go

# Ejecutar el proyecto
run:
	go run main.go

# Ejecutar el linter (¡el que acabamos de configurar!)
lint:
	golangci-lint run

# Limpiar archivos compilados
clean:
	rm -f $(APP_NAME)