// config/setupLogging/interface.go
package setupLogging

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/rs/zerolog"
)

// AppLogger définit l'interface pour le logger de l'application
type AppLogger interface {
	// Méthodes de logging standard
	Trace() *zerolog.Event
	Debug() *zerolog.Event
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
	Fatal() *zerolog.Event
	Panic() *zerolog.Event

	// Méthodes utilitaires
	With() zerolog.Context
	Level(level zerolog.Level) zerolog.Logger
	Sample(s zerolog.Sampler) zerolog.Logger
	Hook(h zerolog.Hook) zerolog.Logger

	// Méthodes contextuelles
	WithContext(ctx context.Context) context.Context
	GetLevel() zerolog.Level
}

// Pour la compatibilité avec le logger global zerolog
var (
	// Ces fonctions pointent vers le logger global zerolog
	// Tu peux les utiliser en attendant la migration complète
	Print  = log.Print
	Printf = log.Printf
	//Println = log.Println

	Debug  = log.Debug
	Debugf = func(format string, v ...interface{}) {
		log.Debug().Msgf(format, v...)
	}
	Info  = log.Info
	Infof = func(format string, v ...interface{}) {
		log.Info().Msgf(format, v...)
	}
	Warn  = log.Warn
	Warnf = func(format string, v ...interface{}) {
		log.Warn().Msgf(format, v...)
	}
	Error  = log.Error
	Errorf = func(format string, v ...interface{}) {
		log.Error().Msgf(format, v...)
	}
	Fatal  = log.Fatal
	Fatalf = func(format string, v ...interface{}) {
		log.Fatal().Msgf(format, v...)
	}
	Panic  = log.Panic
	Panicf = func(format string, v ...interface{}) {
		log.Panic().Msgf(format, v...)
	}
)
