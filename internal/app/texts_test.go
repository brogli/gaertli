// Requirements: FR-UX-11, QA-PORT-05

package app

import (
	"strings"
	"testing"
)

func TestTheConsentNamesTheDistro(t *testing.T) {
	got := consentMessage("Ubuntu")

	if !strings.Contains(got, "Ubuntu") {
		t.Errorf("consent = %q, want it to name the distro", got)
	}
}
