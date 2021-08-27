package main

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
	"os"
	"os/exec"
)

func main() {
	launch(func(bot *tb.Bot) {
		var stopCurrentDownload context.CancelFunc
		bot.Handle(tb.OnText, func(m *tb.Message) {
			pass := checkUser(m.Sender)
			if !pass {
				return
			}

			// cancel function
			var ctx context.Context
			ctx, stopCurrentDownload = context.WithCancel(context.Background())
			cmd := exec.CommandContext(ctx, dlPath, urls...)
			cmd.Env = os.Environ()
			cmd.Dir = "/root/downloads"
			done := runCommand(m.Sender, cmd, &tb.SendOptions{ReplyTo: m})
			go func() {
				<-done
				sendQuick(m.Sender, "done!", &tb.SendOptions{ReplyTo: m})
			}()
		})

		bot.Handle("/stop", func(m *tb.Message) {
			if stopCurrentDownload != nil {
				stopCurrentDownload()
			}
		})

		bot.Handle("/ping", func(m *tb.Message) {
			var ctx context.Context
			ctx, stopCurrentDownload = context.WithCancel(context.Background())
			cmd := exec.CommandContext(ctx, "ping", "baidu.com")
			_ = runCommand(m.Sender, cmd, &tb.SendOptions{ReplyTo: m})
		})

		_ = bot.SetCommands([]tb.Command{
			{"/stop", "stop the running task"},
			{"/ping", "ping baidu"},
		})
	})
}
