/* bot.go
 * Contains information running the bot. Requires a discord bot token, mongodb uri, format style (e.g. swiss) and liquipedia tournament url
 * Authors: Zachary Bower
 * Last modified: November 24th, 2024
 */

package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/bwmarrin/discordgo"
	"github.com/go-andiamo/splitter"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var BotToken string
var Format string
var Round string
var Client *mongo.Client
var TournamentName string
var LiquipediaURL string

type UserPrediction struct {
	UserId string `bson:"userId,omitempty"`
	UserName string `bson:"userName,omitempty"`
	Win [2]string `bson:"win,omitempty"`
	Advance [6]string `bson:"advance,omitempty"`
	Lose [2]string `bson:"lose,omitempty"`
}

type Results struct {
	Round string `bson:"Round,omitempty"`
	TTL time.Time `bson:"TTL,omitempty"`
	Teams map[string]string `bson:"teams,omitempty"`
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message")
	}
}

func Run() {
	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running untill there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// respond to user message if it contains one of the following commands
	switch {
	case startsWith(message.Content, "$check"):
		checkPredictions(discord, message)
	case startsWith(message.Content, "$help"):
		discord.ChannelMessageSend(message.ChannelID, getHelpMessage())
	case startsWith(message.Content, "$leaderboard"):
		getLeaderboard(discord, message)
	case startsWith(message.Content, "$set"):
		setPredictions(discord, message)
	case startsWith(message.Content, "$teams"):
		discord.ChannelMessageSend(message.ChannelID, getTeamsMessage())
	case startsWith(message.Content, "$upcoming"):
		discord.ChannelMessageSend(message.ChannelID, "this feature hasn't been implemented yet")
	case startsWith(message.Content, "$hello"):
		discord.ChannelMessageSend(message.ChannelID, "Hello World!")
	}
}

// Function to check the current status of a user's predictions
// Preconditions: Recieves pointer to the discordgo session and discordgo message
// Postconditions: Sends the status of the users's predictions to the discord channel in the form Succeeded: {succeeded}, Failed: {failed}, Pending: {pending}"
func checkPredictions(discord *discordgo.Session, message *discordgo.MessageCreate) {
	//Check to see if the user has predicitons stored in the db, if not no point continuing
	pred_coll := Client.Database("user_pickems").Collection(fmt.Sprintf("%s_%s_predictions", TournamentName, Round))
	
	user_opts := options.FindOne()
	var result UserPrediction
	err := pred_coll.FindOne(context.TODO(), bson.D{{Key: "userId", Value: message.Author.ID}},user_opts).Decode(&result)
	if err != nil {
		// ErrNoDocuments means that the user does not have their predictions stored in the db
		if errors.Is(err, mongo.ErrNoDocuments) {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s does not have any Pickems stored. Use $set to set your predictions", message.Author.Username))
			return
		} else {
			panic(err)
		}
	}
	//Check if there are results currently stored in the db, if there is, check if we need to update them
	results := getResults(discord, message)
	response, _, _ := calculateScore(result, results)
	discord.ChannelMessageSend(message.ChannelID, response)
}

// Function to return the help message called by `$help`
// Preconditions: None
// Postconditions: Returns a string containing the entire help message
func getHelpMessage() string {
	var message = "PickEms Bot v2.0\n`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\"\n`$check`: shows the current status of your Pick'Ems\n`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n`$upcoming`: shows todays live and upcoming matches\n"
	return message
}

func getLeaderboard(discord *discordgo.Session, message *discordgo.MessageCreate) {
	//Get all users predictions from db
	coll := Client.Database("user_pickems").Collection(fmt.Sprintf("%s_%s_predictions", TournamentName, Round))
	opts := options.Find()

	cursor, err := coll.Find(context.TODO(), opts)
	if err != nil {
		log.Panic(err)
		discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
		return
	}
	var results []UserPrediction
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Panic(err)
		discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
		return
	}

	//Struct that defines a user for the leaderboard
	type User struct {
		name string
		succeeded int
		failed int
	}
	
	teamResults := getResults(discord, message)
	var leaderboard []User

	//Iterate through each user's predictions fetched from the db and append the required information to the leaderboard slice
	for _, result := range results {
		_, succeeded, failed := calculateScore(result, teamResults)	
		leaderboard = append(leaderboard, User{name: result.UserName, succeeded: succeeded, failed: failed})
	}

	//Order the leaderboard in decesending order so that the user with the highest score appear at the top. Note score = successes - failures and there is no tie breaker
	sort.Slice(leaderboard, func(i, j int) bool {
		return (leaderboard[i].succeeded -  leaderboard[i].failed) > (leaderboard[j].succeeded -  leaderboard[j].failed)
	})

	//Generate response string
	response := "The users with the best pickems are:\n"
	for i, user := range leaderboard {
		response += fmt.Sprintf("%d. %s, %d successes, %d failures\n", i, user.name, user.succeeded, user.failed)
	}

	//Finally we can send the response string to the discord channel
	discord.ChannelMessageSend(message.ChannelID, response)
}

