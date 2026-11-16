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
	})

	return err
}

func GetCollection(name string) *mongo.Collection {
	return DB.Collection(name)
}
