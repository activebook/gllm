package service

import (
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	logger *log.Logger
)

const (
	// Terminal colors
	errorColor = "\033[31m"       // Red
	warnColor  = "\033[38;5;208m" // Dark Orange
	debugColor = "\033[34m"       // Blue
	resetColor = "\033[0m"
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

func Infof(format string, args ...interface{}) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(debugColor+format+resetColor, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(warnColor+format+resetColor, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(errorColor+format+resetColor, args...)
	}
}
