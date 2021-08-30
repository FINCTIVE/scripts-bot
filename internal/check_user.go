package internal

import (
	tb "gopkg.in/tucnak/telebot.v2"
)

// CheckUser will check whether the username is in the GlobalConfig.yaml
func CheckUser(sender *tb.User) (pass bool) {
	LogVerbose("check user: ", sender.Username)
	if len(GlobalConfig.Users) == 0 || GlobalConfig.Users[0] == "*" {
		return true
	}

	pass = false
	for _, username := range GlobalConfig.Users {
		if username == sender.Username {
			pass = true
			break
		}
	}
	if pass == false {
		SendQuick(sender, "Sorry, you can't access the bot.")
	}
	LogVerbose("check:", sender.Username, "; pass:", pass)
	return pass
}
