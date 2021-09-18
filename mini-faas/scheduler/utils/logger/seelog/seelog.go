package seelog

import (
	"context"
	"io"

	"github.com/cihub/seelog"

	lg "aliyun/serverless/mini-faas/scheduler/utils/logger"
)

const (
	seelogCallFrameDepth = 2

	logFormat = "%Date %Time [%LEVEL] %RelFile:%FuncShort:%Line %Msg%n"
)

// SeeLogger wraps a see logger.
type SeeLogger struct {
	logger seelog.LoggerInterface
}

// NewSeeLoggerFromFile creates a seelog from a config.
func NewSeeLoggerFromFile(config string) (lg.Logger, error) {
	logger, err := seelog.LoggerFromConfigAsFile(config)
	if err != nil {
		return nil, err
	}
	logger.SetAdditionalStackDepth(seelogCallFrameDepth)
	return &SeeLogger{
		logger: logger,
	}, nil
}

// NewSeeLoggerFromBuffer creates a seelog from a buffer.
func NewSeeLoggerFromBuffer(buf io.Writer, minLevel seelog.LogLevel) (lg.Logger, error) {
	logger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(buf, minLevel, logFormat)
	if err != nil {
		return nil, err
	}
	logger.SetAdditionalStackDepth(seelogCallFrameDepth)
	return &SeeLogger{
		logger: logger,
	}, nil
}

// Flush ...
func (_this *SeeLogger) Flush() {
	_this.logger.Flush()
}

// WithContext ...
func (_this *SeeLogger) WithContext(ctx context.Context) lg.Logger {
	return lg.NewCfLogger(_this, ctx, nil)
}

// WithFields ...
func (_this *SeeLogger) WithFields(fields lg.Fields) lg.Logger {
	return lg.NewCfLogger(_this, nil, fields)
}

// Infof ...
func (_this *SeeLogger) Infof(format string, v ...interface{}) {
	_this.logger.Infof(format, v...)
}

// Debugf ...
func (_this *SeeLogger) Debugf(format string, v ...interface{}) {
	_this.logger.Debugf(format, v...)
}

// Errorf ...
func (_this *SeeLogger) Errorf(format string, v ...interface{}) {
	_this.logger.Errorf(format, v...)
}

// Warningf ...
func (_this *SeeLogger) Warningf(format string, v ...interface{}) {
	_this.logger.Warnf(format, v...)
}

// Criticalf ...
func (_this *SeeLogger) Criticalf(format string, v ...interface{}) {
	_this.logger.Criticalf(format, v...)
}
