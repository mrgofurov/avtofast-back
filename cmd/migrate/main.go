package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"sort"
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
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Error("Failed to connect to database for migrations", err)
		os.Exit(1)
	}
	defer db.Close()

	pattern := "migrations/*.up.sql"
	if *direction == "down" {
		pattern = "migrations/*.down.sql"
	}

	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		log.Error("No migration files found for pattern: "+pattern, err)
		os.Exit(1)
	}

	sort.Strings(files)
	if *direction == "down" {
		// Reverse sort for rollback
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	for _, file := range files {
		log.Info("Applying migration: " + file)
		sqlContent, err := os.ReadFile(file)
		if err != nil {
			log.Error("Failed to read migration file: "+file, err)
			os.Exit(1)
		}

		_, err = db.Pool.Exec(ctx, string(sqlContent))
		if err != nil {
			log.Error("Migration execution failed on "+file, err)
			os.Exit(1)
		}
	}

	log.Info("All migrations applied successfully!")
}
