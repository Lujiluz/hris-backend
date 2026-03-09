package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func LoadConfig() {
	err := godotenv.Load()
	if err != nil && os.Getenv("APP_ENV") != "production" {
		log.Println("Warning: env file not found. Using system environment")
	}
}
