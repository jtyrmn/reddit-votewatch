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
	"net/http"
	"os"
	"strings"
	"time"
)

//container to hold a standard access token recieved from https://www.reddit.com/api/v1/access_token
type accessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpireLength int    `json:"expires_in"`
	Scope        string `json:"scope"`

	//when the access token was recieved from reddit.com. Formatted as unix time (time.Now().Unix()).
	//not that this information is not included in the raw accessTokenResponse from reddit.com, so don't forget to manually set this after unmarshaling.
	InitializationTime int64 `json:"initialization_time"`
}

//**** IMPORTANT: never call cache() or pullFromCache() below if env var CACHE_ACCESS_TOKEN is not true, because ACCESS_TOKEN_PATH will probably not be set and the program will crash

//save the access token and its metadata to filesystem. Returns nil if successful
func (a *accessTokenResponse) cache() error {
	json, _ := json.Marshal(a) //encoding a static struct should never return an error I assume
	err := os.WriteFile(getEnv("ACCESS_TOKEN_PATH"), json, 0666)
	if err != nil {
		return errors.New("error caching access token: " + err.Error())
	}
	return nil
}

//attempt to recieve access token from cache. if cache wasn't found and there wasn't any other error, this function will return (nil, nil)
func (a accessTokenResponse) pullFromCache() (*accessTokenResponse, error) {
	path := getEnv("ACCESS_TOKEN_PATH")

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
type RedditApiHandler struct {
	accessToken      accessTokenResponse
	cacheAccessToken bool //whether or not the access token should be cached/decached

	//client info you should've gotten from https://www.reddit.com/prefs/apps
	clientId     string
	clientSecret string

	//reddit account of your bot
	redditUsername string
	redditPassword string
}

//dont want to print out private secrets + passwords while debugging
func (r RedditApiHandler) String() string {
	return fmt.Sprintf("{%s %v %s <REDACTED> %s <REDACTED>}", r.accessToken, r.cacheAccessToken, r.clientId, r.redditUsername)
}

//NewApi() creates a reddit api client and also initializes
//OAuth2 authentication. Unless data is pulled from cache, this function will call the reddit api

//make sure you have all the env variables assigned before calling this
func NewApi() RedditApiHandler {
	client := RedditApiHandler{
		clientId:         getEnv("REDDIT_CLIENT_ID"),
		clientSecret:     getEnv("REDDIT_CLIENT_SECRET"),
		redditUsername:   getEnv("REDDIT_USERNAME"),
		redditPassword:   getEnv("REDDIT_PASSWORD"),
		cacheAccessToken: strings.ToLower(GetEnvDefault("CACHE_ACCESS_TOKEN", "true")) == "true", /*theres probably a better way to do this*/
	}

	//recieve access token, either by cache or request to api
	lookupAccessTokenCache := client.cacheAccessToken
	if lookupAccessTokenCache { //look in cache
		token, err := client.accessToken.pullFromCache()
		if token == nil {
			if err != nil { //if there was error
				fmt.Printf("error pulling access token from cache:\n%s\n", err.Error())
			} else { //pullFromCache() returning (nil, nil) means the cache doesn't exist/isn't created yet
				fmt.Printf("cache not found at %s\n", GetEnvDefault("ACCESS_TOKEN_PATH", "<ACCESS_TOKEN_PATH>"))
			}

			lookupAccessTokenCache = false //if we couldn't find the access token, must query api for it
		} else {
			fmt.Print("found access token in cache\n")
			client.accessToken = *token
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


	return client
}

//call reddit and request an access token
func fetchAccessToken(client RedditApiHandler) (*accessTokenResponse, error) {
	requestBody := fmt.Sprintf("grant_type=password&username=%s&password=%s", client.redditUsername, client.redditPassword)
	request, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		return nil, errors.New("should this error ever occur? " + err.Error())
	}

	request.Header = http.Header{
		"user-agent":    []string{getEnv("REDDIT_USERAGENT_STRING")},
		"authorization": []string{"basic " + base64.StdEncoding.EncodeToString([]byte(client.clientId+":"+client.clientSecret))},
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

func getEnv(str string) string {
	v, exists := os.LookupEnv(str)
	if !exists {
		log.Fatalf("cannot find environment variable \"%s\": halting execution...\n", str)
	}

	return v
}

//equivelant to getEnv except doesn't cause an error and substitutes a default value (def)
func GetEnvDefault(str string, def string) string {
	var v string
	v, exists := os.LookupEnv(str)
	if !exists {
		fmt.Printf("warning: env variable %s not found, defaulting to \"%s\"...\n", str, def)
		return def
	}

	return v
}
