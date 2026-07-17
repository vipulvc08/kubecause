package rca

import (
	"strings"
	"testing"
)

func TestPagerDutyNote_short(t *testing.T) {
	note := PagerDutyNote("root cause: OOMKilled")
	if !strings.Contains(note, "kubecause RCA") {
		t.Errorf("missing header: %q", note)
	}
	if !strings.Contains(note, "OOMKilled") {
		t.Errorf("missing body")
	}
}

func TestPagerDutyNote_truncates(t *testing.T) {
	long := strings.Repeat("x", 5000)
	note := PagerDutyNote(long)
	if len(note) > 4000 {
		t.Errorf("note not truncated: len=%d", len(note))
	}
	if !strings.Contains(note, "[truncated") {
		t.Errorf("missing truncation footer")
	}
}
