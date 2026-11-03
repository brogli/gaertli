//go:build windows

package app

import "golang.org/x/sys/windows"

// showFatal shows msg before any window exists. No Wails window can be
// assumed working, so this is a native message box.
func showFatal(title, msg string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}

// idYes is MessageBox's return value for its Yes button; golang.org/x/sys/windows
// has the MB_* input flags but not the ID* results.
const idYes = 6

// askYesNo shows a native yes/no question, usable before and after Wails'
// own App.Run exists.
func askYesNo(title, msg string) bool {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	r, _ := windows.MessageBox(0, m, t, windows.MB_YESNO|windows.MB_ICONQUESTION|windows.MB_SETFOREGROUND|windows.MB_TOPMOST)
	return r == idYes
}
