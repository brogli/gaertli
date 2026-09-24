package git

import "testing"

func TestMaskCredentials(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"https user and token": {
			"https://alice:s3cr3t@example.com/x.git",
			"https://***@example.com/x.git",
		},
		"token only, no password": {
			"https://ghp_abc123@example.com/x.git",
			"https://***@example.com/x.git",
		},
		"ssh user, no password": {
			"ssh://git@example.com/x.git",
			"ssh://***@example.com/x.git",
		},
		"with a port": {
			"https://alice:s3cr3t@example.com:8080/x.git",
			"https://***@example.com:8080/x.git",
		},
		"several URLs in one text": {
			"fatal: tried 'https://a:b@host1/x' then 'ssh://c@host2/y'",
			"fatal: tried 'https://***@host1/x' then 'ssh://***@host2/y'",
		},
		"no URL": {
			"fatal: not a git repository",
			"fatal: not a git repository",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := MaskCredentials(tt.text); got != tt.want {
				t.Errorf("MaskCredentials(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
