package logging

import (
	"log/slog"
	"os"
)

// InitLogger initializes a default structured JSON logger.
func InitLogger(serviceName string, nodeID string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler).With(
		slog.String("service", serviceName),
	)
	if nodeID != "" {
		logger = logger.With(slog.String("node_id", nodeID))
	}
	slog.SetDefault(logger)
	return logger
}
