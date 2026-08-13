package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func Load() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal(err)
	}
}

func MongoURI() string {
	Load()

	return os.Getenv("MONGODB_URI")
}

func GetDatabase() string {
	return os.Getenv("DATABASE_NAME")
}
