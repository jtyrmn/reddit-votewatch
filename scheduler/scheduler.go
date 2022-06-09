package scheduler

import (
	"fmt"
	"time"

	"github.com/jtyrmn/reddit-votewatch/util"
)

//this file handles the timing and scheduling of certain events such as refreshing the access token, culling the db, requerying reddit, etc

type redditApiHandlerScheduler interface {
	TimeToNextTokenRefresh() time.Duration
	TokenRefresh() error

	TrackNewlyCreatedPosts() int
}

type databaseConnectionScheduler interface {
}

//this function loops over all the events of both the reddit and database handler simultaneously
func Start(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {

	//ticker for reddit token refresh
	redditTicker := time.NewTicker(reddit.TimeToNextTokenRefresh())
	fmt.Println("waiting on rt")

	//ticker for fetching new posts
	newPostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("NEW_POSTS_REFRESH_PERIOD")))
	

	for {
		select {
		case <-redditTicker.C:
			fmt.Println("refreshing...")
			reddit.TokenRefresh()
			redditTicker.Reset(reddit.TimeToNextTokenRefresh())

		case <-newPostsTicker.C:
			fmt.Println("fetching new posts...")
			count := reddit.TrackNewlyCreatedPosts()
			fmt.Printf("%d new posts tracked\n", count)
		}
	}
}
