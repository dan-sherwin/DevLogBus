package client

import (
	"strings"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

// RedactedValue is the default replacement used by redaction helpers.
const RedactedValue = "[REDACTED]"

// RecordFilter reports whether a record should be published.
type RecordFilter func(protocol.Record) bool

// RecordRedactor returns the record shape that should be published.
type RecordRedactor func(protocol.Record) protocol.Record

// DropSources returns a filter that drops records from the listed sources.
func DropSources(sources ...string) RecordFilter {
	blocked := map[string]struct{}{}
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source != "" {
			blocked[source] = struct{}{}
		}
	}
	return func(record protocol.Record) bool {
		_, drop := blocked[record.Source]
		return !drop
	}
}

// RedactAttrs returns a redactor that replaces matching attrs with RedactedValue.
func RedactAttrs(keys ...string) RecordRedactor {
	return RedactAttrsWith(RedactedValue, keys...)
}

// RedactAttrsWith returns a redactor that replaces matching attrs.
func RedactAttrsWith(replacement any, keys ...string) RecordRedactor {
	matchers := normalizedAttrKeys(keys)
	return func(record protocol.Record) protocol.Record {
		if len(record.Attrs) == 0 || len(matchers) == 0 {
			return record
		}
		record.Attrs = redactAttrs(record.Attrs, "", replacement, matchers)
		return record
	}
}

func prepareRecord(record protocol.Record, filter RecordFilter, redactor RecordRedactor) (protocol.Record, bool, error) {
	if filter != nil && !filter(record) {
		return protocol.Record{}, false, nil
	}
	if redactor != nil {
		record = redactor(record)
	}
	if err := validateRecord(&record); err != nil {
		return protocol.Record{}, false, err
	}
	return record, true, nil
}

func normalizedAttrKeys(keys []string) map[string]struct{} {
	matchers := map[string]struct{}{}
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			matchers[key] = struct{}{}
		}
	}
	return matchers
}

func redactAttrs(attrs map[string]any, prefix string, replacement any, matchers map[string]struct{}) map[string]any {
	out := make(map[string]any, len(attrs))
	for key, value := range attrs {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if attrKeyMatches(key, path, matchers) {
			out[key] = replacement
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = redactAttrs(nested, path, replacement, matchers)
			continue
		}
		out[key] = value
	}
	return out
}

func attrKeyMatches(key string, path string, matchers map[string]struct{}) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	path = strings.ToLower(strings.TrimSpace(path))
	if _, ok := matchers[key]; ok {
		return true
	}
	_, ok := matchers[path]
	return ok
}
