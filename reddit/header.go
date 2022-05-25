package reddit

import (
	"fmt"
	"net/http"

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
