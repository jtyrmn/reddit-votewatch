package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jtyrmn/reddit-votewatch/util"
)

//this file handles management and containment of subreddits

type subreddit struct {
	name string   //does not include the r/.
	last Fullname //last post queried on this subreddit, see GetNewestPosts
}

//gets a list of subreddits defined in SUBREDDITS_PATH
//see subreddits.json.template
func  getSubredditsFromFile() ([]subreddit, error) {
	//get the location of it
	path := util.GetEnv("SUBREDDITS_PATH")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		//cache file does not exist
		return nil, fmt.Errorf("file not found at %s\n", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("error reading subreddits file:\n" + err.Error())
	}
	
	//SUBREDDITS_PATH file is a json object with a "subreddits" field containing an array of strings
	type jsonStruct struct {
		Subreddits []string `json:"subreddits"`
	}

	var parsing jsonStruct
	err = json.Unmarshal(data, &parsing)
	if err != nil {
		return nil, errors.New("error parsing json:\n" + err.Error())
	}

	subreddits := make([]subreddit, len(parsing.Subreddits))
	for idx, name := range parsing.Subreddits {
		subreddits[idx] = subreddit{
			name: name,
			last: "",
		}
	}

	return subreddits, nil
}
