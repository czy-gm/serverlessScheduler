package logger

import (
	"bytes"
	"context"
	"fmt"
)

// If you are seeking for the logger usage. See the static.go.
// Do not use any logger directly. Always use it throught the
// static methods in static.go. This will ensure the call frame
// can be printed correctly.

// Fields is served as key value pairs holder.
type Fields map[string]interface{}

// String converts the map to "k1=v1 k2=v2" format string.
func (f Fields) String() string {
	var b bytes.Buffer
	for k, v := range f {
		b.WriteString(k)
		b.WriteByte('=')
		fmt.Fprint(&b, v)
		b.WriteByte(' ')
	}
	return b.String()
}

// Logger defines the logging interface.
type Logger interface {
	Flush()
	WithContext(ctx context.Context) Logger

	// Log with given fields.
	// WARNING: modifying the same "fields" after calling WithFields is not safe.
	WithFields(fields Fields) Logger
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Warningf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Criticalf(format string, v ...interface{})
}
