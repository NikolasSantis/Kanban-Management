package filters

import (
	"slices"
	"strings"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const maxStatusLength = 50

// ParseRoles lê ?role=admin,manager e valida cada valor contra as roles permitidas.
// Vazio = sem filtro (nil).
func ParseRoles(raw string, allowed []string) ([]string, error) {
	values, err := SplitCSV(raw, MaxValues)

	if err != nil {
		return nil, queryError("Too many values in role (max %d)", MaxValues)
	}

	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return nil, queryError("Invalid role. Allowed: %s", strings.Join(allowed, ", "))
		}
	}

	if len(values) == 0 {
		return nil, nil
	}

	return values, nil
}

// ParseStatuses lê ?status=active,archived (status de projeto é texto livre,
// então só limita o tamanho e a quantidade). Vazio = sem filtro (nil).
func ParseStatuses(raw string) ([]string, error) {
	values, err := SplitCSV(raw, MaxValues)

	if err != nil {
		return nil, queryError("Too many values in status (max %d)", MaxValues)
	}

	for _, value := range values {
		if utf8.RuneCountInString(value) > maxStatusLength {
			return nil, queryError("Invalid status: at most %d characters", maxStatusLength)
		}
	}

	if len(values) == 0 {
		return nil, nil
	}

	return values, nil
}

// ParseObjectID lê um id opcional (param é só o nome para a mensagem de erro).
func ParseObjectID(raw, param string) (*bson.ObjectID, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return nil, nil
	}

	id, err := bson.ObjectIDFromHex(raw)

	if err != nil {
		return nil, queryError("Invalid %s", param)
	}

	return &id, nil
}

// ParseAuthorID lê ?author_id= (um id ou "me"). Vazio = sem filtro (nil).
func ParseAuthorID(raw string, me bson.ObjectID) (*bson.ObjectID, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return nil, nil
	}

	id, ok := ParseIDOrMe(raw, me)

	if !ok {
		return nil, queryError("Invalid author_id. Use an id or me")
	}

	return &id, nil
}

// ParseKind lê ?kind=file|link. Vazio = sem filtro.
func ParseKind(raw string) (string, error) {
	switch kind := strings.ToLower(strings.TrimSpace(raw)); kind {
	case "", "file", "link":
		return kind, nil
	default:
		return "", queryError("Invalid kind. Allowed: file, link")
	}
}
