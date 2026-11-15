package database

import (
	"context"
	"kanban-management/config"
	"log"
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

	log.Println("Conectado ao MongoDB!")

	DB = client.Database(cfg.DatabaseName)

	if err := CreateIndexes(); err != nil {
		return err
	}

	return nil
}

func CreateIndexes() error {
	collection := GetCollection("users")

	_, err := collection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "email", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	)

	return err
}

func GetCollection(name string) *mongo.Collection {
	return DB.Collection(name)
}
