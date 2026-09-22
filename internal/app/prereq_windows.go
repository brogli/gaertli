//go:build windows

package app

import "golang.org/x/sys/windows"

// showFatal shows msg before any window exists. No Wails window can be
// assumed working in either failure case, so this is a native
// message box.
func showFatal(title, msg string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
