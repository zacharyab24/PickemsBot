# bot.py
import asyncio
import os

import pymongo
from dotenv import load_dotenv
import discord
from discord.ext import commands
from discord.ext.commands import MissingRequiredArgument
from bs4 import BeautifulSoup
import requests
from datetime import datetime
from dateutil import tz

#Function to check the results of a match
#Preconditions: recieves team1 and team2 (strings) and score (string of the form X-X)
#Postconditions: returns team1 if team1 wins, team2 if team2 wins and pending if the match hasn't finished
def check_match(team1, team2, score):
    team1_score = int(score[0])
    team2_score = int(score[2])

    if team1_score == 2:
        result = team1
    elif team2_score == 2:
        result = team2
    else:
        result = "pending"
    return result

load_dotenv()

#mongodb stuff
URI = os.getenv('MONGO_URI')
print(URI)
client = pymongo.MongoClient(URI)
db = client.user_pickems

#discordpy stuff
TOKEN = os.getenv('DISCORD_TOKEN')
print(TOKEN)

intents = discord.Intents.default()
intents.message_content = True
bot = commands.Bot(command_prefix='$', intents=intents)

@bot.command()
async def set(ctx, *args):
    if len(args) != 7:
        await ctx.channel.send("Incorrect number of teams were supplied. Please try again")
        return
    
    list_args = list(args)
    list_args = list(map(lambda x:x.lower(), list_args))

    user = ctx.author.id
    key = {'user': user}

    accepted_teams = list(map(lambda x:x.lower(), ["Cloud9", "Team Vitality", "Team Spirit", "FaZe Clan", "Eternal Fire", "Natus Vincere", "MOUZ", "G2 Esports"]))
    #accepted_teams = list(map(lambda x:x.lower(), ["Heroic", "FaZe Clan", "GamerLegion", "Monte", "Team Liquid", "Apeks", "Into The Breach", "Team Vitality"])) Paris Major, used for testing as of time of writing the copenhagen major only has teams for quarterfinals, not semis or gf
    
    for team in list_args:
        if team not in accepted_teams:
            await ctx.channel.send(f"{team} not recognised")
            await ctx.channel.send(f"<@{ctx.author.id}>'s picks have not been updated")
            return
    
    if (db.user_set_pickems.find_one(key)):
        await ctx.channel.send(f"<@{ctx.author.id}> already has pickems stored. Do you wish to update? (y/n)")

        def check(m):
            return m.author == ctx.author and m.channel == ctx.channel

        try:
            response = await bot.wait_for('message', check=check, timeout=30.0)
        except asyncio.TimeoutError:
            return
        
        if response.content.lower() not in ("yes", "y"):
            return
        
        #update here
        filter = {"user": ctx.author.id}
        new_picks = {
            "$set": {"semi": [args[0], args[1], args[2], args[3]], "gf": [args[4], args[5]], "gf-winner": args[6]}
        }
        db.user_set_pickems.update_one(filter, new_picks)

    else:
        db.user_set_pickems.insert_one(
            {
                "user": ctx.author.id,
                "semi": [args[0], args[1], args[2], args[3]], 
                "gf": [args[4], args[5]],
                "gf-winner": args[6]
            }
        )

    await ctx.channel.send(f"{ctx.author}'s Pickems have been updated")
    return