// Function to return valid teams list to the
// Preconditions: None
// Postcondtions: Returns string to be sent in discord channel
func getTeamsMessage() string {
	validTeams := getValidTeams()
	returnString := fmt.Sprintf("Valid teams for the %s stage are:\n", Round)
	for _, team := range validTeams {
		returnString += fmt.Sprintf("- %s\n", team)
	}
	return returnString
}

// Function that processes the user input for `$set` message, validates the picks are correct and updates the values stored in the db
// Preconditions: Recieves pointer to discordgo session and discordgo message
// Postconditions: User predictions are updated if data is valid, else an error message is sent to the discord channel
func setPredictions(discord *discordgo.Session, message *discordgo.MessageCreate) {
	//Check if there is exactly 11 strings using a space delimiter in the message (command and 10 user picks)
	//Note this could be reduced to 10 as we dont need to store the command in memory after this point, but the memory usage of one section of a slice is not a big concern for this application
	spaceSplitter, _ := splitter.NewSplitter(' ', splitter.DoubleQuotes, splitter.LeftRightDoubleDoubleQuotes) //we use splitter here instead of go's build in splitter because now we can have team names that contain spaces e.g. "Faze Clan" recognised as one team not two
	msg, _ := spaceSplitter.Split(message.Content)

	if len(msg) != 11 { //If there is not the required amount of words, return an error
		discord.ChannelMessageSend(message.ChannelID, "Incorrect number of teams were supplied. Please try again")
		return
	}

	//Look up the teams that are competing in this round so we can validate a user has inputted valid team names
	validTeams := getValidTeams()
	var invalidTeams []string

	//Convert user predictions to lower case as this is how they are stored in the db and makes checks easier
	for i := 1; i < len(msg); i++ {
		msg[i] = strings.ReplaceAll(msg[i], "\"", "")
		msg[i] = strings.ReplaceAll(msg[i], "“", "")
		msg[i] = strings.ReplaceAll(msg[i], "”", "")
		msg[i] = strings.ToLower(msg[i])

		if !contains(validTeams, msg[i]) {
			invalidTeams = append(invalidTeams, msg[i])
		}
	}

	//Check if the user has tried to input an invalid team name
	if invalidTeams != nil {
		returnString := "The following team names are invalid: "
		for _, team := range invalidTeams {
			returnString += fmt.Sprintf("%s, ", team)
		}
		returnString += fmt.Sprintf("%s's Pickems have not been updated", message.Author.Username)
		discord.ChannelMessageSend(message.ChannelID, returnString)
		return
	}

	//Create lists for each section to be stored in the db
	var win [2]string
	win[0] = msg[1]
	win[1] = msg[2]

	var lose [2]string
	lose[0] = msg[9]
	lose[1] = msg[10]

	var advance [6]string
	for i := 0; i <= 5; i++ {
		advance[i] = msg[i+3]
	}

	coll := Client.Database("user_pickems").Collection(fmt.Sprintf("%s_%s_predictions", TournamentName, Round))

	opts := options.FindOne()
	var result bson.M
	err := coll.FindOne(
		context.TODO(),
		bson.D{{Key: "userId", Value: message.Author.ID}},
		opts,
	).Decode(&result)
	if err != nil {
		// ErrNoDocuments means that the user does not have their predictions stored in the db
		if errors.Is(err, mongo.ErrNoDocuments) {
			res, err := coll.InsertOne(context.TODO(), bson.D{{Key: "userId", Value: message.Author.ID}, {Key: "userName", Value: message.Author.Username}, {Key: "win", Value: win}, {Key: "advance", Value: advance}, {Key: "lose", Value: lose}})
			if err != nil {
				log.Panic(err)
				discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
				return
			}
			fmt.Printf("inserted document with ID %v\n", res.InsertedID)
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s's Pickems have been updated", message.Author.Username))
			return
		}
		log.Panic(err)
	} else {
		// This means the user already has stored pickems, and we should update that record instead of inserting a new one
		filter := bson.D{{Key: "userId", Value: message.Author.ID}}
		update := bson.D{{Key: "$set", Value: bson.D{{Key: "win", Value: win}, {Key: "advance", Value: advance}, {Key: "lose", Value: lose}}}}
		res, err := coll.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
			panic(err)
		}
		fmt.Printf("Pickems updated with ID %v\n", res.UpsertedID)
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("%s's Pickems have been updated", message.Author.Username))
	}
}

