package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func CopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory not found: %s: %w", src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", src)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFile(path, target)
	})
}

func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source file not found: %s: %w", src, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source path is not a regular file: %s", src)
	}
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, input, info.Mode().Perm())
}
