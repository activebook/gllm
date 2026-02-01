package service

import (
	"os"

	"github.com/activebook/gllm/internal/ui"
	log "github.com/sirupsen/logrus"
)

var (
	logger          *log.Logger
	indicatorActive bool // Tracks if indicator was active before logging
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

func BeforeLog() {
	if logger != nil {
		// Stop indicator to avoid overlap
		indicatorActive = ui.GetIndicator().IsActive()
		// fmt.Println("logger before log")
		ui.GetIndicator().Stop()
	}
}

func AfterLog() {
	if logger != nil {
		// Restart indicator if it was active
		if indicatorActive {
			// fmt.Println("logger after log")
			ui.GetIndicator().Start("")
		}
	}
}

func Infof(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infof(format, args...)
		AfterLog()
	}
}

func Infoln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infoln(args...)
		AfterLog()
	}
}

func Successf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infof(format, args...)
		AfterLog()
	}
}

func Successln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Infoln(args...)
		AfterLog()
	}
}

func Debugf(format string, args ...interface{}) {
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

func Debugln(args ...interface{}) {
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

func Warnf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Warnf(format, args...)
		AfterLog()
	}
}

func Warnln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Warnln(args...)
		AfterLog()
	}
}

func Errorf(format string, args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Errorf(format, args...)
		AfterLog()
	}
}

func Errorln(args ...interface{}) {
	if logger != nil {
		BeforeLog()
		logger.Errorln(args...)
		AfterLog()
	}
}
