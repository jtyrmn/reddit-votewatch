//this file handles all the api access functionality to reddit.com including fetching and caching the access token

package reddit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jtyrmn/reddit-votewatch/util"

	"golang.org/x/time/rate"
)

//container to hold a standard access token recieved from https://www.reddit.com/api/v1/access_token
type accessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpireLength int64  `json:"expires_in"`
	Scope        string `json:"scope"`

	//when the access token was recieved from reddit.com. Formatted as unix time (time.Now().Unix()).
	//not that this information is not included in the raw accessTokenResponse from reddit.com, so don't forget to manually set this after unmarshaling.
	InitializationTime int64 `json:"initialization_time"`
}

//**** IMPORTANT: never call cache() or pullFromCache() below if env var CACHE_ACCESS_TOKEN is not true, because ACCESS_TOKEN_PATH will probably not be set and the program will halt

//save the access token and its metadata to filesystem. Returns nil if successful
func (a *accessTokenResponse) cache() error {
	json, _ := json.Marshal(a) //encoding a static struct should never return an error I assume
	err := os.WriteFile(util.GetEnv("ACCESS_TOKEN_PATH"), json, 0666)
	if err != nil {
		return errors.New("error caching access token: " + err.Error())
	}
	return nil
}

//attempt to recieve access token from cache. if cache wasn't found and there wasn't any other error, this function will return (nil, nil)
func (a accessTokenResponse) pullFromCache() (*accessTokenResponse, error) {
	path := util.GetEnv("ACCESS_TOKEN_PATH")

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		//cache file does not exist
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("error reading cache:\n" + err.Error())
	}

	var token accessTokenResponse
	err = json.Unmarshal(data, &token)
	if err != nil {
		return nil, errors.New("error parsing access token from cache:\n" + err.Error())
	}

	return &token, nil
}

func (a accessTokenResponse) String() string {
	return fmt.Sprintf("{<REDACTED> %s %d %s %d}", a.TokenType, a.ExpireLength, a.Scope, a.InitializationTime)
}

//the api handler object
//should be created using NewApi()
type redditApiHandler struct {
	accessToken      accessTokenResponse
	cacheAccessToken bool //whether or not the access token should be cached/decached

	//client info you should've gotten from https://www.reddit.com/prefs/apps
	clientId     string
	clientSecret string

	//reddit account of your bot
	redditUsername string
	redditPassword string

	//rate limiting
	rateLimiter rate.Limiter
}

//dont want to print out private secrets + passwords while debugging
func (r redditApiHandler) String() string {
	return fmt.Sprintf("{%s %v %s <REDACTED> %s <REDACTED>}", r.accessToken, r.cacheAccessToken, r.clientId, r.redditUsername)
}

//NewApi() creates a reddit api client and also initializes
//OAuth2 authentication. Unless data is pulled from cache, this function will call the reddit api

//make sure you have all the env variables assigned before calling this
func NewApi() redditApiHandler {
	client := redditApiHandler{
		clientId:         util.GetEnv("REDDIT_CLIENT_ID"),
		clientSecret:     util.GetEnv("REDDIT_CLIENT_SECRET"),
		redditUsername:   util.GetEnv("REDDIT_USERNAME"),
		redditPassword:   util.GetEnv("REDDIT_PASSWORD"),
		cacheAccessToken: strings.ToLower(util.GetEnvDefault("CACHE_ACCESS_TOKEN", "true")) == "true", //theres probably a better way to do this

		/*
			The reddit API limits oauth2 clients to 60 requests per minute https://github.com/reddit-archive/reddit/wiki/API#rules
			Observing the x-limit-remaining, x-limit-reset headers from oauth.reddit.com responses makes me thing the rate limit is actually around 600 requests per 10 minutes
			which is the same frequecy but allows for greater bursts. I assume the 60 requests per minute means they don't want to deal with 600-request bursts
		*/
		rateLimiter: *rate.NewLimiter(rate.Every(time.Minute), 60),
	}

	//recieve access token, either by cache or request to api
	lookupAccessTokenCache := client.cacheAccessToken
	if lookupAccessTokenCache { //look in cache
		token, err := client.accessToken.pullFromCache()
		if token == nil {
			if err != nil { //if there was error
				fmt.Printf("error pulling access token from cache:\n%s\n", err.Error())
			} else { //pullFromCache() returning (nil, nil) means the cache doesn't exist/isn't created yet
				fmt.Printf("cache not found at %s\n", util.GetEnvDefault("ACCESS_TOKEN_PATH", "<ACCESS_TOKEN_PATH>"))
			}

			lookupAccessTokenCache = false //if we couldn't find the access token, must query api for it
		} else {

			//make sure token isn't expired
			if time.Now().Unix()-token.InitializationTime > token.ExpireLength {
				fmt.Println("access token from cache is expired")
				lookupAccessTokenCache = false
			} else {
				fmt.Println("found access token in cache")
				client.accessToken = *token
			}
		}
	}
	if !lookupAccessTokenCache { //query reddit api
		fmt.Println("querying reddit for access token...")
		token, err := fetchAccessToken(client)

		if err != nil {
			//cannot obtain an access token at all. Stop the program
			log.Fatal("error querying reddit api for access token:\n" + err.Error())
		}

		fmt.Println("recieved access token")
		client.accessToken = *token

		//assuming we got here, the access token was successfully recieved. Make sure to cache it
		if client.cacheAccessToken {
			err := client.accessToken.cache()
			if err != nil {
				fmt.Println("warning: unable to cache access token:\n" + err.Error())
			} else {
				fmt.Println("cached access token")
			}
		}
	}

	//start the access token refresh scheduler
	go client.startTokenRefreshCycle()

	return client
}

