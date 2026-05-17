# Changelog

## v3.1
Bot now spins up a web server that receives callbacks from Liquipedia when a page is updated. This means we automatically
update our cached data, instead of relying on users making commands. This means responses are faster when previously the
cache would be expired. Some other minor improvements and changes have been made and test coverage has been added.

## v3.0
Reworking the application into two parts: `api` and `bot`
- `api` is a restful api that is used for data retrieval. Lookups will be to the Liquipedia Database instead of scraping
the site using soup. Most of the data will be stored in our own database, as to not exceed the usage requirements of the
liquipedia
- `bot` will make api calls to GET and POST data, instead of doing its own database interaction and web scraping. This
will allow for a smoother experience, cleaner code, and less errors.

## v2.0
Updated the code base to use Go instead of Python.
Updated to work with upcoming Perfect World Shanghai Major as well as be more expandable for other tournaments through the use of command line flags (not user facing).

## v1.1
Added upcoming match support. This may still be broken. I have to wait for today's matches to be finished to check. \
Updated help command. \
Updated formatting for check command.

## v1.0
Launched bot. \
Added allowing any capitalisation of teams. \
Added error handling for incorrect inputs.
