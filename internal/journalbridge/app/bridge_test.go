package app

import (
	"testing"
)

func TestJournalRecordMapping(t *testing.T) {
	entry := journalEntry{
		Fields: map[string]string{
			"MESSAGE":           "database timeout",
			"PRIORITY":          "3",
			"_SYSTEMD_UNIT":     "billing.service",
			"SYSLOG_IDENTIFIER": "billing",
		},
		RealtimeTimestamp: 1_700_000_000_123_456,
	}
	record := journalRecord(entry, bridgeOptions{
		Source:       "journald",
		SourceFields: defaultSourceFields,
	})

	if record.Source != "billing.service" {
		t.Fatalf("source = %q, want billing.service", record.Source)
	}
	if record.Level != "ERROR" {
		t.Fatalf("level = %q, want ERROR", record.Level)
	}
	if record.Message != "database timeout" {
		t.Fatalf("message = %q, want database timeout", record.Message)
	}
	if record.Attrs["PRIORITY"] != "3" {
		t.Fatalf("priority attr = %#v, want 3", record.Attrs["PRIORITY"])
	}
}

func TestShouldSkipDefaultBridgeUnit(t *testing.T) {
	options := bridgeOptions{
		ExcludeUnits:       stringSet(defaultExcludeUnits()),
		ExcludeIdentifiers: stringSet(defaultExcludeIdentifiers()),
	}
	if !shouldSkipEntry(journalEntry{Fields: map[string]string{"_SYSTEMD_UNIT": "devlogbus-journal-bridge.service"}}, options) {
		t.Fatal("expected bridge service unit to be skipped")
	}
	if !shouldSkipEntry(journalEntry{Fields: map[string]string{"SYSLOG_IDENTIFIER": "devlogbus-journal-bridge"}}, options) {
		t.Fatal("expected bridge syslog identifier to be skipped")
	}
}
