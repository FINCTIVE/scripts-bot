package main

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
	"os/exec"
	util "scripts-bot/internal"
	"strconv"
)

func main() {
	util.Launch(func(bot *tb.Bot) {
		pool := util.NewTaskPool(2000)

		commands := []tb.Command{
			//{"/start", "hello!"},
			{"/add_script", ""},
			{"/sh", "sh [your command]"},
			{"/ps", "list running tasks"},
			{"/stop", "stop a task by id"},
		}
		setCmdListErr := bot.SetCommands(commands)
		if setCmdListErr != nil {
			util.LogWarn(setCmdListErr)
		}

		bot.Handle("/start", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}
			util.SendQuick(m.Sender, "hi!")
		})

		bot.Handle("/add_script", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}

			util.SendQuick(m.Sender, "test!")
			newCommandErr := bot.SetCommands(commands)
			if newCommandErr != nil {
				util.LogWarn(newCommandErr)
			}
		})

		bot.Handle("/sh", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}

			// check empty
			if len(m.Payload) == 0 {
				util.SendQuick(m.Sender, "No command found.\nExample: "+
					`<pre>/sh ping baidu.com</pre>`,
					&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
				return
			}
			util.LogVerbose("/sh", m.Payload)

			runCommand(m, pool, bot, "bash", "-c", m.Payload)
		})

		bot.Handle("/ps", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}
			tasks := pool.List()
			if len(tasks) == 0 {
				util.SendQuick(m.Sender, "No running task.")
				return
			}
			for _, v := range tasks {
				util.SendQuick(m.Sender, v)
			}
		})

		bot.Handle("/stop", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}
			id, parseErr := strconv.Atoi(m.Payload)
			if parseErr != nil {
				util.SendQuick(m.Sender, "Please input a task ID number.\nExample: "+
					`<pre>/stop 233</pre>`,
					&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
				return
			}
			ok := pool.Cancel(id)
			if !ok {
				util.SendQuick(m.Sender, "Task ID not found.",
					&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
			}
		})
	})
}

func runCommand(m *tb.Message, pool *util.TaskPool, bot *tb.Bot, cmdName string, cmdArgs ...string) {
	// init
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	taskId, err := pool.Add("bash "+m.Payload, cancel)
	if err != nil {
		util.SendQuick(m.Sender, err.Error(), &tb.SendOptions{ReplyTo: m})
		return
	}

	// message options
	var stopMenu = &tb.ReplyMarkup{}
	stopButton := stopMenu.Data("🚷Stop the task", strconv.Itoa(taskId), strconv.Itoa(taskId))
	stopMenu.Inline(stopMenu.Row(stopButton))
	terminalOptions := &tb.SendOptions{
		ReplyTo:               m,
		ReplyMarkup:           stopMenu,
		ParseMode:             tb.ModeHTML,
		DisableNotification:   true,
		DisableWebPagePreview: true,
	}

	// start cmd
	terminalBytes, cmdDone := util.StartCmd(cmd)
	util.SendUpdate(m.Sender, terminalBytes, "<pre>", "</pre>", ctx, terminalOptions)

	// stop button pressed, stop the task
	bot.Handle(&stopButton, func(c *tb.Callback) {
		id, errConvStop := strconv.Atoi(c.Data)
		if errConvStop != nil {
			util.LogError("stop button err: ", err)
		}
		pool.Cancel(id)
		// *** Always Respond ***
		errResp := bot.Respond(c, &tb.CallbackResponse{})
		if errResp != nil {
			util.LogError("bot.Response err: ", err)
		}
	})

	// Cmd finished
	// Or cancel() is called
	go func() {
		cmdErr := <-cmdDone
		_ = pool.Cancel(taskId)

		// send full terminal message
		termFullOptions := &tb.SendOptions{
			ReplyTo:               m,
			ParseMode:             tb.ModeHTML,
			DisableNotification:   true,
			DisableWebPagePreview: true,
		}
		_ = util.Send(m.Sender, util.FormatTerminal(string(*terminalBytes)),
			"<pre>", "</pre>", termFullOptions)
		// error
		if cmdErr != nil {
			util.SendQuick(m.Sender, "<pre>"+cmdErr.Error()+"</pre>",
				termFullOptions)
		}
	}()
}
