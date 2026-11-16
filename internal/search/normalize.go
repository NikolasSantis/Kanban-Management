// Package search reúne a normalização de texto e os filtros da busca por
// texto (regex em um campo normalizado, sem acentos e em minúsculas).
package search

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	MinQueryLength = 2
	MaxQueryLength = 100
	MaxTerms       = 5
)

// ErrInvalidQuery: termo de busca com menos de 2 ou mais de 100 caracteres.
var ErrInvalidQuery = errors.New("invalid search query")

// InvalidQueryMessage é a mensagem de 400 devolvida pela API.
const InvalidQueryMessage = "Search term must have between 2 and 100 characters"

// Normalize devolve o texto em minúsculas, sem acentos e com espaços colapsados
// ("  Olá   Mundo " → "ola mundo").
func Normalize(s string) string {
	// Um transformer novo por chamada: transform.Chain guarda estado.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	normalized, _, err := transform.String(t, strings.ToLower(s))

	if err != nil {
		normalized = strings.ToLower(s)
	}

	return strings.Join(strings.Fields(normalized), " ")
}

// ParseQuery valida o termo (2–100 caracteres após o trim), normaliza, divide
// em palavras, remove as repetidas e fica com no máximo 5.
func ParseQuery(q string) ([]string, error) {
	q = strings.TrimSpace(q)

	if length := utf8.RuneCountInString(q); length < MinQueryLength || length > MaxQueryLength {
		return nil, ErrInvalidQuery
	}

	seen := map[string]bool{}
	terms := []string{}

	for _, word := range strings.Fields(Normalize(q)) {
		if seen[word] {
			continue
		}

		seen[word] = true
		terms = append(terms, word)

		if len(terms) == MaxTerms {
			break
		}
	}

	if len(terms) == 0 {
		return nil, ErrInvalidQuery
	}

	return terms, nil
}

// RegexFilter exige que TODAS as palavras apareçam no campo, em qualquer ordem.
// Cada palavra passa por regexp.QuoteMeta: o usuário nunca injeta regex
// (nem regex catastrófica), é sempre texto literal.
func RegexFilter(field string, terms []string) bson.M {
	conditions := make([]bson.M, 0, len(terms))

	for _, term := range terms {
		conditions = append(conditions, bson.M{field: bson.M{"$regex": regexp.QuoteMeta(term)}})
	}

	return bson.M{"$and": conditions}
}

// BuildText junta as partes (título, descrição...) no formato do campo search_text.
func BuildText(parts ...string) string {
	return Normalize(strings.Join(parts, " "))
}
