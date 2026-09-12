package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
	"github.com/avtofast/avtofast-back/pkg/logger"
)

func main() {
	direction := flag.String("dir", "up", "migration direction: up or down")
	flag.Parse()

	log := logger.DefaultLogger
	log.Info("Running migrations with direction: " + *direction)

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Error("Failed to load config", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Error("Failed to connect to database for migrations", err)
		return
	}
	defer db.Close()

	var file string
	if *direction == "down" {
		file = "migrations/000001_init_schema.down.sql"
	} else {
		file = "migrations/000001_init_schema.up.sql"
	}

	sqlContent, err := os.ReadFile(file)
	if err != nil {
		log.Error("Failed to read migration file: "+file, err)
		return
	}

	_, err = db.Pool.Exec(ctx, string(sqlContent))
	if err != nil {
		log.Error("Migration execution failed", err)
		return
	}

	log.Info("Migrations applied successfully!")
}
