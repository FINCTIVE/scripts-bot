package main

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	util "scripts-bot/internal"
	"strconv"
	"time"
)

// FILE HIERARCH STRUCTURE
// .
// ├── bot
// └── configs
//     ├── config.yaml
//     └── scripts
//         └── (...)

const configDir = "configs"

var configFilePath = filepath.Join(configDir, "config.yaml")
var scriptsDir = filepath.Join(configDir, "scripts")

var config Config

type Config struct {
	BotToken string   `yaml:"botToken"`
	Users    []string `yaml:"users"`
}

var defaultCommands = []tb.Command{
	{"/sh", "sh [your command]"},
	{"/ps", "show all running tasks"},
	{"/ls", "show script files"},
	{"/stop", "stop a task by id"},
}

func main() {
	// read config file
	configBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	bot, err := tb.NewBot(tb.Settings{
		Token:  config.BotToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	pool := util.NewTaskPool(2000)
	updateCommandList(bot, pool)

	bot.Handle("/start", func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		util.SendQuick(bot, m.Chat, "Hi! You can try:"+`<pre>/sh ping baidu.com</pre>`,
			&tb.SendOptions{ParseMode: tb.ModeHTML})
	})

	bot.Handle("/sh", func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}

		// check empty
		if len(m.Payload) == 0 {
			util.SendQuick(bot, m.Chat, "No command found.\nExample: "+
				`<pre>/sh ping baidu.com</pre>`,
				&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
			return
		}
		util.LogVerbose("/sh", m.Payload)
		sendCmdMessage(bot, m, pool, "bash", "-c", m.Payload)
	})

	bot.Handle("/ps", func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		tasks := pool.List()
		if len(tasks) == 0 {
			util.SendQuick(bot, m.Chat, "No running task.")
			return
		}
		for _, v := range tasks {
			util.SendQuick(bot, m.Chat, v)
		}
	})

	bot.Handle("/stop", func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		id, parseErr := strconv.Atoi(m.Payload)
		if parseErr != nil {
			util.SendQuick(bot, m.Chat, "Please input a task ID number.\nExample: "+
				`<pre>/stop 233</pre>`,
				&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
			return
		}
		ok := pool.Cancel(id)
		if !ok {
			util.SendQuick(bot, m.Chat, "Task ID not found.",
				&tb.SendOptions{ReplyTo: m, ParseMode: tb.ModeHTML})
		}
	})

	// upload script file by sending .sh file to the bot
	bot.Handle(tb.OnDocument, func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		if m.Document.MIME != "application/x-shellscript" {
			return
		}
		path := filepath.Join(scriptsDir, m.Document.FileName)
		util.LogVerbose("upload:", path)
		// download file from telegram server
		fileURL, saveErr := bot.FileURLByID(m.Document.File.FileID)
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, "Can not save the file! Error: "+saveErr.Error())
			return
		}
		resp, saveErr := http.Get(fileURL)
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, "Can not save the file! Error: "+saveErr.Error())
			return
		}
		bytes, saveErr := ioutil.ReadAll(resp.Body)
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, "Can not save the file! Error: "+saveErr.Error())
			return
		}
		saveErr = resp.Body.Close()
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, "Can not save the file! Error: "+saveErr.Error())
			return
		}
		saveErr = os.WriteFile(path, bytes, 0770)
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, "Can not save the file! Error: "+saveErr.Error())
			return
		}

		// checking...
		saveErr = util.CheckBashSyntax(path)
		if saveErr != nil {
			util.SendQuick(bot, m.Chat, saveErr.Error())
			saveErr = os.Remove(path) // something wrong, remove the file.
			if saveErr != nil {
				util.SendQuick(bot, m.Chat, saveErr.Error())
			}
			return
		}

		// finish adding
		updateCommandList(bot, pool)
		sendScriptMessage(bot, m, pool, m.Document.FileName)
		util.SendQuick(bot, m.Chat, "New script added!")
	})

	bot.Handle("/ls", func(m *tb.Message) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		files, err := os.ReadDir(scriptsDir)
		if err != nil {
			util.LogError("ls:", err)
			return
		}
		for _, v := range files {
			sendScriptMessage(bot, m, pool, v.Name())
		}
	})

	// go !!
	util.LogVerbose("bot started")
	bot.Start()
}

