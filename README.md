# subreddit-logger

This software watches a set of subreddits and tracks new posts shortly after their creation. Tracked posts are watched and their upvote + comment counts are recorded on defined intervals. This software is useful if you wish to observe specific subreddits for upvote manipulation.

## installation
After you download and build the source, some things are required for subreddit-logger  to work.

### .env
A `.env` file located in the same directory as your build is required. See `.env.template` for guidance on what information is required for this program to work. All configuration, besides for tracked subreddits, is defined in this file.

### database
a MongoDB server is used by this software to record data. The connection string is required in the `.env` file. More info on the database can be found in `.env.template`

### reddit API secret
As expected, you need API credentials from Reddit. See https://www.reddit.com/prefs/apps to obtain a client and secret for your `.env`.

## usage
After you have your executable built and `.env` filled out, you need to specify the subreddits to watch the `subreddits.json` file stored in the same directory. Example:
```
{
    "subreddits": [
        "subredditname",
        "anothersubredditname",
        "wallstreetbets",
        "clubpenguin"
    ]
}
```