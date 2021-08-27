package main

import (
	tb "gopkg.in/tucnak/telebot.v2"
	"io"
	"os/exec"
	"time"
)

// sendTerminal will send terminal output messages to the user
// and update the message per second.
// Until it received done signal.
// NOTICE: Telegram message ParseMode is forced to be ModeHTML
func sendTerminal(sender *tb.User, terminalBytes *[]byte, done chan error, options ...interface{}) {
	// option settings
	setHTMLParse := false
	for i := range options {
		value, ok := options[i].(*tb.SendOptions)
		if ok {
			(*value).ParseMode = tb.ModeHTML // reset parse mode
			setHTMLParse = true
		}
	}
	if !setHTMLParse {
		options = append(options, &tb.SendOptions{ParseMode: tb.ModeHTML})
	}
	options = append(options, tb.Silent, tb.NoPreview)

	// send messages
	go func() {
		var terminalMessage *tb.Message
		for {
			select {
			case <-time.After(time.Second):
				messageStr := formatTerminal(string(*terminalBytes))
				if len(messageStr) == 0 {
					messageStr = "..." // placeholder, command started
				}

				// limit message, only send the tail
				messageRunes := []rune(messageStr)
				surroundLen := len("<pre>" + "</pre>")
				if len(messageRunes) > longestMessageLen-surroundLen {
					messageRunes = messageRunes[len(messageRunes)-longestMessageLen-surroundLen-1:]
					messageStr = string(messageRunes)
				}
				messageStr = "<pre>" + messageStr + "</pre>"
				var err error
				if terminalMessage == nil {
					// NOTICE: no need to use send(), no retry. (sending messages per second)
					terminalMessage, err = bot.Send(sender, messageStr, options...)
					if err != nil {
						logWarn("send terminal err", err)
						terminalMessage = nil
					}
				} else {
					if terminalMessage.Text != messageStr {
						// Ignore edit err. The message will be deleted anyway.
						// A common err is that old message is still the same as the new one.
						_, _ = bot.Edit(terminalMessage, messageStr, options...)
					}
				}
			case err := <-done:
				if terminalMessage != nil {
					deleteErr := bot.Delete(terminalMessage)
					if deleteErr != nil {
						logWarn("delete terminal messages err:", deleteErr)
					}
				}
				_ = send(sender, formatTerminal(string(*terminalBytes)), "<pre>", "</pre>", options...)
				if err != nil {
					sendQuick(sender, "<pre>"+err.Error()+"</pre>", options...)
				}
				return
			}
		}
	}()
}

// runCommand will run the command and send terminal output messages to the user.
// listen to done to wait until the command is finished.
// NOTICE: Telegram message ParseMode is forced to be ModeHTML
//func runCommand(sender *tb.User, cmd *exec.Cmd, options ...interface{}) (done chan error) {
//	done = make(chan error, 0)
//
//	// option settings
//	setHTMLParse := false
//	for i := range options {
//		value, ok := options[i].(*tb.SendOptions)
//		if ok {
//			(*value).ParseMode = tb.ModeHTML // reset parse mode
//			setHTMLParse = true
//		}
//	}
//	if !setHTMLParse {
//		options = append(options, &tb.SendOptions{ParseMode: tb.ModeHTML})
//	}
//	options = append(options, tb.Silent, tb.NoPreview)
//
//	// run the command
//	output, doneCmd := runCmdAndCapture(cmd)
//
//	// send messages
//	go func() {
//		var terminalMessage *tb.Message
//		for {
//			select {
//			case <-time.After(time.Second):
//				messageStr := formatTerminal(string(*output))
//				if len(messageStr) == 0 {
//					messageStr = "..." // placeholder, command started
//				}
//
//				// limit message, only send the tail
//				messageRunes := []rune(messageStr)
//				surroundLen := len("<pre>" + "</pre>")
//				if len(messageRunes) > longestMessageLen-surroundLen {
//					messageRunes = messageRunes[len(messageRunes)-longestMessageLen-surroundLen-1:]
//					messageStr = string(messageRunes)
//				}
//				messageStr = "<pre>" + messageStr + "</pre>"
//				var err error
//				if terminalMessage == nil {
//					// NOTICE: no need to use send(), no retry
//					terminalMessage, err = bot.Send(sender, messageStr, options...)
//					if err != nil {
//						logWarn("send terminal err", err)
//						terminalMessage = nil
//					}
//				} else {
//					if terminalMessage.Text != messageStr {
//						// Ignore edit err. The message will be deleted anyway.
//						// A common err is that old message is still the same as the new one.
//						_, _ = bot.Edit(terminalMessage, messageStr, options...)
//					}
//				}
//			case err := <-doneCmd:
//				if terminalMessage != nil {
//					deleteErr := bot.Delete(terminalMessage)
//					if deleteErr != nil {
//						logWarn("delete terminal messages err:", deleteErr)
//					}
//				}
//				_ = send(sender, formatTerminal(string(*output)), "<pre>", "</pre>", options...)
//				if err != nil {
//					sendQuick(sender, "<pre>"+err.Error()+"</pre>", options...)
//				}
//				done <- err
//				return
//			}
//		}
//	}()
//	return
//}

const terminalBufferSize = 1024 * 10

// runCmdAndCapture runs a command in the background
// and capture its output bytes (combining stdout and stderr) until EOF(or other error)
// listen to done channel to wait the command finish.
// Note: The terminalBytes slice has no lock, DO NOT write to it in other goroutine.
func runCmdAndCapture(cmd *exec.Cmd) (terminalBytes *[]byte, done chan error) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	outputReader := io.MultiReader(stdout, stderr)
	err := cmd.Start()
	if err != nil {
		logError("cmd.Start() err", err)
	}

	done = make(chan error, 0)
	go func() {
		done <- cmd.Wait()
	}()

	outputBytes := make([]byte, terminalBufferSize)
	go func() {
		// read output
		var buffer = make([]byte, terminalBufferSize)
		for {
			n, readErr := outputReader.Read(buffer)
			if n > 0 {
				outputBytes = append(outputBytes, buffer[:n]...)
			}
			if readErr != nil {
				if readErr != io.EOF {
					// ignore err
					logWarn("terminal output read err:", readErr)
				}
				break
			}
		}
	}()

	terminalBytes = &outputBytes // pass pointer, the underlying array may be changed.
	return
}

// formatTerminal removes \b and \r characters,
// and return a string that just like what you see in a terminal.
func formatTerminal(input string) string {
	var inputRunes, outputRunes []rune
	inputRunes = []rune(input)
	maxLength := 0
	for i := range inputRunes {
		if inputRunes[i] == '\b' {
			outputRunes = outputRunes[:len(outputRunes)-1]
		} else if inputRunes[i] == '\r' {
			for index := range outputRunes {
				if outputRunes[len(outputRunes)-1-index] == '\n' {
					if maxLength < len(outputRunes) {
						maxLength = len(outputRunes)
					}
					outputRunes = outputRunes[:len(outputRunes)-index]
					break
				}
			}
			// When there is no \n
			if maxLength < len(outputRunes) {
				maxLength = len(outputRunes)
			}
			outputRunes = outputRunes[:0]
		} else {
			outputRunes = append(outputRunes, inputRunes[i])
		}
	}

	//return string(outputRunes)
	if maxLength > len(outputRunes) {
		return string(outputRunes[:maxLength])
	} else {
		return string(outputRunes)
	}
}
