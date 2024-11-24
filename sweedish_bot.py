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

@bot.command(name='set')
async def set(ctx, *args):
    if len(args) != 10:
        await ctx.channel.send("Incorrect number of teams were supplied. Please try again")
        return
    
    list_args = list(args)
    list_args = list(map(lambda x:x.lower(), list_args))

    _3_0 = list_args[0], list_args[1]
    _0_3 = list_args[8], list_args[9]
    _3_1_2 = [list_args[2], list_args[3], list_args[4], list_args[5], list_args[6], list_args[7]]

    user = ctx.author.id
    key = {'user': user}

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
        
        accepted_teams = list(map(lambda x:x.lower(), ['FaZe Clan', 'Team Spirit', 'Team Vitality', 'MOUZ', 'Complexity', 'Virtus.pro', 'Natus Vincere', 'G2 Esports', 'HEROIC', 'Cloud9', 'Eternal Fire', 'ECSTATIC', 'paiN Gaming', 'Imperial Esports', 'The MongolZ', 'FURIA Esports']))
        
        for team in list_args:
            if team not in accepted_teams:
                await ctx.channel.send(f"{team} not recognised")
                await ctx.channel.send(f"<@{ctx.author.id}>'s picks have not been updated")
                return

        #update here
        filter = {"user": ctx.author.id}
        new_picks = {
            "$set": {"3-0": _3_0,"advance": _3_1_2,"0-3": _0_3}
        }
        db.user_set_pickems.update_one(filter, new_picks)

    else:
        db.user_set_pickems.insert_one(
            {
                "user": ctx.author.id,
                "3-0": _3_0,
                "advance": _3_1_2,
                "0-3": _0_3
            }
        )

    await ctx.channel.send(f"{ctx.author}'s Pickems have been updated")

@bot.command()
async def check(ctx):
    #Scrape data. This result should get cached to save api calls / time to check / processing cycles
    URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Elimination_Stage"
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    table = soup.find(class_="swisstable")

    teams = {}
    formatted_teams = {}
    for row in table:
        for col in row:
            team = col.find_next(class_="team-template-text").text
            formatted_teams[team.lower()] = team
            team = team.lower()
            score = col.find_next("b").string
            if score == "-":
                score = "0-0"
            teams[team] = score

    #await ctx.channel.send(teams)

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
    _3_0 = pickems['3-0']
    for i in _3_0:
        i = i.lower()
    _0_3 = pickems['0-3']
    for i in _0_3:
        i = i.lower()
    _3_1_2 = pickems['advance']
    for i in _3_1_2:
        i = i.lower()

    response=f"<@{ctx.author.id}>'s picks are:\n"
    response+="[3-0]\n"
    for i in _3_0:
        score = str(teams[i])
        wins = int(score[0])
        loses = int(score[2])

        if loses >= 1:
            result = "[Failed]"
            failed += 1
        elif wins != 3:
            result = "[Pending]"
            pending += 1
        else:
            result = "[Succeeded]"
            succeeded += 1
        response += f"{formatted_teams[i]}: {teams[i]} {result}\n"

    response += "\n[3-1, 3-2]\n"
    for i in _3_1_2:
        score = str(teams[i])
        wins = int(score[0])
        loses = int(score[2])

        if loses == 3 or (wins == 3 and loses == 0):
            result = "[Failed]"
            failed += 1
        elif wins < 3:
            result = "[Pending]"
            pending += 1
        else:
            result = "[Succeeded]"
            succeeded += 1
        
        response += f"{formatted_teams[i]}: {teams[i]} {result}\n"

    response += "\n[0-3]\n"
    for i in _0_3:
        score = str(teams[i])
        wins = int(score[0])
        loses = int(score[2])

        if wins >= 1:
            result = "[Failed]"
            failed += 1
        elif loses != 3:
            result = "[Pending]"
            pending += 1
        else:
            result = "[Succeeded]"
            succeeded += 1
        response += f"{formatted_teams[i]}: {teams[i]} {result}\n"

    response += f"\nSucceeded: {succeeded}, Failed: {failed}, Pending: {pending}"
    await ctx.channel.send(response)
    return

@bot.command()
async def teams(ctx):
    #Scrape data. This result should get cached to save api calls / time to check / processing cycles
    URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Elimination_Stage"
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    table = soup.find(class_="swisstable")

    teams = []

    for row in table:
        for col in row:
            team = col.find_next(class_="team-template-text").text
            if team in teams:
                continue
            else:
                teams.append(team)
    
    res = ""
    for team in teams:
        res += f'"{team}", '
    await ctx.channel.send(f"The teams in the current stage are {res}")

@bot.command()
async def leaderboard(ctx):
    URL = "https://liquipedia.net/counterstrike/PGL/2024/Copenhagen/Elimination_Stage"
    page = requests.get(URL)

    soup = BeautifulSoup(page.content, "html.parser")
    table = soup.find(class_="swisstable")

    teams = {}

    for row in table:
        for col in row:
            team = col.find_next(class_="team-template-text").text.lower()
            score = col.find_next("b").string
            if score == "-":
                score = "0-0"
            teams[team] = score

    leaderboard={}
    res=""
    for pickems in db.user_set_pickems.find({}):
        succeeded = 0
        pending = 0
        failed = 0
        _3_0 = pickems['3-0']
        for i in _3_0:
            i = i.lower()
        _0_3 = pickems['0-3']
        for i in _0_3:
            i = i.lower()
        _3_1_2 = pickems['advance']
        for i in _3_1_2:
            i = i.lower()

        for i in _3_0:
            i = i.lower()
            score = str(teams[i])
            wins = int(score[0])
            loses = int(score[2])

            if loses >= 1:
                failed += 1
            elif wins != 3:
                pending += 1
            else:
                succeeded += 1

        for i in _3_1_2:
            i = i.lower()
            wins = int(score[0])
            loses = int(score[2])

            if loses == 3 or (wins == 3 and loses == 0):
                failed += 1
            elif wins < 3:
                pending += 1
            else:
                succeeded += 1
            
        for i in _0_3:
            i = i.lower()
            score = str(teams[i])
            wins = int(score[0])
            loses = int(score[2])

            if wins >= 1:
                failed += 1
            elif loses != 3:
                pending += 1
            else:
                succeeded += 1

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
    await ctx.channel.send(f"{output}\n{res}")

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
    string = "PickEms Bot v1.1\n`$set [team1] [team2] ... [team10]`: Sets your Pick'Ems. 1 & 2 are the 3-0 teams, 3-8 are the 3-1 / 3-2 teams and 9-10 are the 0-3 teams. Please note that the teams names need to be specified exactly how the appear on liquipedia (not case sensitive) as I'm not doing any proper checking. Names that contains two or more words need to be encased in \" \". E.g. \"The MongolZ\"\n`$check`: shows the current status of your Pick'Ems\n`$teams`: shows the teams currently in the current stage of the tournament. Use this list to set your PickEms\n`$leaderboard`: shows which users have the best pickems in the current stage. This is sorted by number of successful picks. There is no tie breaker in the event two users have the same number of successes\n`$upcoming`: shows todays live and upcoming matches\n"
    await ctx.channel.send(string)
    return


bot.run(TOKEN)