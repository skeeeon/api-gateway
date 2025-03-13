// Package logger provides enhanced logging capabilities with support
// for multiple output destinations (console and file) using zap
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config contains all logging configuration options
type Config struct {
	// Level is the minimum enabled logging level (debug, info, warn, error)
	Level string
	
	// Outputs is a list of output destinations ("console", "file", or both)
	Outputs []string
	
	// FilePath is the path to the log file (required when file output is enabled)
	FilePath string
	
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated
	MaxSize int
	
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int
	
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	
	// Compress determines if the rotated log files should be compressed
	Compress bool
}

// New creates a new logger with the specified configuration
func New(config Config) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(config.Level)); err != nil {
		// Default to info level if invalid
		level = zap.InfoLevel
	}
	
	// Create atom to dynamically change log level
	atom := zap.NewAtomicLevelAt(level)
	
	// Configure encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	
	// Create encoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	
	// Setup output syncs for enabled outputs
	var cores []zapcore.Core
	
	// Add outputs based on configuration
	for _, output := range config.Outputs {
		switch strings.ToLower(output) {
		case "console":
			// Add console output
			cores = append(cores, zapcore.NewCore(
				encoder,
				zapcore.AddSync(os.Stdout),
				atom,
			))
		case "file":
			// Ensure directory exists
			if err := ensureDirectoryExists(config.FilePath); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}
			
			// Configure log rotation
			fileWriter := zapcore.AddSync(&lumberjack.Logger{
				Filename:   config.FilePath,
				MaxSize:    config.MaxSize,    // MB
				MaxAge:     config.MaxAge,     // days
				MaxBackups: config.MaxBackups, // files
				Compress:   config.Compress,
			})
			
			// Add file output
			cores = append(cores, zapcore.NewCore(
				encoder,
				fileWriter,
				atom,
			))
		default:
			return nil, fmt.Errorf("unsupported log output type: %s", output)
		}
	}
	
	// Create multi-core if multiple outputs
	var core zapcore.Core
	if len(cores) == 1 {
		core = cores[0]
	} else if len(cores) > 1 {
		core = zapcore.NewTee(cores...)
	} else {
		// Default to console logging if no valid outputs specified
		core = zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			atom,
		)
	}
	
	// Create logger
	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	
	// Log startup information
	logger.Info("Logger initialized",
		zap.String("level", level.String()),
		zap.Strings("outputs", config.Outputs),
	)
	
	return logger, nil
}

// ensureDirectoryExists creates the directory for a file path if it doesn't exist
func ensureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "" {
		return nil
	}
	
	return os.MkdirAll(dir, 0755)
}

// GetLevel parses a log level string and returns the corresponding zap level
func GetLevel(levelStr string) zapcore.Level {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		return zap.InfoLevel // Default to info
	}
	return level
}

// WithFields adds fields to the logger context
func WithFields(l *zap.Logger, fields ...zapcore.Field) *zap.Logger {
	return l.With(fields...)
}
