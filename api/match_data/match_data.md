# Match Data
This sub package provides the models and functions used for fetching data within this application. Api calls are to the
[WikiMedia API](https://liquipedia.net/counterstrike/api.php), and [Liquidpedia DB API](https://api.liquipedia.net/documentation)
The LiquipediaDB api requires a key, this is located in `.env` (excluded from the git repo) and loaded at run time. It 
exists in an environmental variable called `LIQUIDPEDIADB_API_KEY`. API calls are rate limited to 60 calls per hour. 


## Database
To help comply with rate limiting requirements on the data sources, a copy of the relevant data is stored in the DB for 
this project (MongoDB), that is hosted locally. Within the DB, we have two collections related to match data for a
tournament , `match_results` and `upcoming_matches`:

`match_results` stores the results for matches that have occured. Match data is structured in code as an interface 
called `ResultRecord` with a type of `swiss` or `single-elimination`. Each `ResultRecord` contains a `string` for 
`Round` (this is the round of the tournament, e.g. Stage_1, Playoffs, etc), `int64` for `TTL` (this is a unix epoch time 
representing how long data is valid for in the database), and a map `Teams` of `team_name : results`.

For the type `SwissResultRecord`, `Teams` is a `string` to `string` map where the results are `wins-loses` (e.g. 3-2).
For the type `EliminationResultRecord`, `Teams` is a `string` to `TeamProgress` where team progress is a struct 
containing the highest round a team made it to, and the status ('eliminated', 'advanced' or 'pending'). An example of 
this is `MOUS:{Semi Final, eliminated}`
