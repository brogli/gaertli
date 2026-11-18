// Requirements: FR-UX-11

package core_test

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/brogli/gaertli/internal/core"
)

// TestEveryErrorKeyHasACatalogueEntryWithItsArgs fails if the
// frontend's en.json lacks an errors.<key> entry for a key of ErrorKeys, or
// the entry's {placeholders} are not exactly that key's args.
func TestEveryErrorKeyHasACatalogueEntryWithItsArgs(t *testing.T) {
	data, err := os.ReadFile("../../frontend/src/locales/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(data, &en); err != nil {
		t.Fatal(err)
	}

	placeholder := regexp.MustCompile(`\{(\w+)\}`)
	for key, args := range core.ErrorKeys {
		text, ok := en.Errors[key]
		if !ok {
			t.Errorf("en.json has no errors.%s", key)
			continue
		}
		var used []string
		for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
			used = append(used, m[1])
		}
		slices.Sort(used)
		used = slices.Compact(used)
		if want := slices.Sorted(slices.Values(args)); !slices.Equal(used, want) {
			t.Errorf("errors.%s uses {%v}, want its args {%v}", key, used, want)
		}
	}
}
