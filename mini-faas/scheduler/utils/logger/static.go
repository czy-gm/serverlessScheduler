package logger

import (
    "context"

    "google.golang.org/grpc/metadata"
)

// This file provides global access for loggers. You can set/get loggers,
// or use the default logger to log message. Considering performance and
// setter/getter usage, setter/getter does not provide thread safe access
// to the sotre.
//
// If you are using this as your logging framework, please ensure that
// loggers are setted in a single thread environemnt.

var (
    // Default logger.
    defaultLogger = NewStdLogger()

    // Store caches different loggers.
    store = map[string]Logger{
        "Logger_Default": NewCfLogger(defaultLogger, nil, nil),
    }
)

// SetLogger stores a named logger. Setted logger will be wrapped by
// CfLogger so that inner logger can print the correct call frame.
// The number of call frame between callers and inner logger depends
// on the logger implementation itself.
func SetLogger(name string, logger Logger) {
    if logger == nil {
        return
    }
    store[name] = NewCfLogger(logger, nil, nil)
    if name == "Logger_Default" {
        defaultLogger = logger
    }
}

// GetLogger returns a named wrapped logger.
//
// If the logger doesn't exist, return a default logger.
func GetLogger(name string) Logger {
    if logger, ok := store[name]; ok && logger != nil {
        return logger
    }
    return store["Logger_Default"]
}

// Flush ...
func Flush() {
    defaultLogger.Flush()
}

// WithContext ...
func WithContext(ctx context.Context) Logger {
    return defaultLogger.WithContext(ctx)
}

// WithFields ...
func WithFields(fields Fields) Logger {
    return defaultLogger.WithFields(fields)
}

// Infof ...
func Infof(format string, v ...interface{}) {
    defaultLogger.Infof(format, v...)
}

// Debugf ...
func Debugf(format string, v ...interface{}) {
    defaultLogger.Debugf(format, v...)
}

// Warningf ...
func Warningf(format string, v ...interface{}) {
    defaultLogger.Warningf(format, v...)
}

// Errorf ...
func Errorf(format string, v ...interface{}) {
    defaultLogger.Errorf(format, v...)
}

// Criticalf ...
func Criticalf(format string, v ...interface{}) {
    defaultLogger.Criticalf(format, v...)
}

// RequestID retrieves request id from context
func RequestID(ctx context.Context) string {
    md, ok := metadata.FromContext(ctx)
    if ok {
        value := md["x-acs-request-id"]
        if len(value) > 0 {
            return value[0]
        }
    }

    return ""
}
