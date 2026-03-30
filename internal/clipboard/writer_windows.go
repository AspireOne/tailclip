//go:build windows

package clipboard

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gmemMoveable = 0x0002
)

var (
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
)

func SetText(text string) error {
	if err := openClipboardWithRetry(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	if r1, _, err := procEmptyClipboard.Call(); r1 == 0 {
		return fmt.Errorf("EmptyClipboard failed: %w", err)
	}

	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}

	size := uintptr(len(utf16) * int(unsafe.Sizeof(uint16(0))))
	handle, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if handle == 0 {
		return fmt.Errorf("GlobalAlloc failed: %w", err)
	}

	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed: %w", err)
	}

	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16)), utf16)

	if r1, _, err := procGlobalUnlock.Call(handle); r1 == 0 && err != windows.ERROR_SUCCESS {
		return fmt.Errorf("GlobalUnlock failed: %w", err)
	}

	if r1, _, err := procSetClipboardData.Call(cfUnicodeText, handle); r1 == 0 {
		return fmt.Errorf("SetClipboardData failed: %w", err)
	}

	return nil
}
