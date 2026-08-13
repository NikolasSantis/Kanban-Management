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

func ConnectDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(config.MongoURI()))
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Conectado ao MongoDB!")

	DB = client.Database(config.GetDatabase())

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
