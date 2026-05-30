// Package slogattrs contains shared slog attribute conversion helpers.
package slogattrs

import (
	"fmt"
	"log/slog"
	"time"
)

// Copy returns a shallow copy of source.
func Copy(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

// Add flattens attr into attrs, preserving slog group boundaries with dotted keys.
func Add(attrs map[string]any, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(append([]string{}, groups...), attr.Key)
		}
		for _, groupAttr := range attr.Value.Group() {
			Add(attrs, nextGroups, groupAttr)
		}
		return
	}
	if attr.Key == "" {
		return
	}

	key := attr.Key
	for i := len(groups) - 1; i >= 0; i-- {
		key = groups[i] + "." + key
	}
	attrs[key] = valueAny(attr.Value)
}

func valueAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindUint64:
		return value.Uint64()
	default:
		return fmt.Sprint(value.Any())
	}
}
