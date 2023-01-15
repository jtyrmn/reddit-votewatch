package conv

import (
	"github.com/jtyrmn/reddit-votewatch/pb"
	"github.com/jtyrmn/reddit-votewatch/reddit"
)

/*
	This module's purpose is to convert between native RedditContent structs and
	grpc pb.RedditContent structs
*/

func ToRedditContent(pb pb.RedditContent) reddit.RedditContent {
	rc := reddit.RedditContent{
		Id:          pb.MetaData.Id,
		ContentType: pb.MetaData.ContentType,
		Title:       pb.MetaData.Title,
		Upvotes:     int(pb.MetaData.Upvotes),
		Comments:    int(pb.MetaData.Comments),
		Date:        pb.MetaData.DateCreated,
		QueryDate:   pb.MetaData.DateQueried,
	}

	return rc
}

func ToGrpc(rc reddit.RedditContent) pb.RedditContent {
	return pb.RedditContent{
		Id: rc.ContentType + "_" + rc.Id,
		MetaData: &pb.RedditContent_MetaData{
			ContentType: rc.ContentType,
			Id: rc.Id,
			Title: rc.Title,
			Upvotes: uint32(rc.Upvotes),
			Comments: uint32(rc.Comments),
			DateCreated: rc.Date,
			DateQueried: rc.QueryDate,
		},
		Entries: make([]*pb.RedditContent_ListingEntry, 0), // reddit.RedditContents have no entries by default
		// allocating for an empty array might be expensive but leaving it null is sketchy
	}
} 