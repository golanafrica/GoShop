// tests/testutilitis/logger.go
package testutilitis

import "Goshop/config/setupLogging"

func NewSilentLogger() *setupLogging.Logger {
	return setupLogging.NewLogger(setupLogging.Config{
		Environment: "test",
		ServiceName: "goshop-test",
		Version:     "test",
		LogLevel:    "fatal", // ← seulement fatal/panic
	})
}
