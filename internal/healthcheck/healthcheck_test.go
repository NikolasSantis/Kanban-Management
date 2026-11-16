package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:8000/health", URL(""))
	assert.Equal(t, "http://127.0.0.1:9090/health", URL("9090"))
}

func TestCheck(t *testing.T) {
	status := http.StatusOK

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(status)
	}))
	defer server.Close()

	assert.NoError(t, Check(server.URL+"/health", time.Second), "200 é saudável")

	status = http.StatusServiceUnavailable
	assert.Error(t, Check(server.URL+"/health", time.Second), "503 é não saudável")

	status = http.StatusNotFound
	assert.Error(t, Check(server.URL+"/health", time.Second), "qualquer status diferente de 200 falha")
}

func TestCheck_Unreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL + "/health"
	server.Close()

	assert.Error(t, Check(url, time.Second), "sem servidor escutando")
}

func TestCheck_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	start := time.Now()

	assert.Error(t, Check(server.URL+"/health", 100*time.Millisecond))
	assert.Less(t, time.Since(start), 400*time.Millisecond)
}
