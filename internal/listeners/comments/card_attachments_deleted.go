package comments

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	cardEvent "kanban-management/internal/events/card"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleCardDeletedAttachments faz o soft delete dos anexos ativos do card.
// Os arquivos físicos NÃO são apagados do storage aqui (ficam órfãos de
// propósito: o soft delete é reversível e o storage pode ser limpo depois).
func HandleCardDeletedAttachments(event events.Event) error {
	e := event.(cardEvent.CardDeletedEvent)

	_, err := database.GetCollection("attachments").UpdateMany(e.Ctx, bson.M{
		"card_id": e.CardID,
		"$or":     database.NotDeleted(),
	}, bson.M{
		"$set": bson.M{"deleted_at": time.Now()},
	})

	return err
}
