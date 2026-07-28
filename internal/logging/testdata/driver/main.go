package main

import (
	"log/slog"
	"os"

	"github.com/mrcook1e/amneziawg-panel/internal/logging"
)

func main() {
	logger, err := logging.New(os.Stdout, logging.Config{
		Format: os.Getenv("LOG_FORMAT"),
		Level:  os.Getenv("LOG_LEVEL"),
	})
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("logging configuration invalid",
			slog.String("component", "logging"),
			slog.Any("error", err),
		)
		os.Exit(1)
	}
	logger.Info("logging driver ready", slog.String("component", "logging"))
}
