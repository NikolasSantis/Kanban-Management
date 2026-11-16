package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNotDeletedAcceptsMissingNilAndZeroDate(t *testing.T) {
	conditions := NotDeleted()

	assert.Contains(t, conditions, bson.M{"deleted_at": bson.M{"$exists": false}})
	assert.Contains(t, conditions, bson.M{"deleted_at": nil})
	assert.Contains(t, conditions, bson.M{"deleted_at": time.Time{}})
	assert.Len(t, conditions, 3)
}
