package logger

import (
	"context"
	"fmt"
)

// CfLogger (context field logger) supports formatting value from
// a context object and fields. This ia a wrapper of logger.
type CfLogger struct {
	logger Logger
	ctx    context.Context
	fields Fields
}

// NewCfLogger ...
func NewCfLogger(logger Logger, ctx context.Context, fields Fields) Logger {
	return &CfLogger{
		logger: logger,
		ctx:    ctx,
		fields: fields,
	}
}

// InnerLogger ...
func (_this *CfLogger) InnerLogger() Logger {
	return _this.logger
}

// Flush ...
func (_this *CfLogger) Flush() {
	_this.logger.Flush()
}

// WithContext ...
func (_this *CfLogger) WithContext(ctx context.Context) Logger {
	return NewCfLogger(_this.logger, ctx, _this.fields)
}

// WithFields ...
func (_this *CfLogger) WithFields(fields Fields) Logger {
	return NewCfLogger(_this.logger, _this.ctx, fields)
}

// Infof ...
func (_this *CfLogger) Infof(format string, v ...interface{}) {
	_this.logger.Infof(_this.format(format), v...)
}

// Debugf ...
func (_this *CfLogger) Debugf(format string, v ...interface{}) {
	_this.logger.Debugf(_this.format(format), v...)
}

// Warningf ...
func (_this *CfLogger) Warningf(format string, v ...interface{}) {
	_this.logger.Warningf(_this.format(format), v...)
}

// Errorf ...
func (_this *CfLogger) Errorf(format string, v ...interface{}) {
	_this.logger.Errorf(_this.format(format), v...)
}

// Criticalf ...
func (_this *CfLogger) Criticalf(format string, v ...interface{}) {
	_this.logger.Criticalf(_this.format(format), v...)
	_this.Flush()
}

// Format the format string.
func (_this *CfLogger) format(format string) string {
	// Prepend the request id.
	if _this.ctx != nil {
		reqID := RequestID(_this.ctx)
		format = fmt.Sprintf("[RequestID: %s] %s", reqID, format)
	}
	// Append fields at the end.
	if _this.fields != nil {
		format = format + " " + _this.fields.String()
	}
	return format
}
