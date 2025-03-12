// API Gateway service for authenticating and authorizing HTTP requests
// using MQTT-style permissions from PocketBase
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"api-gateway/internal/config"
	"api-gateway/internal/gateway"
)

func main() {
	// Initialize logger with initial configuration
	logger := initLogger("info")
	defer logger.Sync()

	logger.Info("Starting API Gateway service")

	// Get configuration file path
	configPath := config.GetConfigPath()

	// Load configuration
	cfg, err := config.LoadConfig(configPath, logger)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Re-initialize logger with configuration
	logger = initLogger(cfg.LogLevel)
	defer logger.Sync()

	// Create API Gateway
	gw, err := gateway.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create API Gateway", zap.Error(err))
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: gw,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	logger.Info("Shutting down gracefully...")

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shut down the server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped, goodbye!")
}

// initLogger initializes the zap logger with the specified log level
func initLogger(logLevel string) *zap.Logger {
	// Parse log level
	level := zap.InfoLevel
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		// Default to info level if invalid
		level = zap.InfoLevel
	}

	// Logger configuration
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.DisableCaller = false
	config.DisableStacktrace = false

	// Create logger
	logger, err := config.Build()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	return logger
}
