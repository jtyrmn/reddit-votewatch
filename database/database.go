package database

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/jtyrmn/reddit-votewatch/conv"
	"github.com/jtyrmn/reddit-votewatch/pb"
	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/util"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

/*
This module used to contain mongodb interfacing code, but now serves as a
grpc client to the subreddit-logger-database service. All mongodb code was
moved over there.
*/
type connection struct {
	connection grpc.ClientConn
	client     pb.ListingsDatabaseClient
}

//note: a listing is just a piece of media from reddit. A comment or a post or a link, etc

// this template struct describes how each listing is represented in the db
type document struct {
	Id      reddit.Fullname      `bson:"_id"`
	Listing reddit.RedditContent `bson:"listing"`
}

// call this function to establish a new connection with subreddit-logger-db
func Connect() (*connection, error) {
	conn, err := grpc.Dial(util.GetEnv("SUBREDDIT_LOGGER_DATABASE_LOCATION"),  grpc.WithTransportCredentials(insecure.NewCredentials()))
	// TODO: figure out credentials
	if err != nil {
		return nil, fmt.Errorf("error establishing connection:\n%s", err)
	}

	client := pb.NewListingsDatabaseClient(conn)

	return &connection{connection: *conn, client: client}, nil
}

/*
the connection will be active the entire program, but try to close it when
the program terminates
*/
func (c connection) Close() {
	c.connection.Close()
}

// saves the listings to the database. Note that Fullname IDs in ContentGroup are treated as unique keys so duplicates will not be inserted
// as a result, you should use this function to save listings that were recently created on reddit (probably not in the database yet)
func (c connection) SaveListings(listings reddit.ContentGroup) error {
	// SaveListings requires a listings-count header
	md := metadata.New(map[string]string{"listings-count": strconv.Itoa(len(listings))})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// start streaming
	stream, err := c.client.SaveListings(ctx)
	if err != nil {
		return fmt.Errorf("error creating stream:\n%s", err)
	}

	for ID, listing := range listings {
		toSend := conv.ToGrpc(listing)
		err = stream.Send(&toSend)
		if err != nil {
			return fmt.Errorf("error streaming listing of ID \"%s\":\n%s", ID, err)
		}
	}

	// recieve response
	_, err = stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("error from server response:\n%s", err)
	}

	return nil
}

// pulls *all* the listings from the database and places it into the set parameter.
// doesn't replace pre-existing duplicate, probably more up-to-date, listings in set however
// maxAge: only recieve posts that are at most maxAge seconds old
// returns # of listings inserted into set
func (c connection) RecieveListings(set reddit.ContentGroup, maxAge int64) (int, error) {
	request := pb.RetrieveListingsRequest{MaxAge: uint64(maxAge)}
	stream, err := c.client.RetrieveListings(context.Background(), &request)
	if err != nil {
		return 0, fmt.Errorf("error calling database service:\n%s", err)
	}

	recievedCount := 0
	// recieve listings from stream and put them into set
	for {
		recieved, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading from stream:\n%s", err)
		}

		listing := conv.ToRedditContent(*recieved)
		set[listing.FullId()] = listing
		recievedCount += 1
	}

	return recievedCount, nil
}

// Records all the listings in newData as entries in the database under their respective listings
func (c connection) RecordNewData(newData reddit.ContentGroup) error {
	// UpdateListings requires a listings-count header
	md := metadata.New(map[string]string{"listings-count": strconv.Itoa(len(newData))})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// start streaming
	stream, err := c.client.UpdateListings(ctx)
	if err != nil {
		return fmt.Errorf("error creating stream:\n%s", err)
	}

	for ID, listing := range newData {
		toSend := conv.ToGrpc(listing)
		err = stream.Send(&toSend)
		if err != nil {
			return fmt.Errorf("error streaming listing of ID \"%s\":\n%s", ID, err)
		}
	}

	// recieve response
	_, err = stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("error from server response:\n%s", err)
	}

	return nil
}

func isDuplicateKeyError(err error) bool {
	conv, ok := err.(mongo.BulkWriteException)
	if !ok {
		return false
	}

	for _, writeError := range conv.WriteErrors {
		if writeError.Code == 11000 { //mongodb error code for duplicate key
			return true
		}
	}

	return false
}

// all posts in the database that are past maxAge seconds old get deleted
// returns # of listings deleted
func (c connection) CullListings(maxAge uint64) (int, error) {
	request := pb.CullListingsRequest{MaxAge: maxAge}
	response, err := c.client.CullListings(context.Background(), &request)
	if err != nil {
		return 0, fmt.Errorf("error calling database service:\n%s", err)
	}

	return int(response.NumDeleted), nil
}
