//go:build linux

package devresult

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func openContainedSource(root, relative string) (*os.File, error) {
	current, err := syscall.Open(root, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for index, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NOFOLLOW
		if index < len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		next, openErr := syscall.Openat(current, part, flags, 0)
		_ = syscall.Close(current)
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	file := os.NewFile(uintptr(current), fmt.Sprintf("%s/%s", root, relative))
	if file == nil {
		_ = syscall.Close(current)
		return nil, fmt.Errorf("create source file handle")
	}
	return file, nil
}
