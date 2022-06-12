package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//this module deals with the mongodb connection where we record reddit data
type connection struct {
	connection mongo.Client

	//the listings collection is probably the only collection this file will ever touch
	listings mongo.Collection
}

//note: a listing is just a piece of media from reddit. A comment or a post or a link, etc

//this template struct describes how each listing is represented in the db
type document struct {
	Id      reddit.Fullname      `bson:"_id"`
	Listing reddit.RedditContent `bson:"listing"`
}

//call this function to establish a new connection with your mongodb db
func Connect() (*connection, error) {
	connectionString := util.GetEnv("MONGODB_CONNECTION_STRING")
	databaseName := util.GetEnv("MONGODB_DATABASE_NAME")

	conn, err := mongo.Connect(context.Background(), options.Client().ApplyURI(connectionString))
	if err != nil {
		return nil, errors.New("error connecting to database:\n" + err.Error())
	}

	//"listings" collection
	collection := conn.Database(databaseName).Collection("listings")

	return &connection{connection: *conn, listings: *collection}, nil
}

//saves the listings to the database. Note that Fullname IDs in ContentGroup are treated as unique keys so duplicates will not be inserted
//as a result, you should use this function to save listings that were recently created on reddit (probably not in the database yet)
func (c connection) SaveListings(listings reddit.ContentGroup) error {

	if len(listings) == 0 {
		return nil
	}

	//convert all the items in listings to a bson-friendly array before sending it off to the db
	documents := make([]interface{}, len(listings))
	documents_idx := 0
	for id, listing := range listings {
		documents[documents_idx] = document{Id: id, Listing: listing}
		documents_idx += 1
	}

	_, err := c.listings.InsertMany(context.Background(), documents)
	//error handling mongodb driver is complex so I will this until later. It's not like program state is going to change if an error occurs here
	//TODO
	if err != nil && !isDuplicateKeyError(err) { //ignore duplicate key errors, those are expected
		return err
	}
	return nil
}

//pulls *all* the listings from the database and places it into the set parameter.
//doesn't replace pre-existing duplicate, probably more up-to-date, listings in set however
//maxAge: only recieve posts that are at most maxAge seconds old
//returns # of listings inserted into set
func (c connection) RecieveListings(set reddit.ContentGroup, maxAge int64) (int, error) {

	maxDate := time.Now().Unix() - int64(maxAge)
	fmt.Println(maxDate)
	data, err := c.listings.Find(context.Background(), bson.D{{"listing.date", bson.D{{"$gte", maxDate}}}})
	if err != nil {
		return 0, err
	}

	countInsertions := 0
	for data.Next(context.Background()) {
		var d document
		data.Decode(&d)

		//_id is a unique, required key. I don't think this check is required unless some custom-id document was inserted from somewhere else
		if !d.Id.IsValid() {
			fmt.Printf("warning: listing with invalid ID \"%s\" found in database\n", d.Id)
			continue
		}

		//check if the listing already exists within the output we recieved
		if _, exists := set[d.Id]; exists {
			//fmt.Printf("debugging: %s already exists\n", d.Id)
			continue
		}

		set[d.Id] = d.Listing
		countInsertions += 1
	}

	return countInsertions, nil
}

//Records all the listings in newData as entries in the database under their respective listings
func (c connection) RecordNewData(newData reddit.ContentGroup) error {

	if len(newData) == 0 {
		return nil
	}

	//template for a single entry under a listing
	type record struct {
		Upvotes  int
		Comments int
		Date     uint64
	}

	//have to construct a bulk write, a unique $push update for every entry listing
	models := make([]mongo.WriteModel, len(newData))
	modelsIdx := 0
	for id, listing := range newData {
		r := record{
			Upvotes:  listing.Upvotes,
			Comments: listing.Comments,
			Date:     listing.QueryDate,
		}
		model := mongo.NewUpdateOneModel().SetFilter(bson.D{{"_id", id}}).SetUpdate(bson.D{{"$push", bson.D{{"entries", r}}}})

		models[modelsIdx] = model
		modelsIdx += 1
	}

	_, err := c.listings.BulkWrite(context.Background(), models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return errors.New("error updating entries in database:\n" + err.Error())
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

//all posts in the database that are past maxAge seconds old get deleted
//returns # of listings deleted
func (c connection) CullListings(maxAge uint64) (int, error) {
	res, err := c.listings.DeleteMany(context.Background(), bson.D{{"listing.date", bson.D{{"$lt", uint64(time.Now().Unix()) - maxAge}}}})
	if err != nil {
		return 0, errors.New("error deleting listings from database:\n" + err.Error())
	}

	return int(res.DeletedCount), nil
}
