package recordfmt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func Format(record protocol.Record) string {
	keys := make([]string, 0, len(record.Attrs))
	for key := range record.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("%s=%v", key, record.Attrs[key]))
	}
	suffix := ""
	if len(fields) > 0 {
		suffix = " " + strings.Join(fields, " ")
	}
	return fmt.Sprintf("%s %-5s %-24s %s%s", record.Time.Format("15:04:05.000"), protocol.NormalizeLevel(record.Level), record.Source, record.Message, suffix)
}
