package storage

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// Memory é o FakeStorage: guarda tudo em memória. Usado nos testes.
type Memory struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemory() *Memory {
	return &Memory{objects: map[string][]byte{}}
}

func (m *Memory) Save(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	if key == "" {
		return ErrInvalidKey
	}

	data, err := io.ReadAll(r)

	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.objects[key] = data

	return nil
}

func (m *Memory) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.objects[key]

	if !ok {
		return nil, ErrNotFound
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.objects, key)

	return nil
}

// Has diz se a chave existe (útil nos testes).
func (m *Memory) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.objects[key]

	return ok
}

// Len devolve quantos objetos existem (útil nos testes).
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.objects)
}
