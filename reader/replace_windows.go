//go:build windows

package reader

import (
	"fmt"
	"syscall"
	"unsafe"
)

func replaceExistingSave(original, staged string) error {
	oldName, err := syscall.UTF16PtrFromString(original)
	if err != nil {
		return err
	}
	newName, err := syscall.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")
	result, _, callErr := proc.Call(uintptr(unsafe.Pointer(oldName)), uintptr(unsafe.Pointer(newName)), 0, 0, 0, 0)
	if result == 0 {
		return fmt.Errorf("remplacement atomique impossible : %w", callErr)
	}
	return nil
}
