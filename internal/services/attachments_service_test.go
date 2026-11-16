package services_test

import (
	"strings"
	"testing"

	"kanban-management/internal/services"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSanitizeFileName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"normal", "foto.png", "foto.png"},
		{"path traversal", "../../../evil.png", "evil.png"},
		{"path traversal profundo", "../../etc/passwd", "passwd"},
		{"contrabarras do Windows", `C:\Users\x\..\doc.pdf`, "doc.pdf"},
		{"acentos e espaços são removidos", "relatório final 2026.pdf", "relatriofinal2026.pdf"},
		{"sem extensão", "LEIAME", "LEIAME"},
		{"só caracteres inválidos", "çãõ", "file"},
		{"só pontos", "..", "file"},
		{"vazio", "", "file"},
		{"arquivo oculto perde os pontos do início", ".htaccess", "htaccess"},
		{"mantém a extensão composta pela última", "backup.tar.gz", "backup.tar.gz"},
		{"nulo e controle", "a\x00b\n.png", "ab.png"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, services.SanitizeFileName(tc.in))
		})
	}

	t.Run("nome gigante é limitado a 100 caracteres mantendo a extensão", func(t *testing.T) {
		got := services.SanitizeFileName(strings.Repeat("a", 300) + ".png")

		assert.Len(t, got, 100)
		assert.True(t, strings.HasSuffix(got, ".png"))
	})

	t.Run("extensão gigante também é limitada", func(t *testing.T) {
		got := services.SanitizeFileName("a." + strings.Repeat("x", 300))

		assert.LessOrEqual(t, len(got), 100)
	})

	t.Run("nunca contém separadores de caminho", func(t *testing.T) {
		for _, in := range []string{"a/b", `a\b`, "../x", "/abs/x", "x/..", "..\\..\\x"} {
			got := services.SanitizeFileName(in)

			assert.NotContains(t, got, "/")
			assert.NotContains(t, got, `\`)
			assert.NotEqual(t, "..", got)
			assert.NotEmpty(t, got)
		}
	})
}

func TestDisplayName(t *testing.T) {
	assert.Equal(t, "foto.png", services.DisplayName("../../foto.png"))
	assert.Equal(t, "relatório final.pdf", services.DisplayName("relatório final.pdf"))
	assert.Equal(t, "ab.png", services.DisplayName("a\x00b.png"))
	assert.Equal(t, "file", services.DisplayName("   "))
	assert.Len(t, []rune(services.DisplayName(strings.Repeat("é", 400))), 255)
}

func TestIsAllowedContentType(t *testing.T) {
	for _, ok := range []string{"image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "text/plain", "text/plain; charset=utf-8", "application/zip"} {
		assert.True(t, services.IsAllowedContentType(ok), ok)
	}

	for _, bad := range []string{"", "application/octet-stream", "application/x-msdownload", "text/html", "text/html; charset=utf-8", "image/svg+xml", "application/javascript", "lixo;;;"} {
		assert.False(t, services.IsAllowedContentType(bad), bad)
	}
}

func TestBuildStorageKey(t *testing.T) {
	card, attachment := bson.NewObjectID(), bson.NewObjectID()

	assert.Equal(t, "cards/"+card.Hex()+"/"+attachment.Hex()+"/evil.png",
		services.BuildStorageKey(card, attachment, "../../../evil.png"))
	assert.Equal(t, "cards/"+card.Hex()+"/"+attachment.Hex()+"/file",
		services.BuildStorageKey(card, attachment, ".."))
}

func TestValidateLink(t *testing.T) {
	cases := []struct {
		name    string
		linkURL string
		wantErr bool
	}{
		{"https", "https://exemplo.com/doc", false},
		{"http com porta e query", "http://localhost:8080/x?y=1", false},
		{"javascript", "javascript:alert(1)", true},
		{"data", "data:text/html;base64,AAAA", true},
		{"file", "file:///etc/passwd", true},
		{"ftp", "ftp://exemplo.com/x", true},
		{"sem esquema", "exemplo.com/x", true},
		{"sem host", "https://", true},
		{"vazia", "", true},
		{"muito longa", "https://exemplo.com/" + strings.Repeat("a", 2000), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, gotURL, err := services.ValidateLink("  Doc  ", tc.linkURL)

			if tc.wantErr {
				assert.ErrorIs(t, err, services.ErrInvalidLink)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, "Doc", name)
			assert.Equal(t, tc.linkURL, gotURL)
		})
	}

	_, _, err := services.ValidateLink("   ", "https://exemplo.com")
	assert.ErrorIs(t, err, services.ErrInvalidLink, "nome em branco")

	_, _, err = services.ValidateLink(strings.Repeat("a", 101), "https://exemplo.com")
	assert.ErrorIs(t, err, services.ErrInvalidLink, "nome com 101 caracteres")
}
