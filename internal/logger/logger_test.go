package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"kanban-management/config"
)

func TestSetupJSONAndLevel(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	defer slog.SetDefault(prev)

	l := setup(&config.Config{LogLevel: "warn", LogFormat: "json"}, &buf)
	l.Info("ignorado")
	l.Warn("visivel", "k", "v")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("esperava 1 linha, veio %d: %q", len(lines), buf.String())
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil || m["msg"] != "visivel" {
		t.Errorf("JSON inválido ou inesperado: %v %v", err, m)
	}
}

func TestSetupText(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	defer slog.SetDefault(prev)

	setup(&config.Config{LogLevel: "info", LogFormat: "text"}, &buf).Info("oi")

	if !strings.Contains(buf.String(), "level=INFO") {
		t.Errorf("saída de texto inesperada: %q", buf.String())
	}
}
