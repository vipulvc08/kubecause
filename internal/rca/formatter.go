// Package rca formats the model's final answer for the incident channel
// (PagerDuty note, Slack thread, GitHub issue, ...).
package rca

import "strings"

// PagerDutyNote wraps the raw RCA text into a compact note suitable for
// PagerDuty's incident-note API. PagerDuty notes have a 4KB limit — we
// truncate with a footer if the RCA is longer.
func PagerDutyNote(rca string) string {
	const maxLen = 3800
	trimmed := strings.TrimSpace(rca)
	header := "🤖 kubecause RCA\n\n"
	body := header + trimmed
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "\n\n[truncated — see kubecause logs for full RCA]"
}
