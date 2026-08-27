package store

import (
	"testing"
	"time"
)

func TestParseTime_sqliteDatetime(t *testing.T) {
	tm, err := parseTime("2026-08-27 18:31:20")
	if err != nil {
		t.Fatal(err)
	}
	if tm.Year() != 2026 || tm.Month() != time.August || tm.Day() != 27 {
		t.Fatalf("unexpected time: %v", tm)
	}
}

func TestParseTime_rfc3339Nano(t *testing.T) {
	in := "2026-08-27T18:31:20.123456789Z"
	tm, err := parseTime(in)
	if err != nil {
		t.Fatal(err)
	}
	if tm.UTC().Format(time.RFC3339Nano) != in {
		t.Fatalf("got %q", tm.UTC().Format(time.RFC3339Nano))
	}
}
