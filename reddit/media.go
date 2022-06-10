package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
)

//all types of content from reddit (posts, comments, etc) are represented as the same object in the reddit API and thus are all represented as the same in this struct
//ContentType identifies the type of content. eg: t1_ = comment, t3_ = post, etc. See https://www.reddit.com/dev/api/
//note that certain fields will be 0-initialized for certain content types. Comments dont't have titles for example.
type RedditContent struct {
	ContentType string `json:"kind"`
	Id          string
	Title       string
	//Content     string `json:"selftext"` //can probably remove this later
	Upvotes   int    `json:"ups" mapstructure:"ups"`
	Comments  int    `json:"num_comments" mapstructure:"num_comments"`
	Date      uint64 `json:"created_utc" mapstructure:"created_utc"` //time of creation
	QueryDate uint64 //time of recieval from the API
}

func (r *RedditContent) UnmarshalJSON(data []byte) error {

	var obj map[string]any
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return err
	}

	if f, exists := obj["created_utc"]; exists {
		obj["created_utc"] = uint64(f.(float64)) //make this floating point field an int
	}

	mapstructure.Decode(obj, r)

	return nil
}

//fullname of a reddit listing. Calculated using FullId()
//probably shouldn't be exported. It only is for debugging reasons
type Fullname string

//ensure the fullname is of t-_------ form
func (s Fullname) IsValid() bool {
	result, _ := regexp.MatchString("^t[1-6]_[a-z0-9]{6}$", string(s))
	return result
}

//a common return type/parameter for many functions in this program
type ContentGroup map[Fullname]RedditContent

//the API requires you identify content via their "fullnames", which is the content type + id. For example: t3_62sjuh
func (r RedditContent) FullId() Fullname {
	return Fullname(r.ContentType + "_" + r.Id)
}

//use this struct whenever you need to parse a standard GET response from oauth.reddit.com and get the reddit media
type responseParserStruct struct {
	Data struct {
		After string `json:"after"` //for making multiple calls

		Children []struct {
			ContentType string `json:"kind"`
			Data        RedditContent
		}
	}
}

//converts the tracked reddit posts ContentGroup to a slice of IDs
func(r redditApiHandler) GetTrackedIDs() []Fullname {
	list := make([]Fullname, len(r.trackedListings))
	
	idx := 0
	for ID := range r.trackedListings {
		list[idx] = ID
		idx += 1
	}

	return list
}

func(r redditApiHandler) GetTrackedPosts() ContentGroup {
	return r.trackedListings
}

//get the <num> latest posts at a specific subreddit
//it's important to note that exactly <num> posts being returned is not garanteed. Their might be 100 <num> posts on the subreddit, and other cases
//note: (non-concurrent) api calls are done in groups of 100 listings. So 101 requests will block for twice as long as 100 requests
//while process recieved posts up to last (unless last is nil)
func (r redditApiHandler) getNewestPosts(subreddit string, num int, last *Fullname) ([]RedditContent, error) {
	if num <= 0 {
		return nil, fmt.Errorf("num %d must be positive", num)
	}

	//our nested function to call api. Used in loop below
	callApi := func(url string) (*responseParserStruct, uint64, error) {
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, 0, err
		}

		populateStandardHeaders(&request.Header, r.accessToken)

		r.rateLimiter.Wait(context.Background())
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, 0, err
		}

		//unauthorized
		if response.StatusCode != 200 {
			return nil, 0, errors.New(response.Status + " recieved querying reddit")
		}

		//getting the time this response was sent
		timeSent, err := getTimeOfSending(response)
		if err != nil {
			return nil, 0, errors.New("error querying date of response:\n" + err.Error())
		}

		responseBody, _ := ioutil.ReadAll(response.Body)

		//parsing response
		var responseBodyJson responseParserStruct
		err = json.Unmarshal(responseBody, &responseBodyJson)
		if err != nil {
			return nil, 0, errors.New("error parsing JSON response:\n" + err.Error())
		}

		return &responseBodyJson, timeSent, nil
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
	results := make([]RedditContent, num)
	results_index := 0

	totalCalls := int(math.Ceil(float64(num) / limit)) //how many calls we need to make to get num listings
	listingsNeeded := num                              //keep track of how many listings we need per iteration (for limit= param)
	after := ""

	//whether we should keep checking if each listing in the last 
	checkLast := last != nil
	reachedLast := false

	for currentCall := 0; currentCall < totalCalls && !reachedLast; currentCall += 1 {
		currentListingsNeeded := listingsNeeded
		if currentListingsNeeded > limit {
			currentListingsNeeded = limit
		}

		url := fmt.Sprintf("https://oauth.reddit.com/r/%s/new.json?limit=%d", subreddit, currentListingsNeeded)
		if currentCall > 0 { //if this is past the first call, otherwise "after" doesn't exist yet
			url = url + "&after=" + after
		}

		response, timeSent, err := callApi(url)
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
			post.Data.ContentType = post.ContentType
			post.Data.QueryDate = timeSent

			if checkLast && post.Data.FullId() == *last {
				//stop processing any more listings
				reachedLast = true
				break
			}

			results[results_index] = post.Data
			results_index += 1
		}

		if totalCalls > 1 {
			if reachedLast {
				fmt.Printf("batch request %d/%d reached last and done\n", currentCall+1, totalCalls)
			} else {
				fmt.Printf("batch request %d/%d done\n", currentCall+1, totalCalls)
			}
		}

		listingsNeeded -= limit
	}

	return results[:results_index], nil //dont return the entire slice, just the populated part
}