//call reddit and request an access token
func fetchAccessToken(client redditApiHandler) (*accessTokenResponse, error) {
	requestBody := fmt.Sprintf("grant_type=password&username=%s&password=%s", client.redditUsername, client.redditPassword)
	request, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		return nil, errors.New("should this error ever occur? " + err.Error())
	}

	//headers
	authorization := "basic " + base64.StdEncoding.EncodeToString([]byte(client.clientId+":"+client.clientSecret))
	request.Header = http.Header{
		"user-agent":    []string{util.GetEnv("REDDIT_USERAGENT_STRING")},
		"authorization": []string{authorization},
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.New("error querying for access token:\n" + err.Error())
	}
	//if reddit api rejects our request (unauthorizeed)
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("unauthorized client credentials\nperhaps you should check your client id and secret?")
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err) //panicking because I don't know of any scenario where err isn't nil
	}
	//in some cases reddit sends back an error response with a 200 OK. I don't know why
	//need to check if the response contains an "error" field
	var responseError struct {
		E string `json:"error"`
	}
	json.Unmarshal(responseData, &responseError)
	if responseError.E != "" {
		return nil, errors.New("response error from requesting access token:\n" + responseError.E + "\nperhaps your reddit account login info is incorrect?")
	}

	var responseJSON accessTokenResponse
	err = json.Unmarshal(responseData, &responseJSON)
	if err != nil {
		return nil, errors.New("error parsing access token response body:\n" + err.Error())
	}

	//doesn't matter much that we're using the current time and not the http response's Date header. Otherwise we would have to deal with timezones + parsing the header
	responseJSON.InitializationTime = time.Now().Unix()
	return &responseJSON, nil
}

//once this function is called, it will repeatedly schedule times at which it will refresh the access token
//should be called as a go routine
func (r *redditApiHandler) startTokenRefreshCycle() {
	//calculate the interval amount in seconds
	//more info on TOKEN_REFRESH_LENIENCY in .env.template
	leniency, err := strconv.ParseFloat(util.GetEnvDefault("TOKEN_REFRESH_LENIENCY", "0.99"), 32)
	if err != nil {
		fmt.Println("warning: env variable TOKEN_REFRESH_LENIENCY unreadable. Defaulting to 0.99...")
		leniency = 0.99
	}

	//dont accidently ddos reddit
	minimumLeniency := 0.0001
	if leniency < minimumLeniency {
		fmt.Printf("warning: leniency is dangerously low. Increasing to %f\n", minimumLeniency)
		leniency = minimumLeniency
	}

	//leniency is big; token will expire before it refreshes
	if leniency >= 1.00 {
		fmt.Printf("warning: leniency %f is very high. This will likely result in errors later\n", leniency)
	}

	//how long before the expiration date until it's time to refresh
	delay_sub := float64(r.accessToken.ExpireLength) * (1.0 - leniency)

	regular_delay := float64(r.accessToken.ExpireLength) - delay_sub

	for {
		tokenRefreshCycleIteration(r, regular_delay)
	}
}

func tokenRefreshCycleIteration(r *redditApiHandler, regular_delay float64) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("error during token refresh cycle:\n%s\n", r)
		}
	}()

	//wait until token is about to expire
	//either the regular delay of every loop or incase the token was taken from a cache and is older than expected. Whatever is smaller
	delay := math.Min(regular_delay, float64(r.accessToken.InitializationTime+r.accessToken.ExpireLength-time.Now().Unix()))
	time.Sleep(time.Second * time.Duration(delay))

	//refresh token
	fmt.Println("refreshing token...")
	token, err := fetchAccessToken(*r)
	if err != nil {
		panic(err)
	}

	r.accessToken = *token
	if r.cacheAccessToken {
		err = r.accessToken.cache()
		if err != nil {
			panic(err)
		}
	}
}