package internal

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
	"strings"
	"time"
	"unicode/utf8"
)

// longestMessageLen defines the max length of single message.
// If it's too long, cut it into pieces and SendQuick separately.
const longestMessageLen = 4000

// maxSendRetry ...
const maxSendRetry = 1000

// SendQuick send quicker!!!
func SendQuick(to tb.Recipient, message string, options ...interface{}) (tgMessage *tb.Message) {
	return Send(to, message, "", "", options...)
}

// Send sends message. If failed, retry until it's successful.
// Also, Send() split long message into small pieces. and SendQuick them separately.
// (Telegram has message length limit.)
// It adds prefix and suffix to every single messages.
func Send(to tb.Recipient, message, prefix, suffix string, options ...interface{}) (tgMessage *tb.Message) {
	if len(message) == 0 {
		return nil
	}
	messages := splitByLines(message, longestMessageLen-len([]rune(prefix))-len([]rune(suffix)))
	retryCounter := 0
	for {
		var err error
		tgMessage, err = GlobalBot.Send(to, prefix+messages[0]+suffix, options...)
		if err != nil {
			LogWarn("send message => err:", err, "; message:", messages[0])
			retryCounter++
			if retryCounter >= maxSendRetry {
				lastSignalMessage := "Messages not sent. " +
					"Please check your terminal log." +
					"It may not be an issue with Networking."
				LogError("Max retry.", maxSendRetry, "times.", lastSignalMessage)
				// last signal: for errors not related with network. Maybe we will get it!
				_, lastErr := GlobalBot.Send(to, lastSignalMessage, options...)
				if lastErr != nil {
					LogError("last Signal err:", lastErr)
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

// SendUpdate will send messages to the user and update the message per second.
// For terminal output, use StartCmd() to get terminalBytes and done channel.
// NOTICE: This function only sends one message. Old words will be replaced with new ones.
func SendUpdate(to tb.Recipient, messageBytes *[]byte, prefix, suffix string,
	ctx context.Context, options ...interface{}) (msg *tb.Message) {
	// send messages
	go func() {
		for {
			select {
			case <-time.After(time.Second):
				messageStr := FormatTerminal(string(*messageBytes))
				if len(messageStr) == 0 {
					messageStr = "..." // placeholder, command started
				}

				// limit message, only send the last part
				messageRunes := []rune(messageStr)
				surroundLen := len(prefix) + len(suffix)
				if len(messageRunes) > longestMessageLen-surroundLen {
					messageRunes = messageRunes[len(messageRunes)-longestMessageLen-surroundLen-1:]
					messageStr = string(messageRunes)
				}
				messageStr = prefix + messageStr + suffix
				var err error
				if msg == nil {
					// NOTICE: no need to use send(), no retry. (sending messages per second)
					msg, err = GlobalBot.Send(to, messageStr, options...)
					if err != nil {
						LogWarn("send terminal err", err)
						msg = nil
					}
				} else {
					if msg.Text != messageStr {
						// Ignore edit err. The message will be deleted anyway.
						// A common err is that old message is still the same as the new one.
						_, _ = GlobalBot.Edit(msg, messageStr, options...)
					}
				}
			case <-ctx.Done():
				if msg != nil {
					deleteErr := GlobalBot.Delete(msg)
					if deleteErr != nil {
						LogWarn("delete terminal messages err:", deleteErr)
					}
				}
				return
			}
		}
	}()
	return msg
}
