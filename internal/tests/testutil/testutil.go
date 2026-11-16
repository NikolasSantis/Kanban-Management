// Package testutil reúne a infraestrutura dos testes de integração HTTP.
//
// Segurança: os testes NUNCA rodam contra o banco principal. A inicialização só
// prossegue se TEST_DATABASE_NAME for diferente de DATABASE_NAME e terminar em
// "_test" (ver AssertTestDatabase). Só o banco de teste sofre Drop.
package testutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kanban-management/config"
	"kanban-management/internal/app"
	"kanban-management/internal/database"
	"kanban-management/internal/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Envelope é o envelope padrão de resposta da API.
type Envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

// Map decodifica Data como objeto JSON.
func (e Envelope) Map() map[string]any {
	var m map[string]any
	_ = json.Unmarshal(e.Data, &m)
	return m
}

// List decodifica Data como array JSON de objetos.
func (e Envelope) List() []map[string]any {
	var l []map[string]any
	_ = json.Unmarshal(e.Data, &l)
	return l
}

var (
	once    sync.Once
	testApp *fiber.App
	initErr error
	skipMsg string
	// testDBName guarda o nome do banco de teste em uso (para o Drop conferir).
	testDBName string
	mainDBName string
)

// AssertTestDatabase é a trava de segurança: recusa o banco principal e
// qualquer nome que não termine em "_test".
func AssertTestDatabase(mainDB, testDB string) error {
	if testDB == "" {
		return fmt.Errorf("TEST_DATABASE_NAME vazio")
	}
	if testDB == mainDB {
		return fmt.Errorf("TEST_DATABASE_NAME (%q) é igual a DATABASE_NAME: abortando para não tocar no banco principal", testDB)
	}
	if !strings.HasSuffix(testDB, "_test") {
		return fmt.Errorf("TEST_DATABASE_NAME (%q) precisa terminar em _test", testDB)
	}
	return nil
}

// loadDotEnv procura o .env subindo as pastas a partir do diretório atual
// (os testes rodam dentro de internal/tests/...). Não sobrescreve variáveis
// já definidas no ambiente.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func initialize() {
	_ = os.Setenv("APP_ENV", "test")
	loadDotEnv()

	if os.Getenv("MONGODB_URI") == "" {
		skipMsg = "MONGODB_URI não definido: testes de integração pulados"
		return
	}

	cfg, err := config.Load()
	if err != nil {
		skipMsg = "configuração incompleta: testes de integração pulados (" + err.Error() + ")"
		return
	}

	mainDBName = cfg.DatabaseName
	testDBName = cfg.TestDatabaseName

	if err := AssertTestDatabase(mainDBName, testDBName); err != nil {
		initErr = err
		return
	}

	// A partir daqui tudo usa o banco de teste.
	cfg.DatabaseName = testDBName
	logger.Setup(cfg)

	if err := database.ConnectDB(cfg); err != nil {
		initErr = fmt.Errorf("falha ao conectar no banco de teste: %w", err)
		return
	}

	// Começa de um banco limpo (somente o de teste) e recria os índices.
	if err := DropTestDatabase(); err != nil {
		initErr = err
		return
	}
	if err := database.CreateIndexes(); err != nil {
		initErr = err
		return
	}

	testApp = app.New(cfg)
}

