package scheduler

import (
	"fmt"
	"time"
)

//this file handles the timing and scheduling of certain events such as refreshing the access token, culling the db, requerying reddit, etc

type redditApiHandlerScheduler interface {
	TimeToNextRefresh() time.Duration
	Refresh() error
}

type databaseConnectionScheduler interface {
}

//this function loops over all the events of both the reddit and database handler simultaneously
func Start(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {

	//start ticker for reddit
	redditTicker := time.NewTicker(reddit.TimeToNextRefresh())
	fmt.Println("waiting on rt")

	for {
		select {
		case <-redditTicker.C:
			fmt.Println("refreshing...")
			reddit.Refresh()
			redditTicker.Reset(reddit.TimeToNextRefresh())
		}
	}
}
