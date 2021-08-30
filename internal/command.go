package internal

import (
	"errors"
	"io"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
)

const terminalBufferSize = 1024 * 10

// StartCmd runs a command in the background
// and capture its output bytes (combining stdout and stderr) until EOF(or other error)
// listen to done channel to wait the command finish.
// Note: DO NOT write to terminalBytes in other goroutine.
func StartCmd(cmd *exec.Cmd) (terminalBytes *[]byte, done chan error) {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	outputReader := io.MultiReader(stdout, stderr)

	err := cmd.Start()
	if err != nil {
		LogError("cmd.Start() err: ", err)
	}

	var bytes []byte
	done = make(chan error, 0)
	go func() {
		var buffer = make([]byte, terminalBufferSize)
		for {
			n, readErr := outputReader.Read(buffer)
			if n > 0 {
				bytes = append(bytes, buffer[:n]...)
			}
			// I/O finished
			if readErr != nil {
				if readErr != io.EOF {
					// ignore error
					LogWarn("terminal output read err:", readErr)
				}
				done <- cmd.Wait() // Wait() will close the pipe.
				return
			}
		}
	}()
	// passing pointer; the underlying array of slice may be changed.
	return &bytes, done
}

// FormatTerminal removes \b and \r characters,
// and return a string that just like what you see in a terminal.
// This function is useful for progress bar in terminal.
func FormatTerminal(input string) string {
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

	var resultStr string
	if maxLength > len(outputRunes) {
		resultStr = string(outputRunes[:maxLength])
	} else {
		resultStr = string(outputRunes)
	}
	// remove terminal color
	return StripColorANSI(resultStr)
}

// CheckBashSyntax ...
func CheckBashSyntax(filepath string) error {
	// check syntax error without running it
	cmd := exec.Command("bash", "-n", filepath)
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	errBytes, err := ioutil.ReadAll(errPipe)
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return errors.New(strings.Trim(string(errBytes), "\n "))
	}
	return nil
}

const colorANSI = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

func StripColorANSI(str string) string {
	return regexp.MustCompile(colorANSI).ReplaceAllString(str, "")
}
