package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/jtyrmn/reddit-votewatch/reddit"
)

func main() {
	//load env variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}
	client := reddit.NewApi()
	fmt.Println(client)
}
