// config/setupLogging/test_helper.go
package setupLogging

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// TestLogger singleton pour tous les tests
var testLoggerInstance *Logger

// GetTestLogger retourne un logger configuré pour les tests
func GetTestLogger() *Logger {
	if testLoggerInstance == nil {
		cfg := Config{
			Environment: "test",
			ServiceName: "test",
			Version:     "1.0.0",
			LogLevel:    "panic", // N'affiche presque rien
		}

		// Redirige vers /dev/null ou NUL pour être silencieux
		if f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
			output := zerolog.New(io.MultiWriter(f))
			logger := output.With().Timestamp().Logger()
			testLoggerInstance = &Logger{Logger: logger, config: cfg}
		} else {
			// Fallback
			testLoggerInstance = NewLogger(cfg)
		}
	}
	return testLoggerInstance
}

// SilentLogger crée un logger complètement silencieux
func SilentLogger() *Logger {
	cfg := Config{
		Environment: "silent",
		ServiceName: "silent",
		Version:     "1.0.0",
		LogLevel:    "panic",
	}
	logger := NewLogger(cfg)
	logger.Level(zerolog.Disabled)
	return logger
}