@bot.command()
async def check(ctx):
    # Scrape data. This result should get cached to save api calls / time to check / processing cycles
    #URL = "https://liquipedia.net/counterstrike/BLAST/Major/2023/Paris/Champions_Stage" #Paris
    URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Playoff_Stage" #Copenhagen
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    team_code = soup.findAll(class_="hidden-xs")
    scores_code = soup.findAll(class_="brkts-opponent-score-inner")

    teams = []
    scores = []
    for i in team_code:
        teams.append(i.text)
    for i in scores_code:
        if i.text == "":
            scores.append("0")
        else:
            scores.append(i.text)

    if len(teams) == 8:
        teams.insert(4, "TBD")
    if len(teams) == 9:
        teams.insert(5, "TBD")
    if len(teams) == 10:
        teams.insert(10, "TBD")
    if len(teams) == 11:
        teams.insert(11, "TBD")
    if len(teams) == 12:
        teams.insert(12, "TBD")
    if len(teams) == 13:
        teams.insert(13, "TBD")
    matches = []
    for team in teams:
        for score in scores:
            matches.append([team, score])
            scores.remove(score)
            break

    #Hard coded lists for quarterfinals, semifinals and grandfinal to get data in desired format as liquipedia is not of that form
    qf = [matches[0], matches[1], matches[2], matches[3], matches[6], matches[7], matches[8], matches[9]]
    sf = [matches[4], matches[5], matches[10], matches[11]]
    gf = [matches[12], matches[13]]

    #Get user picks
    user = ctx.author.id
    key = {'user': user}

    if not (db.user_set_pickems.find_one(key)):
        await ctx.channel.send(f"<@{ctx.author.id}> doesn't have Pick'Ems configured. Please configure them with $set")
        return
    
    succeeded = 0
    pending = 0
    failed = 0

    pickems = db.user_set_pickems.find_one(key)
    semis = pickems['semi']
    formatted_semis = []
    for team in semis:
        formatted_semis.append(team.lower())

    grand_final = pickems['gf']
    formatted_grand_final = []
    for team in grand_final:
        formatted_grand_final.append(team.lower())
    
    grand_final_winner = pickems['gf-winner']
    formatted_grand_final_winner = grand_final_winner.lower()

    res = f"<@{ctx.author.id}>'s picks are:\n"
    res += "[Quarterfinals]\n" 
    for i in range(len(qf)):
        if i%2 == 1:
            continue
        team1 = qf[i][0]
        team2 = qf[i+1][0]
        team1_score = qf[i][1]
        team2_score = qf[i+1][1]
        score = team1_score + "-" + team2_score
        winner = check_match(team1, team2, score)
        
        if team1.lower() in formatted_semis:
            res += f"{team1} to beat {team2} "
        if team2.lower() in formatted_semis:
            res += f"{team2} to beat {team1} "
        
        if winner == "pending":
            pending += 1
            res += " [Pending]\n"
        elif winner.lower() in formatted_semis:
            succeeded += 1
            res += " [Succeeded]\n"
        else:
            failed += 1
            res += " [Failed]\n"

    res += "\n[Semifinals]\n" 
    if ["TBD", "0"] in sf:
        res +="Semifinal teams aren't confirmed yet\n"
        pending += 2
    else:
        for i in range(len(sf)):
            if i%2 == 1:
                continue
            team1 = sf[i][0]
            team2 = sf[i+1][0]
            team1_score = sf[i][1]
            team2_score = sf[i+1][1]
            score = team1_score + "-" + team2_score
            winner = check_match(team1, team2, score)

            if team1.lower() in formatted_grand_final:
                res += f"{team1} to beat {team2} "
            elif team2.lower() in formatted_grand_final:
                res += f"{team2} to beat {team1} "
            elif team1.lower() not in formatted_grand_final or team2.lower() not in formatted_grand_final:
                res += f"At least one of the picked teams did not make it to the Semifinals. The matchup is {team1} vs {team2}"

            if winner == "pending":
                pending += 1
                res += " [Pending]\n"
            elif winner.lower() in formatted_grand_final:
                succeeded += 1
                res += " [Succeeded]\n"
            else:
                failed += 1
                res += " [Failed]\n"

    res += "\n[Grand Final]\n" 
    if ["TBD", "0"] in gf:
        res += "Grand-final teams aren't confirmed yet\n"
        pending += 1
    else:
        team1 = gf[0][0]
        team2 = gf[1][0]
        team1_score = gf[0][1]
        team2_score = gf[1][1]
        score = team1_score + "-" + team2_score
        winner = check_match(team1, team2, score)

        if team1.lower() == formatted_grand_final_winner:
            res += f"{team1} to beat {team2} "
        elif team2.lower() in formatted_grand_final_winner:
            res += f"{team2} to beat {team1} "
        elif team1.lower() not in formatted_grand_final or team2.lower() not in formatted_grand_final:
            res += f"At least one of the picked teams did not make it to the Grand Final. The matchup is {team1} vs {team2}"

        if winner == "pending":
            pending += 1
            res += " [Pending]\n"
        elif winner.lower() == formatted_grand_final_winner:
            succeeded += 1
            res += " [Succeeded]\n"
        else:
            failed += 1
            res += " [Failed]\n"

    res += f"\nSucceeded: {succeeded}, Failed: {failed}, Pending: {pending}"
    await ctx.channel.send(res)
    return