// DropTestDatabase apaga todas as coleções do banco de teste. Recusa qualquer
// outro banco (a trava roda antes de qualquer Drop).
func DropTestDatabase() error {
	if database.DB == nil {
		return nil
	}

	if err := AssertTestDatabase(mainDBName, database.DB.Name()); err != nil {
		return err
	}

	// O usuário do Atlas não tem permissão de dropDatabase, então apaga cada
	// coleção do banco de teste (o efeito é o mesmo).
	ctx := context.Background()

	names, err := database.DB.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := database.DB.Collection(name).Drop(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Setup devolve o app de teste ou pula o teste se não houver banco configurado.
func Setup(t *testing.T) *fiber.App {
	t.Helper()

	once.Do(initialize)

	if skipMsg != "" {
		t.Skip(skipMsg)
	}
	require.NoError(t, initErr)

	return testApp
}

// Cleanup apaga o banco de teste ao final da suíte (chamar no TestMain).
func Cleanup() {
	if testApp == nil {
		return
	}
	if err := DropTestDatabase(); err != nil {
		fmt.Fprintln(os.Stderr, "falha ao limpar banco de teste:", err)
	}
}

// Do faz uma request JSON e devolve status e envelope.
func Do(t *testing.T, app *fiber.App, method, path, token string, body any) (int, Envelope) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var env Envelope
	require.NoError(t, json.Unmarshal(raw, &env), "resposta não é o envelope padrão: %s", raw)

	return resp.StatusCode, env
}

// RandomString devolve um sufixo aleatório para nomes e e-mails únicos.
func RandomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// User é um usuário de teste já autenticado.
type User struct {
	ID    string
	Email string
	Token string
}

// CreateUser registra um usuário com e-mail aleatório e faz login.
func CreateUser(t *testing.T, app *fiber.App) User {
	t.Helper()

	email := "u-" + RandomString(6) + "@test.local"
	password := "Senha-" + RandomString(4)

	status, env := Do(t, app, "POST", "/auth/register", "", map[string]any{
		"name": "Teste " + RandomString(3), "email": email, "password": password,
	})
	require.Equal(t, 200, status, "register: %v", env.Error)

	status, env = Do(t, app, "POST", "/auth/login", "", map[string]any{"email": email, "password": password})
	require.Equal(t, 200, status, "login: %v", env.Error)
	token, _ := env.Map()["token"].(string)
	require.NotEmpty(t, token)

	status, env = Do(t, app, "GET", "/auth/me", token, nil)
	require.Equal(t, 200, status)
	id, _ := env.Map()["id"].(string)
	require.NotEmpty(t, id)

	return User{ID: id, Email: email, Token: token}
}

func findByName(t *testing.T, app *fiber.App, listPath, token, name string) string {
	t.Helper()

	status, env := Do(t, app, "GET", listPath, token, nil)
	require.Equal(t, 200, status)

	for _, item := range env.List() {
		if item["name"] == name {
			id, _ := item["id"].(string)
			return id
		}
	}

	require.FailNow(t, "item não encontrado na listagem", "%s %s", listPath, name)
	return ""
}

// CreateWorkspace cria um workspace e devolve o id (o criador vira admin).
func CreateWorkspace(t *testing.T, app *fiber.App, token string) string {
	t.Helper()

	name := "ws-" + RandomString(4)
	status, env := Do(t, app, "POST", "/workspace/", token, map[string]any{"name": name, "description": "teste"})
	require.Equal(t, 200, status, "create workspace: %v", env.Error)

	return findByName(t, app, "/workspace/", token, name)
}

// CreateProject cria um projeto no workspace e devolve o id (o criador vira admin).
func CreateProject(t *testing.T, app *fiber.App, token, workspaceID string) string {
	t.Helper()

	name := "proj-" + RandomString(4)
	status, env := Do(t, app, "POST", "/project/", token, map[string]any{
		"workspace_id": workspaceID, "name": name, "description": "teste",
	})
	require.Equal(t, 201, status, "create project: %v", env.Error)

	return findByName(t, app, "/project/", token, name)
}

// AddProjectMember adiciona userID ao projeto com a role informada.
func AddProjectMember(t *testing.T, app *fiber.App, adminToken, projectID, userID, role string) {
	t.Helper()

	status, env := Do(t, app, "POST", "/project/member/"+projectID, adminToken, map[string]any{
		"user_id": userID, "role": role,
	})
	require.Equal(t, 201, status, "add member: %v", env.Error)
}

// CreateBoard cria um board no projeto e devolve o id.
func CreateBoard(t *testing.T, app *fiber.App, token, projectID string) string {
	t.Helper()

	status, env := Do(t, app, "POST", "/board/project/"+projectID, token, map[string]any{"name": "board-" + RandomString(3)})
	require.Equal(t, 201, status, "create board: %v", env.Error)

	id, _ := env.Map()["id"].(string)
	require.NotEmpty(t, id)

	return id
}
