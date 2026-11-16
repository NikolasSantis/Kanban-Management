package handler

import (
	"strings"

	"kanban-management/internal/search"

	"github.com/gofiber/fiber/v2"
)

// searchTerms lê ?q=. Sem q (ou só espaços) a listagem é a normal (searching=false).
// Com q inválido (menos de 2 ou mais de 100 caracteres) devolve erro → 400.
func searchTerms(c *fiber.Ctx) (terms []string, searching bool, err error) {
	if strings.TrimSpace(c.Query("q")) == "" {
		return nil, false, nil
	}

	terms, err = search.ParseQuery(c.Query("q"))

	if err != nil {
		return nil, false, err
	}

	return terms, true, nil
}
