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

	fmt.Println("\ngetting data...")
	data, err := client.GetNewestPosts("dwarffortress", 10)
	if err != nil {
		panic(err)
	}
	for _, p := range data {
		fmt.Println(p.Upvotes, p.Title)
	}
	fmt.Println(len(data), "posts listed")

	fmt.Println("\nre-requesting data...")
	IDs := make([]string, len(data))
	for i := range IDs {
		IDs[i] = data[i].FullId()
	}

	data2, _ := client.FetchPosts(IDs)
	for _, p := range data2 {
		fmt.Println(p.Upvotes, p.Title)
	}
}
