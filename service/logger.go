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
	cyanColor  = "\033[36m"       // Bright Green
	bbColor    = "\033[90m"       // Bright Black
	dimColor   = "\033[2m"        // Dim
	greyColor  = "\033[38;5;240m" // Grey
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
	logger.SetLevel(log.InfoLevel) // Default to Info level initially
	logger.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true, // Remove timestamp numbers like [0000]
	})
}

func Infof(format string, args ...interface{}) {
	if logger != nil {
		logger.Infof(cyanColor+format+resetColor, args...)
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
