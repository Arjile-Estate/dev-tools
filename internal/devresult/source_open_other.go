//go:build !linux

package devresult

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func openContainedSource(root, relative string) (*os.File, error) {
	current := root
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("source path contains a symbolic link")
		}
	}
	return os.Open(current)
}
