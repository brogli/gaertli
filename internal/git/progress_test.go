package git

import "testing"

func TestFR_SYNC_07_ParseProgressLine(t *testing.T) {
	tests := []struct {
		line        string
		wantPercent int
		wantMessage string
		wantOK      bool
	}{
		{"remote: Counting objects:  50% (45/90)   ", 50, "Counting objects:  50% (45/90)", true},
		{"Receiving objects: 100% (90/90), 11.86 KiB | 11.86 MiB/s, done.", 100, "Receiving objects: 100% (90/90), 11.86 KiB | 11.86 MiB/s, done.", true},
		{"Compressing objects: 100% (10/10), done.", 100, "Compressing objects: 100% (10/10), done.", true},
		{"Resolving deltas:   0% (0/29)", 0, "Resolving deltas:   0% (0/29)", true},
		{"remote: Enumerating objects: 90, done.", 0, "Enumerating objects: 90, done.", false},
		{"Cloning into 'repo'...", 0, "Cloning into 'repo'...", false},
		{"remote: Total 90 (delta 29), reused 90 (delta 0), pack-reused 0", 0, "Total 90 (delta 29), reused 90 (delta 0), pack-reused 0", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
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
