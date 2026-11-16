// Package storage guarda os bytes dos anexos. Os handlers só conhecem a
// interface Storage: trocar o disco por S3/MinIO é implementar a mesma interface.
package storage

import (
	"context"
	"errors"
	"io"

	"kanban-management/config"
)

// ErrNotFound: a chave não existe no storage.
var ErrNotFound = errors.New("storage: object not found")

// ErrInvalidKey: chave vazia, absoluta ou com ".." (path traversal).
var ErrInvalidKey = errors.New("storage: invalid key")

type Storage interface {
	Save(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// New escolhe o driver por cfg.StorageDriver (a config já validou o valor):
// "memory" para testes, qualquer outro valor = disco local.
func New(cfg *config.Config) Storage {
	if cfg.StorageDriver == "memory" {
		return NewMemory()
	}

	return NewLocal(cfg.StorageLocalPath)
}
