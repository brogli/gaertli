//go:build !windows

package remote

// embeddedCore is empty on non-Windows builds: only the Windows shell
// embeds the linux/amd64 core (embed_windows.go).
var embeddedCore []byte
