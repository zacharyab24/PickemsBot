# Pick'Ems Bot
## About
This is a discord bot used to track Pick'Ems for the CS2 Copenhagen major. The source of data for this project is 
[Liquipedia](https://liquipedia.net/counterstrike/PGL/2024/Copenhagen). Originally, the bot did data scraping using soup to extract the data from the liquipedia match page,
however, this proved to be inefficient, unreliable, and cause unexpected errors. We now use the Liquipedia api and db to 
obtain information. This data is stored in our own database to reduce the number of calls we must make to Liquipedia's 
servers, reducing latency and complying with the [API usage requirements](https://liquipedia.net/api-terms-of-use).

## Bot Commands
The following are discord messages that the bot will respond to. These can be in a server the bot is added to or dm'd to the bot. Note that there is no server-specific rankings. It is all global \
`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams, and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how they appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\" \
`$check`: shows the current status of your Pick'Ems \
`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your Pick'Ems \
`$leaderboard`: shows which users have the best Pick'Ems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes \
`$upcoming`: shows todays live and upcoming matches

## Usage
TODO: Update commands for V3.

Run the bot with `go run main.go -format="<format>" -url="<url>` \
Alternatively, use the docker image, this will provide a persistant bot so if you close the terminal the bot doesn't go offline. \
Build the docker image with `docker build -t pickems-bot .` \
Run the container with `docker run pickems-bot`  

## Version History
### v1.0
Launched bot \
Added allowing any capitalisation of teams \
Added error handling for incorrect inputs

### v1.1
Added upcoming match support. This may still be broken. I have to wait for today's matches to be finished to check \
Updated help command
Updated formatting for check command

### v2.0
Updated the code base to use go instead of python.
Updated to work with upcoming Perfect World Shanghai Major as well as be more expandible for other tournaments through the use of command line flags (not user facing)

### v3.0
Reworking the application into two parts: `api` and `bot`
- `api` is a restful api that is used for data retrieval. Lookups will be to the Liquipedia Database instead of scraping 
the site using soup. Most of the data will be stored in our own database, as to not exceed the usage requirements of the
liquipedia 
- `bot` will make api calls to GET and POST data, instead of doing its own database interacton and web scraping. This 
will allow for a smoother experience, cleaner code, and  less errors.