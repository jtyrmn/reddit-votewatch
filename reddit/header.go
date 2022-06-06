package reddit

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jtyrmn/reddit-votewatch/util"
)

//function to set standard outgoing headers to reddit.com
//only useful for queries after you get the access token, not before
func populateStandardHeaders(header *http.Header, token accessTokenResponse) {
	userAgent := util.GetEnv("REDDIT_USERAGENT_STRING")
	authorization := fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)

	header.Add("user-agent", userAgent)
	header.Add("authorization", authorization)
}

//get the time an http response was sent
func getTimeOfSending(response *http.Response) (uint64, error) {

	//RFC 7231: time formatting of date headers
	formatting := "Mon, 02 Jan 2006 15:04:05 GMT"

	header, exists := response.Header["Date"]
	if !exists {
		return 0, errors.New("http response lacks Date header") //should never happen
	}

	date, err := time.Parse(formatting, header[0])
	if err != nil {
		return 0, errors.New("error parsing http date header:\n" + err.Error()) //also should never happen
	}

	return uint64(date.Unix()), nil

}
