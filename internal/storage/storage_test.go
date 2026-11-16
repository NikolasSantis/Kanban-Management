package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"kanban-management/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contract roda os mesmos testes em qualquer implementação de Storage.
func contract(t *testing.T, store Storage) {
	t.Helper()

	ctx := context.Background()
	key := "cards/c1/a1/foto.png"
	payload := []byte("conteúdo do arquivo")

	require.NoError(t, store.Save(ctx, key, bytes.NewReader(payload), int64(len(payload)), "image/png"))

	reader, err := store.Open(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, payload, got)

	require.NoError(t, store.Delete(ctx, key))

	_, err = store.Open(ctx, key)
	assert.ErrorIs(t, err, ErrNotFound)

	assert.NoError(t, store.Delete(ctx, key), "apagar o que não existe não é erro")
}

func TestMemoryStorage(t *testing.T) {
	contract(t, NewMemory())
}

func TestLocalStorage(t *testing.T) {
	contract(t, NewLocal(t.TempDir()))
}

func TestLocalStorage_WritesInsideBaseFolder(t *testing.T) {
	base := t.TempDir()
	store := NewLocal(base)

	require.NoError(t, store.Save(context.Background(), "cards/c1/a1/foto.png", bytes.NewReader([]byte("x")), 1, ""))

	_, err := os.Stat(filepath.Join(base, "cards", "c1", "a1", "foto.png"))
	assert.NoError(t, err, "o arquivo fica em <base>/cards/<card>/<anexo>/<nome>")
}

func TestLocalStorage_RejectsPathTraversal(t *testing.T) {
	store := NewLocal(t.TempDir())
	ctx := context.Background()

	for _, key := range []string{"", "../fora.txt", "a/../../fora.txt", "/etc/passwd", `..\fora.txt`, "a/b/../../../c"} {
		err := store.Save(ctx, key, bytes.NewReader([]byte("x")), 1, "")
		assert.ErrorIs(t, err, ErrInvalidKey, "key=%q", key)

		_, err = store.Open(ctx, key)
		assert.ErrorIs(t, err, ErrInvalidKey, "key=%q", key)
	}
}

func TestLocalStorage_DoesNotOverwrite(t *testing.T) {
	store := NewLocal(t.TempDir())
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, "k/a.txt", bytes.NewReader([]byte("1")), 1, ""))

	assert.Error(t, store.Save(ctx, "k/a.txt", bytes.NewReader([]byte("2")), 1, ""))
}

func TestNew_ChoosesDriver(t *testing.T) {
	assert.IsType(t, &Memory{}, New(&config.Config{StorageDriver: "memory"}))
	assert.IsType(t, &Local{}, New(&config.Config{StorageDriver: "local", StorageLocalPath: t.TempDir()}))
}

func TestMemory_HasAndLen(t *testing.T) {
	m := NewMemory()
	require.NoError(t, m.Save(context.Background(), "a", bytes.NewReader([]byte("1")), 1, ""))

	assert.True(t, m.Has("a"))
	assert.False(t, m.Has("b"))
	assert.Equal(t, 1, m.Len())
}
