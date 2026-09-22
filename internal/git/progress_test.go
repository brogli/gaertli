package git

import "testing"

func TestFR_SYNC_07_ParseProgressLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantPercent int
		wantMessage string
		wantOK      bool
	}{
		{
			name:        "remote-side Counting objects with a percent",
			line:        "remote: Counting objects:  50% (45/90)   ",
			wantPercent: 50,
			wantMessage: "Counting objects:  50% (45/90)",
			wantOK:      true,
		},
		{
			name:        "local Receiving objects with a percent",
			line:        "Receiving objects: 100% (90/90), 11.86 KiB | 11.86 MiB/s, done.",
			wantPercent: 100,
			wantMessage: "Receiving objects: 100% (90/90), 11.86 KiB | 11.86 MiB/s, done.",
			wantOK:      true,
		},
		{
			name:        "local Compressing objects with a percent",
			line:        "Compressing objects: 100% (10/10), done.",
			wantPercent: 100,
			wantMessage: "Compressing objects: 100% (10/10), done.",
			wantOK:      true,
		},
		{
			name:        "local Resolving deltas with a percent",
			line:        "Resolving deltas:   0% (0/29)",
			wantPercent: 0,
			wantMessage: "Resolving deltas:   0% (0/29)",
			wantOK:      true,
		},
		{
			name:        "remote-side status line with no percent strips the remote: prefix",
			line:        "remote: Enumerating objects: 90, done.",
			wantMessage: "Enumerating objects: 90, done.",
			wantOK:      false,
		},
		{
			name:        "local status line with no percent",
			line:        "Cloning into 'repo'...",
			wantMessage: "Cloning into 'repo'...",
			wantOK:      false,
		},
		{
			name:        "final summary line with no percent",
			line:        "remote: Total 90 (delta 29), reused 90 (delta 0), pack-reused 0",
			wantMessage: "Total 90 (delta 29), reused 90 (delta 0), pack-reused 0",
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			percent, message, ok := ParseProgressLine([]byte(tt.line))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && percent != tt.wantPercent {
				t.Errorf("percent = %d, want %d", percent, tt.wantPercent)
			}
			if message != tt.wantMessage {
				t.Errorf("message = %q, want %q", message, tt.wantMessage)
			}
		})
	}
}
