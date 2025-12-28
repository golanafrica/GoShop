// config/setupLogging/setupLogging.go
package setupLogging

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config contient la configuration du logging
type Config struct {
	Environment string // "development", "staging", "production"
	ServiceName string
	Version     string
	LogLevel    string // "debug", "info", "warn", "error"

	// Pour la production
	LogFilePath string
	MaxSizeMB   int
	MaxBackups  int
	MaxAgeDays  int
	Compress    bool
}

// Logger est l'interface pour le logger de l'application
type Logger struct {
	zerolog.Logger
	config Config
}

// NewLogger crée et configure un nouveau logger
func NewLogger(cfg Config) *Logger {
	// Configure le stack trace des erreurs
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Définit le niveau de log
	setLogLevel(cfg.LogLevel, cfg.Environment)

	// Crée le writer (console ou fichier)
	var output io.Writer
	if cfg.Environment == "development" {
		output = createConsoleWriter()
	} else {
		output = createFileWriter(cfg)
	}

	// Ajoute le caller (fichier:ligne) pour les erreurs et warnings
	callerWriter := zerolog.New(output).With().
		CallerWithSkipFrameCount(4). // Ajuste selon ta stack
		Logger()

	// Crée le logger final avec les champs globaux
	logger := callerWriter.With().
		Str("service", cfg.ServiceName).
		Str("version", cfg.Version).
		Str("environment", cfg.Environment).
		Timestamp().
		Logger()

	return &Logger{
		Logger: logger,
		config: cfg,
	}
}

// setLogLevel définit le niveau de log global
func setLogLevel(level string, env string) {
	var logLevel zerolog.Level

	switch strings.ToLower(level) {
	case "trace":
		logLevel = zerolog.TraceLevel
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn", "warning":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	case "fatal":
		logLevel = zerolog.FatalLevel
	case "panic":
		logLevel = zerolog.PanicLevel
	default:
		// Par défaut selon l'environnement
		switch env {
		case "development":
			logLevel = zerolog.DebugLevel
		case "production":
			logLevel = zerolog.InfoLevel
		default:
			logLevel = zerolog.InfoLevel
		}
	}

	zerolog.SetGlobalLevel(logLevel)

	log.Debug().
		Str("level", logLevel.String()).
		Str("environment", env).
		Msg("log level configured")
}

// createConsoleWriter crée un writer console coloré pour le développement
func createConsoleWriter() io.Writer {
	return zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05.000",
		FormatLevel: func(i interface{}) string {
			var level string
			if ll, ok := i.(string); ok {
				switch ll {
				case "trace":
					level = colorize("TRACE", 90) // Gris
				case "debug":
					level = colorize("DEBUG", 36) // Cyan
				case "info":
					level = colorize("INFO ", 32) // Vert
				case "warn":
					level = colorize("WARN ", 33) // Jaune
				case "error":
					level = colorize("ERROR", 31) // Rouge
				case "fatal":
					level = colorize("FATAL", 35) // Magenta
				case "panic":
					level = colorize("PANIC", 35) // Magenta
				default:
					level = colorize(ll, 0)
				}
			}
			return fmt.Sprintf("|%s|", level)
		},
		FormatMessage: func(i interface{}) string {
			if msg, ok := i.(string); ok {
				return colorize(msg, 1) // Gras
			}
			return ""
		},
		FormatFieldName: func(i interface{}) string {
			if name, ok := i.(string); ok {
				return colorize(name+":", 36) // Cyan
			}
			return ""
		},
		FormatFieldValue: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
		},
	}
}

// createFileWriter crée un writer fichier avec rotation pour la production
func createFileWriter(cfg Config) io.Writer {
	if cfg.LogFilePath == "" {
		cfg.LogFilePath = "/var/log/goshop/app.log"
	}

	return &lumberjack.Logger{
		Filename:   cfg.LogFilePath,
		MaxSize:    cfg.MaxSizeMB,  // MB
		MaxBackups: cfg.MaxBackups, // nombre de fichiers
		MaxAge:     cfg.MaxAgeDays, // jours
		Compress:   cfg.Compress,
	}
}

// colorize ajoute des codes couleur ANSI
func colorize(s interface{}, colorCode int) string {
	if colorCode == 0 {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", colorCode, s)
}

// GetDefaultConfig retourne la configuration par défaut
func GetDefaultConfig() Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	return Config{
		Environment: env,
		ServiceName: "goshop-api",
		Version:     getVersion(),
		LogLevel:    os.Getenv("LOG_LEVEL"),
		LogFilePath: os.Getenv("LOG_FILE_PATH"),
		MaxSizeMB:   100,  // 100MB par fichier
		MaxBackups:  3,    // Garde 3 fichiers backup
		MaxAgeDays:  28,   // Garde 28 jours
		Compress:    true, // Compresse les vieux logs
	}
}

// getVersion récupère la version depuis les variables d'environnement ou git
func getVersion() string {
	if version := os.Getenv("APP_VERSION"); version != "" {
		return version
	}
	return "1.0.0"
}

// Helper functions pour faciliter l'utilisation

// WithRequestID ajoute un request ID au logger
func (l *Logger) WithRequestID(requestID string) *Logger {
	logger := l.With().Str("request_id", requestID).Logger()
	return &Logger{Logger: logger, config: l.config}
}

// WithComponent ajoute un composant au logger
func (l *Logger) WithComponent(component string) *Logger {
	logger := l.With().Str("component", component).Logger()
	return &Logger{Logger: logger, config: l.config}
}

// WithUserID ajoute un user ID au logger
func (l *Logger) WithUserID(userID string) *Logger {
	logger := l.With().Str("user_id", userID).Logger()
	return &Logger{Logger: logger, config: l.config}
}

// WithOperation ajoute une opération au logger
func (l *Logger) WithOperation(operation string) *Logger {
	logger := l.With().Str("operation", operation).Logger()
	return &Logger{Logger: logger, config: l.config}
}

// NewContext crée un nouveau contexte avec le logger
func (l *Logger) NewContext(ctx context.Context) context.Context {
	return l.WithContext(ctx)
}

// Global helper pour récupérer un logger depuis le contexte
func FromContext(ctx context.Context) *Logger {
	if logger := zerolog.Ctx(ctx); logger != nil {
		return &Logger{Logger: *logger}
	}
	// Retourne un logger par défaut si aucun dans le contexte
	cfg := GetDefaultConfig()
	return NewLogger(cfg)

}

// Zerolog retourne le logger zerolog sous-jacent
func (l *Logger) Zerolog() zerolog.Logger {
	return l.Logger
}
