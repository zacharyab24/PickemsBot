# Pick'Ems Bot
## About
This is a discord bot used to track Pickems for the CS2 Copenhagen major \
Data is being scraped from [Liquipedia](https://liquipedia.net/counterstrike/PGL/2024/Copenhagen) \
The data scraping is relatively hard coded to the event however it could be modified for other events easily provided the Liquipedia page is a similar format \
The code is by no means efficient. Several methods were copy pasted instead of making it a function to get the results. No caching of matches / results is being done locally which means concurrent requests will all rescrape the data. Obviously this could be more efficient but I couldn't be bothered given its usage is currently private.

## Bot Commands
The following are discord messages that the bot will respond to. These can be in a server the bot is added to or dm'd to the bot. Note that there is no server-specific rankings. It is all global \
`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\" \
`$check`: shows the current status of your Pick'Ems \
`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms \
`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes \
`$upcoming`: shows todays live and upcoming matches

## Usage
To run this bot, first install packages: `pip install -r requirements.txt` \
Run the bot with `python bot.py` \
Alternatively use the docker image, this will provide a persistant bot so if you close the terminal the bot doesn't go offline. \
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