// Function to get a list of teams competing in the current round of the tournament
// Preconditions: LiquipediaURL is valid and the host is online
// Postconditions: Returns string slice containing list of competing teams
func getValidTeams() []string {
	table := getOverviewTable()
	rows := table.FindAll("tr")
	//Iterate over each row in the table. We start from index 1 not 0 as the first row just contains th not td and not skipping it causes more issues than it solves
	var teams []string
	for _, row := range rows[1:] {
		team := row.Find("span", "class", "team-template-text").Find("a").Text()
		teams = append(teams, strings.ToLower(team))
	}
	return teams
}

//Function to check if the results stored in the db exist, are out of date, or fine to use
//Preconditions: Recieves pointer to discordgo session and discordgo message
//Postconditions: returns a Results struct and updates the results cache in db if required
func getResults(discord *discordgo.Session, message *discordgo.MessageCreate) Results {
	coll := Client.Database("user_pickems").Collection(fmt.Sprintf("%s_results", TournamentName))
	opts := options.FindOne()
	var teams map[string]string
	var results Results

	results_err := coll.FindOne(
		context.TODO(),
		bson.D{{Key: "Round", Value: Round}},
		opts,
	).Decode(&results)
	if results_err != nil {
		// ErrNoDocuments means that there is no results for this round stored in the db
		if errors.Is(results_err, mongo.ErrNoDocuments) {
			fmt.Println("No results for this round stored, adding to db")
			teams = getTeams()
			//Convert map to Bson and update results in db
			bsonTeams := bson.M{}
			for key, value := range teams {
				bsonTeams[key] = value
			}
			res, err := coll.InsertOne(context.TODO(), bson.M{"Round": Round, "TTL": time.Now().Add(15 * time.Minute), "teams": bsonTeams})
			if err != nil {
				log.Panic(err)
				discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
			} else {
				fmt.Printf("Results stored with ID %v\n", res.InsertedID)
			}
		} else {
			log.Panic(results_err)
			discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
		}
	} else {
		//Check if the db needs updating
		if results.TTL.Compare(time.Now()) == -1  {
			fmt.Println("Cache is outdated... updating")
			// Update stored value
			teams = getTeams()
			//Convert map to Bson and update results in db
			bsonTeams := bson.M{}
			for key, value := range teams {
				bsonTeams[key] = value
			}
			filter := bson.D{{Key: "Round", Value: Round}}
			update := bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "Round", Value: Round},
					{Key: "TTL", Value: time.Now().Add(15 * time.Minute)},
					{Key: "teams", Value: bsonTeams},
				}},
			}
			res, err := coll.UpdateOne(context.TODO(), filter, update)
			if err != nil {
				discord.ChannelMessageSend(message.ChannelID, "An unexpected error has occured")
				log.Panic(err)
			} else {
				fmt.Printf("Stored results updated with ID%v\n", res.UpsertedID)
			}
			
		}
	}
	return results
}