@bot.command()
async def teams(ctx):
    #Hard coded for Copenhagen Finals
    teams = ["Cloud9", "Team Vitality", "Team Spirit", "FaZe Clan", "Eternal Fire", "Natus Vincere", "MOUZ", "G2 Esports"]

    res = ""
    for team in teams:
        res += f'"{team}", '
    await ctx.channel.send(f"The teams in the current stage are {res}")

@bot.command()
async def leaderboard(ctx):
    # Scrape data. This result should get cached to save api calls / time to check / processing cycles
    URL = "https://liquipedia.net/counterstrike/BLAST/Major/2023/Paris/Champions_Stage" #Paris
    #URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Playoff_Stage" #Copenhagen
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    team_code = soup.findAll(class_="hidden-xs")
    scores_code = soup.findAll(class_="brkts-opponent-score-inner")

    teams = []
    scores = []
    for i in team_code:
        teams.append(i.text)
    for i in scores_code:
        if i.text == "":
            scores.append("0")
        else:
            scores.append(i.text)

    if len(teams) == 8:
        teams.insert(4, "TBD")
    if len(teams) == 9:
        teams.insert(5, "TBD")
    if len(teams) == 10:
        teams.insert(10, "TBD")
    if len(teams) == 11:
        teams.insert(11, "TBD")
    if len(teams) == 12:
        teams.insert(12, "TBD")
    if len(teams) == 13:
        teams.insert(13, "TBD")
    matches = []
    for team in teams:
        for score in scores:
            matches.append([team, score])
            scores.remove(score)
            break

    #Hard coded lists for quarterfinals, semifinals and grandfinal to get data in desired format as liquipedia is not of that form
    qf = [matches[0], matches[1], matches[2], matches[3], matches[6], matches[7], matches[8], matches[9]]
    sf = [matches[4], matches[5], matches[10], matches[11]]
    gf = [matches[12], matches[13]]

    #Get user picks
    user = ctx.author.id
    key = {'user': user}

    if not (db.user_set_pickems.find_one(key)):
        await ctx.channel.send(f"<@{ctx.author.id}> doesn't have Pick'Ems configured. Please configure them with $set")
        return
    
    succeeded = 0
    pending = 0
    failed = 0

    pickems = db.user_set_pickems.find_one(key)
    semis = pickems['semi']
    formatted_semis = []
    for team in semis:
        formatted_semis.append(team.lower())

    grand_final = pickems['gf']
    formatted_grand_final = []
    for team in grand_final:
        formatted_grand_final.append(team.lower())
    
    grand_final_winner = pickems['gf-winner']
    formatted_grand_final_winner = grand_final_winner.lower()

    for i in range(len(qf)):
        if i%2 == 1:
            continue
        team1 = qf[i][0]
        team2 = qf[i+1][0]
        team1_score = qf[i][1]
        team2_score = qf[i+1][1]
        score = team1_score + "-" + team2_score
        winner = check_match(team1, team2, score)
        
        if winner == "pending":
            pending += 1
        elif winner.lower() in formatted_semis:
            succeeded += 1
        else:
            failed += 1

    if ["TBD", "0"] in sf:
        pending += 2
    else:
        for i in range(len(sf)):
            if i%2 == 1:
                continue
            team1 = sf[i][0]
            team2 = sf[i+1][0]
            team1_score = sf[i][1]
            team2_score = sf[i+1][1]
            score = team1_score + "-" + team2_score
            winner = check_match(team1, team2, score)

            if winner == "pending":
                pending += 1
            elif winner.lower() in formatted_grand_final:
                succeeded += 1
            else:
                failed += 1
 
    if ["TBD", "0"] in gf:
        pending += 1
    else:
        team1 = gf[0][0]
        team2 = gf[1][0]
        team1_score = gf[0][1]
        team2_score = gf[1][1]
        score = team1_score + "-" + team2_score
        winner = check_match(team1, team2, score)

        if winner == "pending":
            pending += 1
        elif winner.lower() == formatted_grand_final_winner:
            succeeded += 1
        else:
            failed += 1

    #Old code for sweedish format
    # URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Elimination_Stage"
    # page = requests.get(URL)

    # soup = BeautifulSoup(page.content, "html.parser")
    # table = soup.find(class_="swisstable")

    # teams = {}

    # for row in table:
    #     for col in row:
    #         team = col.find_next(class_="team-template-text").text.lower()
    #         score = col.find_next("b").string
    #         if score == "-":
    #             score = "0-0"
    #         teams[team] = score

    # leaderboard={}
    # res=""
    # for pickems in db.user_set_pickems.find({}):
    #     succeeded = 0
    #     pending = 0
    #     failed = 0
    #     _3_0 = pickems['3-0']
    #     for i in _3_0:
    #         i = i.lower()
    #     _0_3 = pickems['0-3']
    #     for i in _0_3:
    #         i = i.lower()
    #     _3_1_2 = pickems['advance']
    #     for i in _3_1_2:
    #         i = i.lower()

    #     for i in _3_0:
    #         i = i.lower()
    #         score = str(teams[i])
    #         wins = int(score[0])
    #         loses = int(score[2])

    #         if loses >= 1:
    #             failed += 1
    #         elif wins != 3:
    #             pending += 1
    #         else:
    #             succeeded += 1

    #     for i in _3_1_2:
    #         i = i.lower()
    #         score = str(teams[i])
    #         wins = int(score[0])
    #         loses = int(score[2])
    #         if loses == 3 or (wins == 3 and loses == 0):
    #             failed += 1
    #         elif wins < 3:
    #             pending += 1
    #         else:
    #             succeeded += 1

    #     for i in _0_3:
    #         i = i.lower()
    #         score = str(teams[i])
    #         wins = int(score[0])
    #         loses = int(score[2])

    #         if wins >= 1:
    #             failed += 1
    #         elif loses != 3:
    #             pending += 1
    #         else:
    #             succeeded += 1

    leaderboard = {}

    score = 0 + succeeded - failed
    li = [pickems['user'], succeeded, failed]
    data = tuple(li)
    leaderboard[data] = score

    sorted_leaderboard = sorted(leaderboard.items(), key=lambda x:x[1], reverse=True)
    sorted_leaderboard = dict(sorted_leaderboard)
    
    output = "The users with the best pickems are:\n"
    counter = 1
    for data in sorted_leaderboard:
        user = data[0]
        successes = data[1]
        failures = data[2]

        output += f"{counter}. <@{user}>, {successes} successes, {failures} failures\n"
        counter += 1
    await ctx.channel.send(f"{output}\n")

