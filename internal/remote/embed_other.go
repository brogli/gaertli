//go:build !windows

package remote

// embeddedCore is empty on non-Windows builds: only the Windows shell
// embeds the linux/amd64 core (ADR-3, components.md's "Linux and macOS
// builds do not embed it"). embed_windows.go supplies the real bytes.
var embeddedCore []byte
