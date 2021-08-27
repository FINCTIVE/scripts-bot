package main

import "log"

func logVerbose(v ...interface{}) {
	logWithPrefix("[verbose]", v...)
}

func logWarn(v ...interface{}) {
	logWithPrefix("[warn]", v...)
}

func logError(v ...interface{}) {
	logWithPrefix("[error]", v...)
}

func logWithPrefix(prefix string, v ...interface{}) {
	var args []interface{}
	args = append(args, prefix)
	args = append(args, v...)
	log.Println(args...)
}