//given a list of fullname IDs (justFullID()), queries reddit for the posts corresponding to those IDS
//returns a mapping of listings, indexed by their own fullname IDs
func (r redditApiHandler) FetchPosts(IDs []Fullname) (*ContentGroup, error) {
	const limit = 100
	/*
		the /api/info endpoint allows at most 100 listings to be fetched in a single call, or behaviour will be undefined
		therefore I will make multiple api calls of 100 (or less) listings each.
	*/

	numListings := len(IDs)
	totalCalls := int(math.Ceil(float64(numListings) / limit))

	//wrapper for returned items of fetchBatch func
	type fetchBatchReturn struct {
		content  []RedditContent
		timeSent uint64
	}

	//the concurrent function to request a batch of IDs
	//given a set of IDs, request their corresponding content from reddit and pipe them into out channel
	fetchBatch := func(in []Fullname, out chan<- fetchBatchReturn, errChan chan<- error) {
		//construct the url
		//see reddit api documentation on /api/info
		var url_builder strings.Builder
		for _, ID := range in {
			url_builder.WriteString(string(ID) + ",")
		}
		url := "https://oauth.reddit.com/api/info/?id=" + url_builder.String()
		//fmt.Println(url)

		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			errChan <- err
			return
		}

		populateStandardHeaders(&request.Header, r.accessToken)

		r.rateLimiter.Wait(context.Background())
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			errChan <- err
			return
		}

		//unauthorized
		if response.StatusCode != 200 {
			errChan <- errors.New(response.Status + " recieved querying reddit")
			return
		}

		//getting the time this response was sent
		timeSent, err := getTimeOfSending(response)
		if err != nil {
			errChan <- errors.New("error querying date of response:\n" + err.Error())
			return
		}

		responseBody, _ := ioutil.ReadAll(response.Body)

		//parsing response
		var responseBodyJson responseParserStruct
		json.Unmarshal(responseBody, &responseBodyJson)

		//return all the redditContent in responseBodyJson
		redditContentArray := make([]RedditContent, len(responseBodyJson.Data.Children))

		for i, post := range responseBodyJson.Data.Children {
			redditContentArray[i] = post.Data
			redditContentArray[i].ContentType = post.ContentType
		}

		out <- fetchBatchReturn{
			content:  redditContentArray,
			timeSent: timeSent,
		}

	}

	//create range of IDs for each call
	batchIDs := make([][]Fullname, totalCalls)
	currentIndex := 0
	for currentCall := 0; currentCall < totalCalls; currentCall += 1 {
		//if this is the last batch, the number of remaining IDs is in range (0, 100], not strictly 100
		if currentIndex+limit >= numListings {
			batchIDs[currentCall] = IDs[currentIndex:]
		} else {
			batchIDs[currentCall] = IDs[currentIndex : currentIndex+limit]
		}
		currentIndex += limit
	}

	//send out the batch requests
	out := make(chan fetchBatchReturn)
	errChan := make(chan error)

	r.rateLimiter.WaitN(context.Background(), totalCalls)
	for currentCall := 0; currentCall < totalCalls; currentCall += 1 {
		go fetchBatch(batchIDs[currentCall], out, errChan)
	}

	//recieve content from goroutines
	contentMap := make(ContentGroup)
	for i := 0; i < totalCalls; i += 1 {
		select {
		case result := <-out: //a response was successfully recieved and processed
			for _, content := range result.content {
				content.QueryDate = result.timeSent
				contentMap[content.FullId()] = content
			}
		case err := <-errChan: //not successful
			//apparently im supposed to use an errgroup instead of an error channel for this? idk
			fmt.Printf("error during batch request %d:\n%s\n", i+1, err.Error())
		}
		fmt.Printf("batch request %d/%d done\n", i+1, totalCalls)
	}

	//check over all our IDs to make sure they were inserted
	for _, ID := range IDs {
		if _, exists := contentMap[ID]; !exists {
			fmt.Printf("warning: ID %s returned nothing from reddit\n", ID)
		}
	}

	return &contentMap, nil
}

//this function is called on a routine to fetch all the newly created posts from the subreddit list and add them to the tracked posts
func (r *redditApiHandler) TrackNewlyCreatedPosts() int {
	TEMP := 10

	//just holds the output of task func
	type taskResult struct {
		result []RedditContent
		trackPosts bool
		err error
	}

	//do a new goroutine for each subreddit
	task := func(sub *subreddit, out chan<- taskResult) {
		var last *Fullname = nil
		if sub.last != "" {
			last = &sub.last
		}

		//whether or not we should actually save any posts this iteration for this subreddit. We only want to save posts if last is set, or else the posts we recieved were untracked for some time before recieving them
		trackPosts := last != nil

		result, err := r.getNewestPosts(sub.name, TEMP, last)
		if err != nil {
			out <- taskResult{nil, false, fmt.Errorf("error getting posts from %s:\n", err.Error())}
			return
		}

		//the newest post recieved is now the last post seen in this subreddit  
		if len(result) > 0 {
			sub.last = result[0].FullId()
		}

		out <- taskResult{result, trackPosts, nil}
	}

	out := make(chan taskResult)
	for idx := range r.subreddits {
		go task(&r.subreddits[idx], out)
	}

	postsTracked := 0 //keep count

	//recieve the channels and add the new posts to the tracker
	for i := 0; i < len(r.subreddits); i += 1 {
		results := <-out
		if results.err != nil {
			fmt.Println("warning: " + results.err.Error())
		}

		if !results.trackPosts {
			//we don't want to track this subreddit's posts this iteration
			continue
		}

		for _, post := range results.result {
			r.trackedListings[post.FullId()] = post
			postsTracked += 1
		}
	}
	
	return postsTracked
}