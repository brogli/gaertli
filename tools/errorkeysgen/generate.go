// Command errorkeysgen writes core.ErrorKeys as JSON, so the frontend's
// Vitest suite can check en.json has an errors.* entry for every key
// without importing Go code.
package main

import (
	"encoding/json"

	"github.com/brogli/nebegit/internal/core"
)

// Generate returns core.ErrorKeys as indented JSON, keys sorted (encoding/json
// sorts a map's keys), with a trailing newline.
func Generate() ([]byte, error) {
	data, err := json.MarshalIndent(core.ErrorKeys, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
