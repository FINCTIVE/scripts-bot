package main

import (
	tb "gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

var bot *tb.Bot
var config Config

// launch loads the yaml conf file, and start the bot.
func launch(initFunc func(bot *tb.Bot)) {
	configBytes, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	bot, err = tb.NewBot(tb.Settings{
		Token:  config.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	initFunc(bot)
	logVerbose("bot started")
	bot.Start()
}

// longestMessageLen defines the max length of single message.
// If it's too long, cut it into pieces and sendQuick separately.
const longestMessageLen = 4000

// maxSendRetry ...
const maxSendRetry = 1000

func sendQuick(sender *tb.User, message string, options ...interface{}) {
	send(sender, message, "", "", options...)
}

// send() sends message. If failed, retry until it's successful.
// Also, send() split long message into small pieces. and sendQuick them separately.
// (Telegram has message length limit.)
// It adds prefix and suffix to every single messages.
func send(sender *tb.User, message, prefix, suffix string, options ...interface{}) (tgMessage *tb.Message) {
	if len(message) == 0 {
		return nil
	}
	messages := splitByLines(message, longestMessageLen-len([]rune(prefix))-len([]rune(suffix)))
	retryCounter := 0
	for {
		var err error
		tgMessage, err = bot.Send(sender, prefix+messages[0]+suffix, options...)
		if err != nil {
			logWarn("send message => err:", err, "; message:", messages[0])
			retryCounter++
			if retryCounter >= maxSendRetry {
				lastSignalMessage := "Messages not sent. " +
					"Please check your terminal log." +
					"It may not be an issue with Networking."
				logError("Max retry.", maxSendRetry, "times.", lastSignalMessage)
				// last signal: for errors not related with network. Maybe we will get it!
				_, lastErr := bot.Send(sender, lastSignalMessage, options...)
				if lastErr != nil {
					logError("last Signal err:", lastErr)
				}
				break
			}
		} else {
			if len(messages) == 1 {
				break
			} else {
				messages = messages[1:]
			}
		}
	}
	return
}

// splitByLines will split input string into pieces within limit length.
// It splits input by lines. (\n)
// If one line is too long, it will be broken into two results item.
func splitByLines(input string, limit int) (results []string) {
	if utf8.RuneCountInString(input) <= limit {
		return []string{input}
	}

	messageRunes := []rune(input)
	var splitMessage [][]rune

	startIndex := 0
	for {
		cutIndex := startIndex + limit - 1
		if cutIndex > len(messageRunes)-1 {
			cutIndex = len(messageRunes) - 1
		}
		fullLine := false
		for i := cutIndex; i >= startIndex; i-- {
			if messageRunes[i] == '\n' {
				splitMessage = append(splitMessage, messageRunes[startIndex:i+1])
				startIndex = i + 1
				fullLine = true
				break
			}
		}
		if !fullLine {
			splitMessage = append(splitMessage, messageRunes[startIndex:cutIndex+1])
			startIndex = cutIndex + 1
		}
		if startIndex == len(messageRunes) {
			break
		}
	}

	for _, v := range splitMessage {
		msg := strings.Trim(string(v), " \n")
		if len(msg) != 0 {
			results = append(results, msg)
		}
	}
	return
}
