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

	fmt.Println("getting data...")
	data, err := client.GetNewestPosts("dwarffortress", 10)
	if err != nil {
		panic(err)
	}
	for _, p := range data {
		fmt.Println(p)
	}
	fmt.Println(len(data), "posts listed")
}
