package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/jtyrmn/reddit-votewatch/database"
	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/scheduler"
)

func main() {
	//load env variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	reddit, err := reddit.Connect()
	if err != nil {
		log.Fatal("error connecting to reddit:\n" + err.Error())
	}
	fmt.Println(reddit)

	database, err := database.Connect()
	if err != nil {
		log.Fatal("error connecting to database:\n" + err.Error())
	}

	scheduler.Start(reddit, database)
}
