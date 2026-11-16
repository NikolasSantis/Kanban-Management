package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newApp(delay time.Duration, started chan<- struct{}) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/slow", func(c *fiber.Ctx) error {
		if started != nil {
			close(started)
		}

		time.Sleep(delay)

		return c.SendString("terminou")
	})

	return app
}

func listen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return listener
}

func TestRun_LetsInFlightRequestsFinishOnShutdown(t *testing.T) {
	started := make(chan struct{})
	listener := listen(t)
	url := "http://" + listener.Addr().String() + "/slow"

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)

	go func() { runErr <- Run(ctx, newApp(500*time.Millisecond, started), listener, 5*time.Second) }()

	type result struct {
		status int
		body   string
		err    error
	}

	responses := make(chan result, 1)

	go func() {
		resp, err := http.Get(url)

		if err != nil {
			responses <- result{err: err}
			return
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		responses <- result{status: resp.StatusCode, body: string(body)}
	}()

	<-started // a request está em andamento
	cancel()  // equivale ao SIGTERM

	got := <-responses
	require.NoError(t, got.err, "a request em andamento não pode ser cortada")
	assert.Equal(t, 200, got.status)
	assert.Equal(t, "terminou", got.body)

	select {
	case err := <-runErr:
		assert.NoError(t, err, "encerramento normal devolve nil")
	case <-time.After(5 * time.Second):
		t.Fatal("Run não terminou depois do shutdown")
	}

	_, err := http.Get(url)
	assert.Error(t, err, "depois do shutdown não aceita conexões novas")
}

func TestRun_ReturnsErrorWhenTimeoutExpires(t *testing.T) {
	started := make(chan struct{})
	listener := listen(t)
	url := "http://" + listener.Addr().String() + "/slow"

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)

	go func() { runErr <- Run(ctx, newApp(2*time.Second, started), listener, 100*time.Millisecond) }()

	go func() {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
		}
	}()

	<-started
	cancel()

	select {
	case err := <-runErr:
		assert.Error(t, err, "request que passa do timeout faz o shutdown falhar")
	case <-time.After(5 * time.Second):
		t.Fatal("Run não respeitou o timeout")
	}
}

func TestRun_StopsRightAwayWhenNothingIsInFlight(t *testing.T) {
	listener := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)

	go func() { runErr <- Run(ctx, newApp(0, nil), listener, 5*time.Second) }()

	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	cancel()

	select {
	case err := <-runErr:
		assert.NoError(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("Run não terminou")
	}
}
