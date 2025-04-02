package service

import (
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	logger *log.Logger
)

func NewLogger() *log.Logger {
	logger = log.New()
	return logger
}

func GetLogger() *log.Logger {
	if logger == nil {
		logger = NewLogger()
	}
	return logger
}

func InitLogger() {
	logger.SetOutput(os.Stderr)
	logger.SetLevel(log.InfoLevel)            // Default to Info level initially
	logger.SetFormatter(&log.TextFormatter{}) // Default to TextFormatter
}

func Logf(format string, args ...interface{}) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}
