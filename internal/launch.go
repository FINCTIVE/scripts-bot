package internal

import (
	tb "gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"time"
)

var GlobalBot *tb.Bot

// Launch loads the yaml conf file, and start the bot.
func Launch(initFunc func(bot *tb.Bot)) {
	configBytes, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = yaml.Unmarshal(configBytes, &GlobalConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	GlobalBot, err = tb.NewBot(tb.Settings{
		Token:  GlobalConfig.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	initFunc(GlobalBot)
	LogVerbose("bot started")
	GlobalBot.Start()
}
