package main

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	util "scripts-bot/internal"
	"strconv"
)

const custumScriptsDir = "scripts"

func main() {
	util.Launch(func(bot *tb.Bot) {
		pool := util.NewTaskPool(2000)

		// default commands
		defaultCommands := []tb.Command{
			{"/sh", "sh [your command]"},
			{"/ps", "show all running tasks"},
			{"/ls", "show script files"},
			{"/stop", "stop a task by id"},
		}
		setCmdListErr := bot.SetCommands(defaultCommands)
		if setCmdListErr != nil {
			util.LogWarn(setCmdListErr)
		}

		bot.Handle("/start", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}
			util.SendQuick(m.Sender, "Hi! You can try:"+`<pre>/sh ping baidu.com</pre>`)
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

			runCommand(bot, m, pool, "bash", "-c", m.Payload)
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

		bot.Handle(tb.OnDocument, func(m *tb.Message) {
			if m.Document.MIME != "application/x-shellscript" {
				return
			}
			path := filepath.Join(custumScriptsDir, m.Document.FileName)
			util.LogVerbose("upload:", path)
			// download from telegram server
			fileURL, saveErr := bot.FileURLByID(m.Document.File.FileID)
			if saveErr != nil {
				util.SendQuick(m.Sender, "Can not save the file! Error: "+saveErr.Error())
				return
			}
			resp, saveErr := http.Get(fileURL)
			defer resp.Body.Close()
			if saveErr != nil {
				util.SendQuick(m.Sender, "Can not save the file! Error: "+saveErr.Error())
				return
			}
			bytes, saveErr := ioutil.ReadAll(resp.Body)
			if saveErr != nil {
				util.SendQuick(m.Sender, "Can not save the file! Error: "+saveErr.Error())
				return
			}
			saveErr = os.WriteFile(path, bytes, 0770)
			if saveErr != nil {
				util.SendQuick(m.Sender, "Can not save the file! Error: "+saveErr.Error())
				return
			}

			// syntax
			saveErr = util.CheckBashSyntax(path)
			if saveErr != nil {
				util.SendQuick(m.Sender, saveErr.Error())
				saveErr = os.Remove(path)
				if saveErr != nil {
					util.SendQuick(m.Sender, saveErr.Error())
				}
				return
			}

			// finish adding
			sendScript(bot, m, pool, m.Document.FileName)

			// update command list
			commands := defaultCommands
			files, err := os.ReadDir(custumScriptsDir)
			if err != nil {
				util.LogError("ls:", err)
				return
			}
			for _, v := range files {
				scriptName := "/" + v.Name()[:len(v.Name())-len(filepath.Ext(v.Name()))]
				commands = append(commands, tb.Command{scriptName, "---"})
				bot.Handle(scriptName, func(msg *tb.Message) {
					runCommand(bot, msg, pool, "bash", filepath.Join(custumScriptsDir, v.Name()))
				})
			}
			setCmdListErr = bot.SetCommands(commands)
			if setCmdListErr != nil {
				util.LogWarn(setCmdListErr)
			}
			util.SendQuick(m.Sender, "New script added!")
		})

		bot.Handle("/ls", func(m *tb.Message) {
			pass := util.CheckUser(m.Sender)
			if !pass {
				return
			}
			files, err := os.ReadDir(custumScriptsDir)
			if err != nil {
				util.LogError("ls:", err)
				return
			}
			for _, v := range files {
				sendScript(bot, m, pool, v.Name())
			}
		})
	})
}

func runCommand(bot *tb.Bot, m *tb.Message, pool *util.TaskPool, cmdName string, cmdArgs ...string) {
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

func sendScript(bot *tb.Bot, m *tb.Message, pool *util.TaskPool, scriptName string) {
	// message options
	var fileMenu = &tb.ReplyMarkup{}
	// for some reason, tele bot callback routing doesn't support ".sh"
	uniqueStr := scriptName[:len(scriptName)-len(filepath.Ext(scriptName))]
	delButton := fileMenu.Data("🗑️Delete", "del"+uniqueStr, scriptName)
	runButton := fileMenu.Data("▶Run", "run"+uniqueStr, scriptName)
	fileMenu.Inline(fileMenu.Row(delButton, runButton))

	fileMessage := util.SendQuick(m.Sender, "<pre>"+scriptName+"</pre>", &tb.SendOptions{
		ReplyMarkup: fileMenu,
		ParseMode:   tb.ModeHTML,
	})

	bot.Handle(&runButton, func(c *tb.Callback) {
		util.LogVerbose("run file:", c.Data)
		runCommand(bot, m, pool, "bash", filepath.Join(custumScriptsDir, c.Data))
		// *** Always Respond ***
		errResp := bot.Respond(c, &tb.CallbackResponse{})
		if errResp != nil {
			util.LogError("bot.Response err: ", errResp)
		}
	})

	bot.Handle(&delButton, func(c *tb.Callback) {
		util.LogVerbose("delete file:", c.Data)
		errDel := os.Remove(filepath.Join(custumScriptsDir, c.Data))
		if errDel != nil {
			util.SendQuick(c.Sender, "Delete file error: "+errDel.Error())
		}
		errDel = bot.Delete(fileMessage)
		if errDel != nil {
			util.SendQuick(c.Sender, "Delete message error: "+errDel.Error())
		}
		// *** Always Respond ***
		errResp := bot.Respond(c, &tb.CallbackResponse{})
		if errResp != nil {
			util.LogError("bot.Response err: ", errResp)
		}
	})
}
