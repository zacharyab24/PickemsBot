FROM python:3.11

ADD finals_bot.py .
ADD .env .

RUN pip install pymongo python-dotenv requests beautifulsoup4 discord.py python-dateutil

CMD ["python", "./finals_bot.py"]