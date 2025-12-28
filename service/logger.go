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
	logger.SetLevel(log.InfoLevel) // Default to Info level initially
	logger.SetFormatter(&log.TextFormatter{
		DisableColors:    false,
		DisableTimestamp: true, // Remove timestamp numbers like [0000]
	})
}

func Infof(format string, args ...interface{}) {
	if logger != nil {
		logger.Infof(cyanColor+format+resetColor, args...)
	}
}

func Successf(format string, args ...interface{}) {
	if logger != nil {
		logger.Infof(completeColor+format+resetColor, args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(debugColor+format+resetColor, args...)
	}
}

func Debugln(args ...interface{}) {
	if logger != nil {
		logger.Debugln(args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(warnColor+format+resetColor, args...)
	}
}

func Warnln(args ...interface{}) {
	if logger != nil {
		logger.Warnln(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(errorColor+format+resetColor, args...)
	}
}

func Errorln(args ...interface{}) {
	if logger != nil {
		logger.Errorln(args...)
	}
}
