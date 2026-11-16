// Package healthcheck implementa a flag -healthcheck do binário: a imagem
// distroless não tem curl, então o próprio binário consulta GET /health.
package healthcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// URL monta a URL do /health local. port vazio usa 8000 (o padrão da API).
func URL(port string) string {
	if port == "" {
		port = "8000"
	}

	return "http://127.0.0.1:" + port + "/health"
}

// Check faz GET na URL e devolve erro se a resposta não for 200.
func Check(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: status %d", resp.StatusCode)
	}

	return nil
}
