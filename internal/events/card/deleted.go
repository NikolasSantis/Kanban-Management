package card

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CardDeletedEvent é disparado ao deletar um card. Os listeners de comentários
// e anexos entram nas etapas 10 e 11.
type CardDeletedEvent struct {
	CardID bson.ObjectID
	Ctx    context.Context
}

func (e CardDeletedEvent) Name() string {
	return "card.deleted"
}
