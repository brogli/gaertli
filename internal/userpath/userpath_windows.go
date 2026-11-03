//go:build windows

package userpath

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const pathValueName = "Path"

// Add puts dir at the end of the user PATH, once, and tells running
// processes about the change.
func Add(dir string) error {
	return editUserPath(func(path string) string { return withEntry(path, dir) })
}

// Remove drops dir from the user PATH and tells running processes about the
// change.
func Remove(dir string) error {
	return editUserPath(func(path string) string { return withoutEntry(path, dir) })
}

// editUserPath opens HKCU\Environment and applies update to it.
func editUserPath(update func(string) string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	return updateKey(key, update)
}

// updateKey reads Path from key, applies edit, and writes the result back
// as REG_EXPAND_SZ only when it changed, then broadcasts the change so
// terminals started from Explorer see it. It takes key as a parameter so
// tests can use a scratch key instead of the real HKCU\Environment.
func updateKey(key registry.Key, edit func(string) string) error {
	current, _, err := key.GetStringValue(pathValueName)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	updated := edit(current)
	if updated == current {
		return nil
	}
	if err := key.SetExpandStringValue(pathValueName, updated); err != nil {
		return err
	}
	broadcastEnvironmentChange()
	return nil
}

const (
	hwndBroadcast      = 0xffff
	wmSettingChange    = 0x001a
	smtoAbortIfHung    = 0x0002
	broadcastTimeoutMs = 5000
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

// broadcastEnvironmentChange tells running processes, including terminals
// started from Explorer, that an environment variable changed without a new
// login (WM_SETTINGCHANGE, lParam "Environment").
func broadcastEnvironmentChange() {
	param, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	_, _, _ = procSendMessageTimeout.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(param)),
		smtoAbortIfHung,
		broadcastTimeoutMs,
		0,
	)
}
