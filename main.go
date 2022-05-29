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

	client := reddit.Connect()
	fmt.Println(client)

	fmt.Println("\ngetting data...")
	data, err := client.GetNewestPosts("dwarffortress", 101)
	if err != nil {
		panic(err)
	}
	// for _, p := range data {
	// 	fmt.Println(p.Title)
	// }
	fmt.Println(len(data), "posts listed")

	fmt.Println("\nre-requesting data...")
	IDs := make([]reddit.Fullname, len(data))
	for i := range IDs {
		IDs[i] = data[i].FullId()
	}

	data2, err := client.FetchPosts(IDs)
	if err != nil {
		panic(err)
	}

	for ID, post := range *data2 {
		fmt.Printf("%s: %s\n", ID, post.Title)
	}
}
