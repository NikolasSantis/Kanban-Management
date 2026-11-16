package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Local guarda os arquivos em disco, dentro de uma pasta base.
type Local struct {
	base string
}

func NewLocal(base string) *Local {
	return &Local{base: base}
}

// path resolve a chave dentro da pasta base e recusa qualquer coisa que escape dela.
func (l *Local) path(key string) (string, error) {
	if key == "" || strings.ContainsRune(key, '\\') || strings.ContainsRune(key, 0) {
		return "", ErrInvalidKey
	}

	for _, part := range strings.Split(key, "/") {
		if part == ".." {
			return "", ErrInvalidKey
		}
	}

	if filepath.IsAbs(key) || strings.HasPrefix(key, "/") {
		return "", ErrInvalidKey
	}

	return filepath.Join(l.base, filepath.FromSlash(key)), nil
}

func (l *Local) Save(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	target, err := l.path(key)

	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)

	if err != nil {
		return err
	}

	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(target)

		return err
	}

	if err := file.Close(); err != nil {
		os.Remove(target)

		return err
	}

	return nil
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	target, err := l.path(key)

	if err != nil {
		return nil, err
	}

	file, err := os.Open(target)

	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	return file, nil
}

// Delete remove o arquivo; apagar o que não existe não é erro.
func (l *Local) Delete(_ context.Context, key string) error {
	target, err := l.path(key)

	if err != nil {
		return err
	}

	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}