func sendCmdMessage(bot *tb.Bot, m *tb.Message, pool *util.TaskPool, cmdName string, cmdArgs ...string) {
	// init
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	taskId, err := pool.Add("bash "+m.Payload, cancel)
	if err != nil {
		util.SendQuick(bot, m.Chat, err.Error(), &tb.SendOptions{ReplyTo: m})
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
	util.SendUpdate(bot, m.Chat, terminalBytes, "<pre>", "</pre>", ctx, terminalOptions)

	// stop button pressed, stop the task
	bot.Handle(&stopButton, func(c *tb.Callback) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
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
		_ = util.Send(bot, m.Chat, util.FormatTerminal(string(*terminalBytes)),
			"<pre>", "</pre>", termFullOptions)
		// error
		if cmdErr != nil {
			util.SendQuick(bot, m.Chat, "<pre>"+cmdErr.Error()+"</pre>",
				termFullOptions)
		}
	}()
}

func sendScriptMessage(bot *tb.Bot, m *tb.Message, pool *util.TaskPool, scriptName string) {
	// message options
	var fileMenu = &tb.ReplyMarkup{}
	// for some reason, tele bot callback routing doesn't support ".sh"
	uniqueStr := scriptName[:len(scriptName)-len(filepath.Ext(scriptName))]
	delButton := fileMenu.Data("🗑️Delete", "del"+uniqueStr, scriptName)
	runButton := fileMenu.Data("▶Run", "run"+uniqueStr, scriptName)
	fileMenu.Inline(fileMenu.Row(delButton, runButton))

	fileMessage := util.SendQuick(bot, m.Chat, "<pre>"+scriptName+"</pre>", &tb.SendOptions{
		ReplyMarkup: fileMenu,
		ParseMode:   tb.ModeHTML,
	})

	bot.Handle(&runButton, func(c *tb.Callback) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		util.LogVerbose("run file:", c.Data)
		sendCmdMessage(bot, m, pool, "bash", filepath.Join(scriptsDir, c.Data))
		// *** Always Respond ***
		errResp := bot.Respond(c, &tb.CallbackResponse{})
		if errResp != nil {
			util.LogError("bot.Response err: ", errResp)
		}
	})

	bot.Handle(&delButton, func(c *tb.Callback) {
		pass := util.CheckUser(bot, m.Sender, config.Users)
		if !pass {
			return
		}
		util.LogVerbose("delete file:", c.Data)
		errDel := os.Remove(filepath.Join(scriptsDir, c.Data))
		if errDel != nil {
			util.SendQuick(bot, c.Message.Chat, "Delete file error: "+errDel.Error())
		}
		errDel = bot.Delete(fileMessage)
		if errDel != nil {
			util.SendQuick(bot, c.Message.Chat, "Delete message error: "+errDel.Error())
		}
		// *** Always Respond ***
		errResp := bot.Respond(c, &tb.CallbackResponse{})
		if errResp != nil {
			util.LogError("bot.Response err: ", errResp)
		}
	})
}

func updateCommandList(bot *tb.Bot, pool *util.TaskPool) {
	// update command list
	commands := defaultCommands
	files, err := os.ReadDir(scriptsDir)
	if err != nil {
		util.LogError("error reading script folder:", err)
		return
	}
	for _, v := range files {
		scriptName := "/" + v.Name()[:len(v.Name())-len(filepath.Ext(v.Name()))]
		commands = append(commands, tb.Command{scriptName, "---"})
		bot.Handle(scriptName, func(msg *tb.Message) {
			pass := util.CheckUser(bot, msg.Sender, config.Users)
			if !pass {
				return
			}
			sendCmdMessage(bot, msg, pool, "bash", filepath.Join(scriptsDir, v.Name()))
		})
	}
	setCmdListErr := bot.SetCommands(commands)
	if setCmdListErr != nil {
		util.LogWarn(setCmdListErr)
	}
}
