//go:build windows

package reader

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

//go:embed data/libzstd.dll
var zstdDLL []byte

// compressFMFrame reproduces the checked Zstandard frame shape accepted by
// FM27 in the controlled v6-v10 save experiments. The library is bundled so
// the Windows executable needs no Git or Python installation at runtime.
func compressFMFrame(src []byte) ([]byte, error) {
	file, err := os.CreateTemp("", "pauscout-zstd-*.dll")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err = file.Write(zstdDLL); err != nil {
		file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("chargement Zstandard: %w", err)
	}
	defer dll.Release()
	proc := func(name string) (*syscall.Proc, error) { return dll.FindProc(name) }
	create, err := proc("ZSTD_createCCtx")
	if err != nil {
		return nil, err
	}
	free, err := proc("ZSTD_freeCCtx")
	if err != nil {
		return nil, err
	}
	set, err := proc("ZSTD_CCtx_setParameter")
	if err != nil {
		return nil, err
	}
	bound, err := proc("ZSTD_compressBound")
	if err != nil {
		return nil, err
	}
	compress, err := proc("ZSTD_compress2")
	if err != nil {
		return nil, err
	}
	isError, err := proc("ZSTD_isError")
	if err != nil {
		return nil, err
	}
	ctx, _, _ := create.Call()
	if ctx == 0 {
		return nil, fmt.Errorf("allocation Zstandard impossible")
	}
	defer free.Call(ctx)
	check := func(value uintptr) error {
		flag, _, _ := isError.Call(value)
		if flag != 0 {
			return fmt.Errorf("erreur de compression Zstandard (%d)", value)
		}
		return nil
	}
	for _, option := range [][2]int{{100, 3}, {101, 20}, {200, 0}, {201, 1}} {
		value, _, _ := set.Call(ctx, uintptr(option[0]), uintptr(option[1]))
		if err := check(value); err != nil {
			return nil, err
		}
	}
	capacity, _, _ := bound.Call(uintptr(len(src)))
	if err := check(capacity); err != nil {
		return nil, err
	}
	if capacity == 0 || capacity > 1<<30 {
		return nil, fmt.Errorf("taille Zstandard excessive")
	}
	dst := make([]byte, int(capacity))
	var source uintptr
	if len(src) > 0 {
		source = uintptr(unsafe.Pointer(&src[0]))
	}
	value, _, _ := compress.Call(ctx, uintptr(unsafe.Pointer(&dst[0])), uintptr(len(dst)), source, uintptr(len(src)))
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)
	if err := check(value); err != nil {
		return nil, err
	}
	return dst[:int(value)], nil
}
