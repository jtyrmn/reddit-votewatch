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
)

//container to hold a standard access token recieved from https://www.reddit.com/api/v1/access_token
type accessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpireLength int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func (a accessTokenResponse) String() string {
	return fmt.Sprintf("{<REDACTED> %s %d %s}", a.TokenType, a.ExpireLength, a.Scope)
}

//the api handler object
//should be created using NewApi()
type RedditApiHandler struct {
	accessToken accessTokenResponse

	//client info you should've gotten from https://www.reddit.com/prefs/apps
	clientId     string
	clientSecret string

	//reddit account of your bot
	redditUsername string
	redditPassword string
}

//dont want to print out private secrets + passwords while debugging
func (r RedditApiHandler) String() string {
	return fmt.Sprintf("{%s %s <REDACTED> %s <REDACTED>}", r.accessToken, r.clientId, r.redditUsername)
}

//Creates new RedditApiHandler. Make sure you have the proper env variables
func NewApi() RedditApiHandler {
	client := RedditApiHandler{
		clientId:       getEnv("REDDIT_CLIENT_ID"),
		clientSecret:   getEnv("REDDIT_CLIENT_SECRET"),
		redditUsername: getEnv("REDDIT_USERNAME"),
		redditPassword: getEnv("REDDIT_PASSWORD"),
	}

	accessTokenResponse, err := fetchAccessToken(client)
	if err != nil {
		log.Fatal("error requesting access token:\n" + err.Error())
	}
	client.accessToken = *accessTokenResponse

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
		return nil, errors.New("error querying for access token: " + err.Error())
	}

	//debugging
	fmt.Println(response.StatusCode)

	//if reddit api rejects our request (unauthorizeed)
	if response.StatusCode == http.StatusUnauthorized {
		return nil,errors.New("unauthorized client credentials")
	}

	
	responseData, err := ioutil.ReadAll(response.Body)
	fmt.Println(string(responseData))
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
		return nil, errors.New("response error from requesting access token: " + responseError.E + "\nperhaps your reddit account login info is incorrect?")
	}
	
	var responseJSON accessTokenResponse
	err = json.Unmarshal(responseData, &responseJSON)
	if err != nil {
		return nil, errors.New("error parsing access token response body: " + err.Error())
	}

	return &responseJSON, nil
}

func getEnv(str string) string {
	v, exists := os.LookupEnv(str)
	if !exists {
		log.Fatalf("cannot find environment variable \"%s\": halting execution...\n", str)
	}

	return v
}