//Function that calculates a user's score and generates a response to be sent in discord
//Preconditions: recieves UserPrediction struct of a player's predictions
//Postconditions: returns a string containing the response message, int containing the players successes and int containing the player's losses
func calculateScore(pred UserPrediction, results Results) (string, int, int) {
	//Comparisons for checks
	succeeded := 0
    pending := 0
    failed := 0

	response := fmt.Sprintf("%s's picks are:\n", pred.UserName)

	teams := results.Teams

	//3-0 Calculation
	response += "[3-0]\n"
	for i := range pred.Win {
		team := pred.Win[i]
		score := teams[team]
		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			log.Panic(err)
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			log.Panic(err)
		}

		var result string
		if loses >= 1 {
			result = "[Failed]"
			failed += 1
		} else if wins != 3 {
			result = "[Pending]" 
			pending += 1
		} else {
			result = "[Succeeded]"
			succeeded += 1
		}
		response += fmt.Sprintf("%s: %s %s\n", team, score, result)
	}

	//3-1/2 Calculation
	response += "[3-1, 3-2]\n"
	for i := range pred.Advance {
		team := pred.Advance[i]
		score := teams[team]
		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			log.Panic(err)
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			log.Panic(err)
		}

		var result string
		if loses == 3 || (wins == 3 && loses == 0) {
			result = "[Failed]"
			failed += 1
		} else if wins < 3 {
			result = "[Pending]" 
			pending += 1
		} else {
			result = "[Succeeded]"
			succeeded += 1
		}
		response += fmt.Sprintf("%s: %s %s\n", team, score, result)
	}
		//0-3 Calculation
	response += "[0-3]\n"
	for i := range pred.Lose {
		team := pred.Lose[i]
		score := teams[team]
		wins, err := strconv.Atoi(string(score[0]))
		if err != nil {
			log.Panic(err)
		}
		loses, err := strconv.Atoi(string(score[2]))
		if err != nil {
			log.Panic(err)
		}

		var result string
		if wins >= 1 {
			result = "[Failed]"
			failed += 1
		} else if loses != 3 {
			result = "[Pending]" 
			pending += 1
		} else {
			result = "[Succeeded]"
			succeeded += 1
		}
		response += fmt.Sprintf("%s: %s %s\n", team, score, result)
	}
	response += fmt.Sprintf("\nSucceeded: %d, Failed: %d, Pending: %d", succeeded, failed, pending)
	return response, succeeded, failed
}

//Function to scrape liquipedia and get the results table present on the tournament page
//Preconditions: None
//Postconditions: Returns Root pointer for a soup tree that points to the results table
func getOverviewTable() soup.Root {
	url := LiquipediaURL
	if Round == "opening" {
		url += "/Opening_Stage"
	} else if Round == "elimination" {
		url += "/Elimination_Stage"
	} else if Round == "playoffs" {
		url += "Playoff_Stage"
	} else {
		fmt.Println("Invalid round specified. Input should be 'opening', 'elimination' or 'playoffs'")
	}

	res, err := soup.Get(url)
	if err != nil {
		log.Panic(err)
	}
	page := soup.HTMLParse(res)
	table := page.Find("table", "class", "swisstable").Find("tbody")
	return table
}

//Function to get a list of valid team names for this round of the tournament and each teams score
//Preconditions: None
//Postconditions: Returns a map of the form teamName : score
func getTeams() map[string]string {
	//Iterate over each row in the table. We start from index 1 not 0 as the first row just contains th not td and not skipping it causes more issues than it solves
	table := getOverviewTable()
	rows := table.FindAll("tr")
	teams := make(map[string]string)
	for _, row := range rows[1:] {
		team := row.Find("span", "class", "team-template-text").Find("a").Text()
		team = strings.ToLower(team)
		score := row.Find("b").Text()
		if score == "-" {
			score = "0-0"
		}
		teams[team] = score
	}
	return teams

}

// Function to check if a slice contains an input string
// Preconditions: Recieves string slice and string
// Postconditions: Returns true if the input string is in the slice or false if it is not
func contains(slice []string, inputString string) bool {
	for _, sstring := range slice {
		if sstring == inputString {
			return true
		}
	}
	return false
}

//Function to check if a string starts with a given substring
//Preconditions: Recieves an input string and a substring
//Postconditions: Returns true if the substring is at the start of the string, else returns false
func startsWith(inputString string, substring string) bool {
	//Check if the substring is present in the input string
	if !strings.Contains(inputString, substring) {
		return false
	}
	strLength := len(substring)
	for i := range(strLength) {
		if inputString[i] != substring[i] {
			return false
		}
	}
	return true
}
