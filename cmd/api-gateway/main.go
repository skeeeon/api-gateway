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

	"api-gateway/internal/config"
	"api-gateway/internal/gateway"
	"api-gateway/internal/logger"
)

func main() {
	// Initialize basic logger for bootstrapping
	bootstrapLogger, _ := zap.NewProduction()
	defer bootstrapLogger.Sync()

	bootstrapLogger.Info("Starting API Gateway service")

	// Get configuration file path
	configPath := config.GetConfigPath()

	// Load configuration with bootstrap logger
	cfg, err := config.LoadConfig(configPath, bootstrapLogger)
	if err != nil {
		bootstrapLogger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize the enhanced logger with config
	log, err := logger.New(logger.Config{
		Level:      cfg.Logging.Level,
		Outputs:    cfg.Logging.Outputs,
		FilePath:   cfg.Logging.FilePath,
		MaxSize:    cfg.Logging.MaxSize,
		MaxAge:     cfg.Logging.MaxAge,
		MaxBackups: cfg.Logging.MaxBackups,
		Compress:   cfg.Logging.Compress,
	})
	if err != nil {
		bootstrapLogger.Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer log.Sync()

	log.Info("API Gateway service started with enhanced logging")

	// Create API Gateway with our enhanced logger
	gw, err := gateway.New(cfg, log)
	if err != nil {
		log.Fatal("Failed to create API Gateway", zap.Error(err))
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: gw,
	}

	// Start the server in a goroutine
	go func() {
		log.Info("Starting HTTP server", zap.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	log.Info("Shutting down gracefully...")

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shut down the server
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error", zap.Error(err))
	}

	log.Info("Server stopped, goodbye!")
}
