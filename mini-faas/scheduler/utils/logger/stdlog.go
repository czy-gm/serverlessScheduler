package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	stdlogCallFrameDepth = 3

	logFormat = log.LstdFlags | log.Llongfile
)

//StdLogger wraps a built-in logger.
type StdLogger struct {
	logger *log.Logger
}

//NewStdLogger outputs logs to stdout.
func NewStdLogger() Logger {
	return NewStdLoggerFromConfig(os.Stdout, logFormat)
}

// NewStdLoggerFromBuffer ...
func NewStdLoggerFromBuffer(buf io.Writer) Logger {
	return NewStdLoggerFromConfig(buf, logFormat)
}

// NewStdLoggerFromConfig ...
func NewStdLoggerFromConfig(buf io.Writer, logFormat int) Logger {
	return &StdLogger{
		logger: log.New(buf, "", logFormat),
	}
}

// Flush ...
func (_this *StdLogger) Flush() {
}

// WithContext ...
func (_this *StdLogger) WithContext(ctx context.Context) Logger {
	return NewCfLogger(_this, ctx, nil)
}

// WithFields ...
func (_this *StdLogger) WithFields(fields Fields) Logger {
	return NewCfLogger(_this, nil, fields)
}

// Infof :
func (_this *StdLogger) Infof(format string, v ...interface{}) {
	f := "[Info] " + format
	_this.logger.Output(stdlogCallFrameDepth, fmt.Sprintf(f, v...))
}

// Debugf ...
func (_this *StdLogger) Debugf(format string, v ...interface{}) {
	f := "[Debug] " + format
	_this.logger.Output(stdlogCallFrameDepth, fmt.Sprintf(f, v...))
}

// Warningf ...
func (_this *StdLogger) Warningf(format string, v ...interface{}) {
	f := "[Warning] " + format
	_this.logger.Output(stdlogCallFrameDepth, fmt.Sprintf(f, v...))
}

// Errorf ...
func (_this *StdLogger) Errorf(format string, v ...interface{}) {
	f := "[Error] " + format
	_this.logger.Output(stdlogCallFrameDepth, fmt.Sprintf(f, v...))
}

// Criticalf ...
func (_this *StdLogger) Criticalf(format string, v ...interface{}) {
	f := "[Critical] " + format
	_this.logger.Output(stdlogCallFrameDepth, fmt.Sprintf(f, v...))
}
