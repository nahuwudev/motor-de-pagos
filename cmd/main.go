package main

import (
	"context"
	"log"
	"main/internal/config"
	"main/internal/db"
)

func main() {
	cfg, err := config.Load()

	if err != nil {
		log.Fatalf("❌ Error fatal: no se pudo cargar la configuración: %v", err)
	}
	log.Println("✅ Configuración cargada correctamente.")

	ctx := context.Background()
	pool, err := db.NewPostgresConnectionPool(ctx, cfg.DatabaseURL)

	if err != nil {
		log.Fatalf("❌ Error fatal: no se pudo conectar a la base de datos: %v", err)
	}

	defer pool.Close()
	log.Println("✅ Conexión a la base de datos establecida exitosamente.")
}
