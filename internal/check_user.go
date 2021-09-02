package internal

import (
	tb "gopkg.in/tucnak/telebot.v2"
)

// CheckUser will check whether the username is in the GlobalConfig.yaml
func CheckUser(bot *tb.Bot, sender *tb.User, users []string) (pass bool) {
	LogVerbose("check user: ", sender.Username)
	if len(users) == 0 || users[0] == "*" {
		return true
	}

	pass = false
	for _, username := range users {
		if username == sender.Username {
			pass = true
			break
		}
	}
	if pass == false {
		SendQuick(bot, sender, "Sorry, you can't access the bot.")
	}
	LogVerbose("check:", sender.Username, "; pass:", pass)
	return pass
}
