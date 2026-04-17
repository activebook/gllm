package util

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// LoggerHook defines an interface for external components to intercept logging
// and handle UI-specific tasks like stopping/restarting indicators.
type LoggerHook interface {
	BeforeLog() bool // Returns true if indicator was active
	AfterLog(wasActive bool)
}

var (
	logger     *log.Logger
	globalHook LoggerHook
)

// RegisterLoggerHook registers a hook to be called before and after logging.
func RegisterLoggerHook(h LoggerHook) {
	globalHook = h
}

var (
	indicatorActive bool // Tracks if indicator was active before logging
)

// func NewLogger() *log.Logger {
// 	logger = log.New()
// 	return logger
// }

// func GetLogger() *log.Logger {
// 	if logger == nil {
// 		logger = NewLogger()
// 	}
// 	return logger
// }

func InitLogger() {
	if logger == nil {
		logger = log.New()
		logger.SetOutput(os.Stderr)
		logger.SetLevel(log.InfoLevel) // Default to Info level initially
		logger.SetFormatter(&log.TextFormatter{
			DisableColors:    false,
			DisableTimestamp: true, // Remove timestamp numbers like [0000]
		})
	}
}

func SetLoggerLevel(level log.Level) {
	if logger != nil {
		logger.SetLevel(level)
	}
}

func BeforeLog() {
	if logger != nil && globalHook != nil {
		indicatorActive = globalHook.BeforeLog()
	}
}

func AfterLog() {
	if logger != nil && globalHook != nil {
		globalHook.AfterLog(indicatorActive)
	}
}

func LogInfof(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infof(format, args...)
		AfterLog()
	}
}

func LogInfoln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infoln(args...)
		AfterLog()
	}
}

func LogSuccessf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infof(format, args...)
		AfterLog()
	}
}

func LogSuccessln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infoln(args...)
		AfterLog()
	}
}

func LogDebugf(format string, args ...interface{}) {
	if logger != nil {
		if logger.Level == log.DebugLevel {
			BeforeLog()
		}
		logger.Debugf(format, args...)
		if logger.Level == log.DebugLevel {
			AfterLog()
		}
	}
}

func LogDebugln(args ...interface{}) {
	if logger != nil {
		if logger.Level == log.DebugLevel {
			BeforeLog()
		}
		logger.Debugln(args...)
		if logger.Level == log.DebugLevel {
			AfterLog()
		}
	}
}

func LogWarnf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Warnf(format, args...)
		AfterLog()
	}
}

func LogWarnln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Warnln(args...)
		AfterLog()
	}
}

func LogErrorf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Errorf(format, args...)
		// Error stop here
		// AfterLog()
	}
}

func LogErrorln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Errorln(args...)
		// Error stop here
		// AfterLog()
	}
}
