// Package server roda o Fiber com encerramento gracioso.
package server

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Run serve o app no listener até ctx ser cancelado (SIGINT/SIGTERM no main).
// Ao cancelar, para de aceitar conexões novas e espera as requests em andamento
// terminarem por até timeout. Devolve nil no encerramento normal; erro se o
// servidor não subiu ou se o timeout estourou.
func Run(ctx context.Context, app *fiber.App, listener net.Listener, timeout time.Duration) error {
	serveErr := make(chan error, 1)

	go func() {
		serveErr <- app.Listener(listener)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("sinal de encerramento recebido: aguardando requests em andamento", "timeout", timeout.String())

	if err := app.ShutdownWithTimeout(timeout); err != nil {
		return err
	}

	return <-serveErr
}