@bot.command()
async def upcoming(ctx):
    from_zone = tz.tzutc()
    to_zone = tz.tzlocal()

    URL = "https://liquipedia.net/counterstrike/Liquipedia:Matches"
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    games = soup.findAll(class_="infobox_matches_content")

    matches = {}
    for game in games:
        div = game.find(class_="toggle-area-1")
        team1 = game.find_next("td")
        team1_name = team1.find_next("a").get('title')
        vs = game.find_next("td")
        team2 = game.find_next(class_="team-right")
        team2_name = team2.find_next("a").get('title')

        tournament = game.find_next("tr")
        tournament = tournament.find_next("tr")
        time_until = tournament.find_next(class_="match-countdown").text
        tournament = tournament.find_next("a").text
        
        if tournament == "PGL Major Copenhagen 2024":
            if f"{team1_name} vs {team2_name}: {time_until}" in matches:
                continue
            else:
                if "CET" in time_until:
                    continue

                time = time_until.split(" ")
                day = time[1].replace(",", "")
                time = time[4].split(":")
                hr = time[0]
                minute = time[1]

                time_string = f"2024-03-{day} {hr}:{minute}:00"
                utc = datetime.strptime(time_string, '%Y-%m-%d %H:%M:%S')
                utc = utc.replace(tzinfo=from_zone)
                central = utc.astimezone(to_zone)
                timestamp = int(central.timestamp())

                matches[f"{team1_name} vs {team2_name}"] = timestamp

    response = ""
    for i in matches:
        response += (f"{i} <t:{matches[i]}>\n")
    await ctx.send(response)

@bot.command(name="get_help")
async def get_help(ctx):
    string = "PickEms Bot v1.1\n`$set [team1] [team2] ... [team7]`: Sets your Pick'Ems. 1-4 are the teams that will win the quarter finals and make it to the semis, 5-6 are the two teams to make it to the grand final, 7 is the team that will win the grand final. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\"\n`$check`: shows the current status of your Pick'Ems\n`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n`$upcoming`: shows todays live and upcoming matches\n"

    await ctx.channel.send(string)
    return


bot.run(TOKEN)