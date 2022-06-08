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

	r, err := reddit.Connect()
	if err != nil {
		log.Fatal("error connecting to reddit:\n" + err.Error())
	}
	fmt.Println(r)

	// database, err := database.Connect()
	// if err != nil {
	// 	log.Fatal("error connecting to database:\n" + err.Error())
	// }

	// scheduler.Start(reddit, database)

	f := reddit.Fullname("t3_v7e2ci")
	d, _ := r.GetNewestPosts("unturned", 100, &f)
	for _, p := range d {
		fmt.Printf("%s: %s\n", p.FullId(), p.Title)
	}
	fmt.Println(len(d))
}