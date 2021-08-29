package internal

import "log"

// LogVerbose ...
func LogVerbose(v ...interface{}) {
	logWithPrefix("[verbose]", v...)
}

// LogWarn ...
func LogWarn(v ...interface{}) {
	logWithPrefix("[warn]", v...)
}

// LogError ...
func LogError(v ...interface{}) {
	logWithPrefix("[error]", v...)
}

func logWithPrefix(prefix string, v ...interface{}) {
	var args []interface{}
	args = append(args, prefix)
	args = append(args, v...)
	log.Println(args...)
}
