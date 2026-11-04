//go:build !windows

package devresult

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openContainedSource(root, relative string) (*os.File, error) {
	current, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for index, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
		if index < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(current, part, flags, 0)
		_ = unix.Close(current)
		if openErr != nil {
			return nil, openErr
		}
		current = next
	}
	file := os.NewFile(uintptr(current), fmt.Sprintf("%s/%s", root, relative))
	if file == nil {
		_ = unix.Close(current)
		return nil, fmt.Errorf("create source file handle")
	}
	return file, nil
}
