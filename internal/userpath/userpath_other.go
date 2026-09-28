//go:build !windows

package userpath

import "errors"

// errUnsupported is what Add and Remove return outside Windows: the user
// PATH they edit is a Windows-only concept.
var errUnsupported = errors.New("the user PATH is a Windows-only concept")

// Add fails outside Windows.
func Add(string) error {
	return errUnsupported
}

// Remove fails outside Windows.
func Remove(string) error {
	return errUnsupported
}
