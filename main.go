package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/jtyrmn/reddit-votewatch/database"
	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/scheduler"
)

func main() {
	//load env variables
	envPath := ".env"
	if e, exists := os.LookupEnv("ENV_PATH"); exists {
		envPath = e
	}

	err := godotenv.Load(envPath)
	if err != nil {
		log.Fatal("error loading .env file: " + err.Error())
	}

	// init APIs to reddit and database
	r, err := reddit.Connect()
	if err != nil {
		log.Fatal("error connecting to reddit:\n" + err.Error())
	}

	database, err := database.Connect()
	if err != nil {
		log.Fatal("error connecting to database:\n" + err.Error())
	}

	scheduler.Start(r, database)
}
