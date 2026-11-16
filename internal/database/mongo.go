package database

import (
	"context"
	"kanban-management/config"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var DB *mongo.Database

func ConnectDB(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	slog.Info("Conectado ao MongoDB", "database", cfg.DatabaseName)

	DB = client.Database(cfg.DatabaseName)

	if err := CreateIndexes(); err != nil {
		return err
	}

	return nil
}

func CreateIndexes() error {
	ctx := context.Background()

	_, err := GetCollection("users").Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "email", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	)

	if err != nil {
		return err
	}

	_, err = GetCollection("comments").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "card_id", Value: 1},
			{Key: "created_at", Value: 1},
		},
	})

	if err != nil {
		return err
	}

	// Nome de label único por board (sem diferenciar maiúsculas) enquanto ativa.
	// O Mongo não aceita {$exists: false} em partialFilterExpression, por isso o
	// índice usa o campo "active" (true até o soft delete).
	_, err = GetCollection("labels").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "board_id", Value: 1},
			{Key: "name_normalized", Value: 1},
		},
		Options: options.Index().
			SetUnique(true).
			SetPartialFilterExpression(bson.M{"active": true}),
	})

	if err != nil {
		return err
	}

	_, err = GetCollection("cards").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "board_id", Value: 1},
				{Key: "column_id", Value: 1},
				{Key: "position", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "board_id", Value: 1},
				{Key: "priority_level", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "due_date", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "board_id", Value: 1},
				{Key: "label_ids", Value: 1},
			},
		},
	})

	return err
}

func GetCollection(name string) *mongo.Collection {
	return DB.Collection(name)
}
