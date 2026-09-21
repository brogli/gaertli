//go:build windows

package git

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestTR_12_SysProcAttrHidesWindow(t *testing.T) {
	attr := sysProcAttr()
	if attr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Errorf("CreationFlags = %#x, want CREATE_NO_WINDOW set", attr.CreationFlags)
	}
}
