//go:build windows

package devresult

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func openContainedSource(root, relative string) (*os.File, error) {
	requested := filepath.Join(root, relative)
	pointer, err := windows.UTF16PtrFromString(requested)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	closeHandle := true
	defer func() {
		if closeHandle {
			_ = windows.CloseHandle(handle)
		}
	}()

	buffer := make([]uint16, 32768)
	length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return nil, err
	}
	if length == 0 || length >= uint32(len(buffer)) {
		return nil, fmt.Errorf("resolved source path is too long")
	}
	resolved := normalizeWindowsHandlePath(windows.UTF16ToString(buffer[:length]))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	contained, err := pathIsWithin(resolvedRoot, resolved)
	if err != nil || !contained {
		return nil, fmt.Errorf("resolved source path escapes run directory")
	}
	file := os.NewFile(uintptr(handle), requested)
	if file == nil {
		return nil, fmt.Errorf("create source file handle")
	}
	closeHandle = false
	return file, nil
}

func normalizeWindowsHandlePath(path string) string {
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	}
	return strings.TrimPrefix(path, `\\?\`)
}
