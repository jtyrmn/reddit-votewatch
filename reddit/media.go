package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

//all types of content from reddit (posts, comments, etc) are represented as the same object in the reddit API and thus are all represented as the same in this struct
//ContentType identifies the type of content. eg: t1_ = comment, t3_ = post, etc. See https://www.reddit.com/dev/api/
//note that certain fields will be 0-initialized for certain content types. Comments dont't have titles for example.
type redditContent struct {
	ContentType string `json:"kind"`
	Id          string
	Title       string
	Content     string `json:"selftext"`
	Upvotes     int    `json:"ups"`
}

//the API requires you identify content via their "fullnames", which is the content type + id. For example: t3_62sjuh
func (r redditContent) FullId() string {
	return r.ContentType + "_" + r.Id
}

//get the <num> latest posts at a specific subreddit
//it's important to note that exactly <num> posts being returned is not garanteed.

//note: currently this will only return at most 100 posts due to the reddit api restriction.
//TODO: parallelize this function to allow 100> posts. Should do this after implementing rate limiting
func (r *redditApiHandler) GetNewestPosts(subreddit string, num int) ([]redditContent, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("https://oauth.reddit.com/r/%s/new.json?limit=%d", subreddit, num), nil)
	if err != nil {
		panic(err)
	}

	PopulateStandardHeaders(&request.Header, r.accessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, _ := ioutil.ReadAll(response.Body)

	//struct to parse the valuable redditContent data from the response JSON
	var responseBodyJson struct {
		Data struct {
			Children []struct {
				//Content redditContent
				ContentType string `json:"kind"`
				Data        redditContent
			}
		}
	}
	err = json.Unmarshal(responseBody, &responseBodyJson)
	if err != nil {
		return nil, errors.New("error parsing response body:\n" + err.Error())
	}

	//if subreddit doesn't exist, reddit doesn't explicity tell us. It just has data.children be empty
	if len(responseBodyJson.Data.Children) == 0 {
		return nil, fmt.Errorf("subreddit %s either doesn't exist or has 0 posts", subreddit)
	}

	redditContentArray := make([]redditContent, len(responseBodyJson.Data.Children))
	for i, post := range responseBodyJson.Data.Children {
		redditContentArray[i] = post.Data
		redditContentArray[i].ContentType = post.ContentType
	}

	return redditContentArray, nil
}
