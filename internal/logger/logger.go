package logger

import (
	"log"
	"os"
)

// Logger implements ports.ILogger interface using standard log package
type Logger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	debugLog *log.Logger
}

// New creates a new logger instance
func New() *Logger {
	return &Logger{
		infoLog:  log.New(os.Stdout, "[INFO] ", log.LstdFlags),
		errorLog: log.New(os.Stderr, "[ERROR] ", log.LstdFlags),
		debugLog: log.New(os.Stdout, "[DEBUG] ", log.LstdFlags),
	}
}

// Info logs informational messages
func (l *Logger) Info(args ...interface{}) {
	l.infoLog.Print(args...)
}

// Infof logs formatted informational messages
func (l *Logger) Infof(format string, args ...interface{}) {
	l.infoLog.Printf(format, args...)
}

// Error logs error messages
func (l *Logger) Error(args ...interface{}) {
	l.errorLog.Print(args...)
}

// Errorf logs formatted error messages
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.errorLog.Printf(format, args...)
}

// Debug logs debug messages
func (l *Logger) Debug(args ...interface{}) {
	l.debugLog.Print(args...)
}

// Debugf logs formatted debug messages
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.debugLog.Printf(format, args...)
}
