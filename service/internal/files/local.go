package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SafeJoin(root, area, key string) (string, error) {
	if area != "public" && area != "private" {
		return "", errors.New("unknown_storage")
	}
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) {
		return "", errors.New("invalid_key")
	}
	return filepath.Join(root, area, key), nil
}

type Local struct {
	Root string
}

func (l Local) Put(area, key string, r io.Reader) error {
	path, err := SafeJoin(l.Root, area, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, r)
	return err
}

func (l Local) Open(area, key string) (*os.File, error) {
	path, err := SafeJoin(l.Root, area, key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
