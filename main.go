package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/jtyrmn/reddit-votewatch/database"
	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/scheduler"
	"github.com/jtyrmn/reddit-votewatch/util"
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

	database, err := database.Connect()
	if err != nil {
		log.Fatal("error connecting to database:\n" + err.Error())
	}

	scheduler.Start(r, database)

	fmt.Println(util.GetEnvInt("YEHEBOI"))

	// f := reddit.Fullname("t3_v7zzrm")
	// d, _ := r.GetNewestPosts("wallstreetbets", 20, &f)
	// for _, p := range d {
	// 	fmt.Printf("%s: %s\n", p.FullId(), p.Title)
	// }
	// fmt.Println(len(d))
}
