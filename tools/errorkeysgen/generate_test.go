package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFR_UX_11_GeneratedErrorKeysJSONIsCurrent fails if core.ErrorKeys and
// the checked-in frontend/src/locales/errorKeys.generated.json have drifted
// apart.
func TestFR_UX_11_GeneratedErrorKeysJSONIsCurrent(t *testing.T) {
	want, err := Generate()
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("..", "..", "frontend", "src", "locales", "errorKeys.generated.json")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Errorf("%s is stale; regenerate with: go generate ./internal/core", path)
	}
}
