//go:build windows

package reader

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func moveNewSave(staged, target string) error {
	src, err := syscall.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	dst, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileW")
	result, _, callErr := proc.Call(uintptr(unsafe.Pointer(src)), uintptr(unsafe.Pointer(dst)))
	if result != 0 {
		return nil
	}
	if callErr == syscall.ERROR_ALREADY_EXISTS || callErr == syscall.ERROR_FILE_EXISTS {
		return os.ErrExist
	}
	if _, err := os.Lstat(target); err == nil {
		return os.ErrExist
	}
	return &os.LinkError{Op: "MoveFileW", Old: staged, New: target, Err: callErr}
}

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
