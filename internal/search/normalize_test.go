package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Implementação":    "implementacao",
		"  Olá   Mundo ":   "ola mundo",
		"AUTENTICAÇÃO JWT": "autenticacao jwt",
		"Ação & Coração":   "acao & coracao",
		"Crème Brûlée":     "creme brulee",
		"":                 "",
		"   ":              "",
		"já normalizado":   "ja normalizado",
		"tab\tnova\nlinha": "tab nova linha",
		"ÇÃO":              "cao",
		"x\u0301":          "x",
	}

	for in, want := range cases {
		assert.Equal(t, want, Normalize(in), "entrada %q", in)
	}
}

func TestNormalize_IsIdempotent(t *testing.T) {
	once := Normalize("  Implementação   de Autenticação ")

	assert.Equal(t, once, Normalize(once))
}

func TestParseQuery(t *testing.T) {
	t.Run("normaliza e divide em palavras", func(t *testing.T) {
		terms, err := ParseQuery("  Login  IMPLEMENTAÇÃO ")

		assert.NoError(t, err)
		assert.Equal(t, []string{"login", "implementacao"}, terms)
	})

	t.Run("remove palavras repetidas", func(t *testing.T) {
		terms, err := ParseQuery("Login login LOGIN")

		assert.NoError(t, err)
		assert.Equal(t, []string{"login"}, terms)
	})

	t.Run("7 palavras viram 5", func(t *testing.T) {
		terms, err := ParseQuery("um dois tres quatro cinco seis sete")

		assert.NoError(t, err)
		assert.Equal(t, []string{"um", "dois", "tres", "quatro", "cinco"}, terms)
	})

	t.Run("regex vira texto literal (só normalizado)", func(t *testing.T) {
		terms, err := ParseQuery(".*")

		assert.NoError(t, err)
		assert.Equal(t, []string{".*"}, terms)
	})

	cases := []struct {
		name string
		in   string
	}{
		{"vazio", ""},
		{"só espaços", "     "},
		{"1 caractere", "a"},
		{"1 caractere com espaços", "  a  "},
		{"101 caracteres", strings.Repeat("a", 101)},
		{"só acentos soltos", "\u0301\u0301"},
	}

	for _, tc := range cases {
		t.Run("erro: "+tc.name, func(t *testing.T) {
			_, err := ParseQuery(tc.in)
			assert.ErrorIs(t, err, ErrInvalidQuery)
		})
	}

	t.Run("100 caracteres é permitido; 2 também", func(t *testing.T) {
		_, err := ParseQuery(strings.Repeat("a", 100))
		assert.NoError(t, err)

		_, err = ParseQuery("ab")
		assert.NoError(t, err)
	})

	t.Run("conta caracteres, não bytes", func(t *testing.T) {
		_, err := ParseQuery(strings.Repeat("é", 100))
		assert.NoError(t, err)
	})
}

func TestRegexFilter(t *testing.T) {
	filter := RegexFilter("search_text", []string{"login", "a+b"})

	assert.Equal(t, bson.M{"$and": []bson.M{
		{"search_text": bson.M{"$regex": "login"}},
		{"search_text": bson.M{"$regex": `a\+b`}},
	}}, filter)
}

func TestRegexFilter_EscapesRegexMetacharacters(t *testing.T) {
	filter := RegexFilter("f", []string{"(a+)+$"})

	conditions := filter["$and"].([]bson.M)
	assert.Equal(t, `\(a\+\)\+\$`, conditions[0]["f"].(bson.M)["$regex"], "nada de regex arbitrária vinda do usuário")
}

func TestBuildText(t *testing.T) {
	assert.Equal(t, "autenticacao jwt descricao", BuildText("Autenticação JWT", "Descrição"))
	assert.Equal(t, "so titulo", BuildText("Só Título", ""))
}
