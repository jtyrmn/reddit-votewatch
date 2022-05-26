package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
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
		After string `json:"after"` //for making multiple calls

		Children []struct {
			ContentType string `json:"kind"`
			Data        redditContent
		}
	}
}

//get the <num> latest posts at a specific subreddit
//it's important to note that exactly <num> posts being returned is not garanteed. Their might be 100 <num> posts on the subreddit, and other cases
//note: (non-concurrent) api calls are done in groups of 100 listings. So 101 requests will block for twice as long as 100 requests 
func (r redditApiHandler) GetNewestPosts(subreddit string, num int) ([]redditContent, error) {
	if num <= 0 {
		return nil, fmt.Errorf("num %d must be positive", num)
	}


	//our nested function to call api. Used in loop below
	callApi := func(url string) (*responseParserStruct, error) {
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		populateStandardHeaders(&request.Header, r.accessToken)

		r.rateLimiter.Wait(context.Background())
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, err
		}

		//unauthorized
		if response.StatusCode != 200 {
			return nil, errors.New(response.Status + " recieved querying reddit")
		}

		responseBody, _ := ioutil.ReadAll(response.Body)

		//parsing response
		var responseBodyJson responseParserStruct
		json.Unmarshal(responseBody, &responseBodyJson)

		return &responseBodyJson, nil
	}

	/*
		a single api call to reddit will only return at most 100 listings, therefore we have to do ceil(num/100) api calls to get num listings
		unfortunetly reddit does not provide any way to make calls in parallel, as a call for past the first 100 listings requires a "after" parameter
		to indicate where the next 100 listings are. The after param is obtained from the previous response.
		https://www.reddit.com/dev/api/
		therefore we have to do the api calls in non-parallel sequence
	*/

	//reddit's max limit= param value
	const limit = 100

	//note: it's not garanteed for results to be full after this operation. Have to reduce it's size later if that's the case
	results := make([]redditContent, num)
	results_index := 0

	totalCalls := int(math.Ceil(float64(num) / limit)) //how many calls we need to make to get num listings
	listingsNeeded := num                              //keep track of how many listings we need per iteration (for limit= param)
	after := ""

	for currentCall := 0; currentCall < totalCalls; currentCall += 1 {
		currentListingsNeeded := listingsNeeded
		if currentListingsNeeded > limit {
			currentListingsNeeded = limit
		}

		url := fmt.Sprintf("https://oauth.reddit.com/r/%s/new.json?limit=%d", subreddit, currentListingsNeeded)
		if currentCall > 0 { //if this is past the first call, otherwise "after" doesn't exist yet
			url = url + "&after=" + after
		}

		response, err := callApi(url)
		if err != nil {
			return nil, fmt.Errorf("error calling reddit api on iteration %d:\n%s", currentCall+1, err.Error())
		}

		//check to see there are actual results in response
		if len(response.Data.Children) == 0 {
			fmt.Printf("warning: subreddit r/%s either doesn't exist or has no posts\n", subreddit)
			break
		}

		after = response.Data.After

		//fill the results array with this iteration's 100 or less listings
		for _, post := range response.Data.Children {
			results[results_index] = post.Data
			results[results_index].ContentType = post.ContentType
			results_index += 1
		}

		if totalCalls > 1 {
			fmt.Printf("batch request %d/%d done\n", currentCall+1, totalCalls)
		}

		listingsNeeded -= limit
	}

	return results[:results_index], nil //dont return the entire slice, just the populated part
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

	r.rateLimiter.Wait(context.Background())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	//unauthorized
	if response.StatusCode != 200 {
		return nil, errors.New(response.Status + " recieved querying reddit")
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
