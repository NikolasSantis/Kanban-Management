package app

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"kanban-management/config"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp monta o app sem banco (a documentação e o registro das rotas não precisam dele).
func newTestApp(swaggerEnabled bool) *fiber.App {
	return New(&config.Config{
		AppEnv:              "test",
		LogFormat:           "json",
		AttachmentMaxSizeMB: 10,
		StorageDriver:       "memory",
		SwaggerEnabled:      swaggerEnabled,
	})
}

type swaggerDoc struct {
	Swagger             string                          `json:"swagger"`
	Paths               map[string]map[string]swaggerOp `json:"paths"`
	Definitions         map[string]swaggerDefinition    `json:"definitions"`
	SecurityDefinitions map[string]map[string]any       `json:"securityDefinitions"`
	Tags                []map[string]any                `json:"tags"`
	Info                map[string]any                  `json:"info"`
}

type swaggerOp struct {
	Tags     []string           `json:"tags"`
	Security []map[string][]any `json:"security"`
	Summary  string             `json:"summary"`
	Params   []map[string]any   `json:"parameters"`
	Resp     map[string]any     `json:"responses"`
	Consumes []string           `json:"consumes"`
	Produces []string           `json:"produces"`
}

type swaggerDefinition struct {
	Properties map[string]json.RawMessage `json:"properties"`
}

func fetchDoc(t *testing.T, app *fiber.App) swaggerDoc {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest("GET", "/swagger/doc.json", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, 200, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var doc swaggerDoc
	require.NoError(t, json.Unmarshal(raw, &doc))

	return doc
}

func normalizePath(path string) string {
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}

	path = strings.Join(parts, "/")

	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}

	return path
}

func TestSwagger_DocJSONIsServed(t *testing.T) {
	doc := fetchDoc(t, newTestApp(true))

	assert.Equal(t, "2.0", doc.Swagger)
	assert.Contains(t, doc.Paths, "/board/{board_id}")
	assert.Equal(t, "Kanban Management API", doc.Info["title"])
}

func TestSwagger_UIIsServed(t *testing.T) {
	resp, err := newTestApp(true).Test(httptest.NewRequest("GET", "/swagger/index.html", nil), -1)

	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestSwagger_CanBeDisabled(t *testing.T) {
	app := newTestApp(false)

	for _, path := range []string{"/swagger/doc.json", "/swagger/index.html"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil), -1)

		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, 404, resp.StatusCode, path)
	}
}

func TestSwagger_EveryRegisteredRouteIsDocumented(t *testing.T) {
	app := newTestApp(true)
	doc := fetchDoc(t, app)

	documented := map[string]bool{}

	for path, operations := range doc.Paths {
		for method := range operations {
			documented[strings.ToUpper(method)+" "+normalizePath(path)] = true
		}
	}

	var missing []string
	checked := 0

	for _, route := range app.GetRoutes(true) {
		if route.Method == "HEAD" || strings.HasPrefix(route.Path, "/swagger") {
			continue
		}

		checked++
		key := route.Method + " " + normalizePath(route.Path)

		if !documented[key] {
			missing = append(missing, key)
		}
	}

	sort.Strings(missing)

	assert.Empty(t, missing, "rotas registradas que não aparecem no Swagger")
	assert.Greater(t, checked, 50, "o teste precisa estar enxergando as rotas")
}

func TestSwagger_NoSensitiveFieldsInSchemas(t *testing.T) {
	doc := fetchDoc(t, newTestApp(true))
	require.NotEmpty(t, doc.Definitions)

	internalFields := []string{"storage_key", "search_text", "priority_level", "name_normalized", "active"}

	for name, definition := range doc.Definitions {
		for field := range definition.Properties {
			assert.NotContains(t, internalFields, field, "schema %s expõe o campo interno %q", name, field)

			// Só os corpos de request (login/cadastro) têm senha.
			if field == "password" {
				assert.True(t, strings.HasSuffix(name, "Request"), "schema %s expõe password", name)
			}
		}
	}
}

func TestSwagger_BearerAuthOnProtectedRoutesOnly(t *testing.T) {
	doc := fetchDoc(t, newTestApp(true))

	require.Contains(t, doc.SecurityDefinitions, "BearerAuth")
	assert.Equal(t, "header", doc.SecurityDefinitions["BearerAuth"]["in"])
	assert.Equal(t, "Authorization", doc.SecurityDefinitions["BearerAuth"]["name"])

	public := map[string]bool{"/auth/login": true, "/auth/register": true, "/health": true}

	for path, operations := range doc.Paths {
		for method, operation := range operations {
			if public[path] {
				assert.Empty(t, operation.Security, "%s %s é pública", method, path)
				continue
			}

			require.NotEmpty(t, operation.Security, "%s %s precisa de BearerAuth", method, path)
			assert.Contains(t, operation.Security[0], "BearerAuth", "%s %s", method, path)
		}
	}
}

func TestSwagger_AllTagsAreListed(t *testing.T) {
	doc := fetchDoc(t, newTestApp(true))

	listed := map[string]bool{}
	for _, tag := range doc.Tags {
		listed[tag["name"].(string)] = true
	}

	for _, tag := range []string{
		"auth", "users", "workspaces", "workspace-members", "projects", "project-members", "boards",
		"columns", "cards", "labels", "comments", "attachments", "health",
	} {
		assert.True(t, listed[tag], "tag %s", tag)
	}

	used := map[string]bool{}

	for _, operations := range doc.Paths {
		for _, operation := range operations {
			for _, tag := range operation.Tags {
				used[tag] = true
			}
		}
	}

	for tag := range listed {
		assert.True(t, used[tag], "a tag %s não tem nenhuma rota", tag)
	}
}

func TestSwagger_ListRoutesDocumentPaginationAndFilters(t *testing.T) {
	doc := fetchDoc(t, newTestApp(true))

	paramNames := func(path, method string) []string {
		var names []string

		for _, param := range doc.Paths[path][method].Params {
			names = append(names, param["name"].(string))
		}

		return names
	}

	cards := paramNames("/card/board/{board_id}", "get")

	for _, want := range []string{"board_id", "page", "limit", "column_id", "priority", "label_ids", "label_mode", "assignee_id",
		"created_by", "status", "due_before", "due_after", "overdue", "has_due_date", "q", "sort", "order"} {
		assert.Contains(t, cards, want)
	}

	assert.Contains(t, paramNames("/project/", "get"), "workspace_id")
	assert.Contains(t, paramNames("/attachment/card/{card_id}", "get"), "kind")
	assert.Contains(t, paramNames("/comment/card/{card_id}", "get"), "author_id")
	assert.Contains(t, paramNames("/project/member/{id}", "get"), "role")

	upload := doc.Paths["/attachment/card/{card_id}"]["post"]
	assert.Contains(t, upload.Consumes, "multipart/form-data")

	var hasFileParam bool

	for _, param := range upload.Params {
		if param["name"] == "file" && param["in"] == "formData" && param["type"] == "file" {
			hasFileParam = true
		}
	}

	assert.True(t, hasFileParam, "o upload documenta o campo file (formData)")
}
