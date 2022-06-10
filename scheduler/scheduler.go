package scheduler

import (
	"fmt"
	"time"

	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/util"
)

//this file handles the timing and scheduling of certain events such as refreshing the access token, culling the db, requerying reddit, etc

type redditApiHandlerScheduler interface {
	TimeToNextTokenRefresh() time.Duration
	TokenRefresh() error

	TrackNewlyCreatedPosts() int
	GetTrackedPosts() reddit.ContentGroup

	GetTrackedIDs() []reddit.Fullname
	FetchPosts([]reddit.Fullname) (*reddit.ContentGroup, error)
}

type databaseConnectionScheduler interface {
	RecordNewData(newData reddit.ContentGroup) error

	SaveListings(listings reddit.ContentGroup) error
}

//this function starts a forever loops that goes over all the events of both the reddit and database handler simultaneously
func Start(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {

	//ticker for reddit token refresh
	redditTicker := time.NewTicker(reddit.TimeToNextTokenRefresh())
	fmt.Println("waiting on rt")

	//ticker for fetching new posts
	newPostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("NEW_POSTS_REFRESH_PERIOD")))

	//ticker for downloading fetching new posts and downloading them to db
	updatePostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("UPDATE_TRACKED_POSTS_REFRESH_PERIOD")))

	for {
		select {
		case <-redditTicker.C:
			fmt.Println("refreshing access token...")
			err := reddit.TokenRefresh()
			if err != nil {
				fmt.Println("error refreshing access token:\n" + err.Error())
			}
			redditTicker.Reset(reddit.TimeToNextTokenRefresh())

		case <-newPostsTicker.C:
			fmt.Println("fetching new posts...")
			count := reddit.TrackNewlyCreatedPosts()
			fmt.Printf("%d new posts tracked\n", count)
			fmt.Printf("%d total posts tracked\n", len(reddit.GetTrackedIDs()))

			fmt.Println("saving posts...")
			err := database.SaveListings(reddit.GetTrackedPosts())
			if err != nil {
				fmt.Println("error saving posts:\n" + err.Error())
			}

		case <-updatePostsTicker.C:
			fmt.Println("updating posts...")
			err := updateTrackedPosts(reddit, database)
			if err != nil {
				fmt.Println("error updating:\n" + err.Error())
			}
		}
	}
}

func updateTrackedPosts(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) error {
	// IDs := reddit.GetTrackedIDs()

	// posts, err := reddit.FetchPosts(IDs)
	// if err != nil {
	// 	return errors.New("error fetching posts from reddit:\n" + err.Error())
	// }

	// err = database.RecordNewData(*posts)
	// if err != nil {
	// 	return errors.New("error recording data in database:\n" + err.Error())
	// }

	return nil
}
