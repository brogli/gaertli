package main

import (
	"fmt"
	"time"
)

// spikeRow approximates one future LogService row (FR-LOG-01: hash, subject,
// author, date, ref labels). Graph lanes are excluded — internal/graph
// computes those client-side from parent data, not part of a row's own
// payload. ~180-220 bytes marshaled JSON per row: a 40-char hash, a ~50-rune
// subject, a ~20-rune author, an RFC3339 date, 0-2 short ref labels.
type spikeRow struct {
	Hash    string   `json:"hash"`
	Subject string   `json:"subject"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Refs    []string `json:"refs,omitempty"`
}

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// spikeRows returns n deterministic rows sized like a real log window
// (case 3's payload).
func spikeRows(n int) []spikeRow {
	rows := make([]spikeRow, n)
	for i := range rows {
		rows[i] = spikeRow{
			Hash:    fmt.Sprintf("%040x", i+1),
			Subject: fmt.Sprintf("commit message number %d, a realistic subject line length", i),
			Author:  "Jane Committer",
			Date:    epoch.Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
			Refs:    refsFor(i),
		}
	}
	return rows
}

// refsFor gives every 10th row one ref label and every 25th row two, so the
// average payload includes the Refs field without inflating every row.
func refsFor(i int) []string {
	switch {
	case i%25 == 0:
		return []string{"main", "v1.2.0"}
	case i%10 == 0:
		return []string{"main"}
	default:
		return nil
	}
}
