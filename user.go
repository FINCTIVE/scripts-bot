package main

import tb "gopkg.in/tucnak/telebot.v2"

// checkUser will check whether the username is in the config.yaml
func checkUser(sender *tb.User) (pass bool) {
	if len(config.Users) == 0 || config.Users[0] == "*" {
		return true
	}

	pass = false
	for _, username := range config.Users {
		if username == sender.Username {
			pass = true
			break
		}
	}
	if pass == false {
		sendQuick(sender, "Sorry, you can't access the bot.")
	}
	logVerbose("check:", sender.Username, "; pass:", pass)
	return pass
}
