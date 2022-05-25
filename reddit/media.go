package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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

//use this struct whenever you need to parse a standard GET response from oauth.reddit.com and get the reddit media
type responseParserStruct struct {
	Data struct {
		Children []struct {
			//Content redditContent
			ContentType string `json:"kind"`
			Data        redditContent
		}
	}
}

//get the <num> latest posts at a specific subreddit
//it's important to note that exactly <num> posts being returned is not garanteed.

//note: currently this will only return at most 100 posts due to the reddit api restriction.
//TODO: parallelize this function to allow >100 posts. Should do this after implementing rate limiting
func (r redditApiHandler) GetNewestPosts(subreddit string, num int) ([]redditContent, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("https://oauth.reddit.com/r/%s/new.json?limit=%d", subreddit, num), nil)
	if err != nil {
		panic(err)
	}

	populateStandardHeaders(&request.Header, r.accessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, _ := ioutil.ReadAll(response.Body)

	//parsing response
	var responseBodyJson responseParserStruct
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

//given a list of fullname IDs (justFullID()), queries reddit for the posts corresponding to those IDS 
func (r redditApiHandler) FetchPosts(IDs []string) ([]redditContent, error) {
	//construct the url
	//see reddit api documentation on /api/info
	var url_builder strings.Builder
	for _, ID := range IDs {
		url_builder.WriteString(ID + ",")
	}
	url := "https://oauth.reddit.com/api/info/?id=" + url_builder.String()

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	populateStandardHeaders(&request.Header, r.accessToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, _ := ioutil.ReadAll(response.Body)

	//parsing response
	var responseBodyJson responseParserStruct
	json.Unmarshal(responseBody, &responseBodyJson)

	//return all the redditContent in responseBodyJson
	redditContentArray := make([]redditContent, len(responseBodyJson.Data.Children))
	
	//maybe I should instead return a map with IDs as keys and redditContents as values? perhaps
	for i, post := range responseBodyJson.Data.Children {
		redditContentArray[i] = post.Data
		redditContentArray[i].ContentType = post.ContentType
	}

	return redditContentArray, nil
}
