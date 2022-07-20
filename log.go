package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

var LoggingInfo struct {
	file *os.File
}

func set_log_file(path string) {
	LoggingInfo.file, _ = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	wrt := io.MultiWriter(os.Stdout, LoggingInfo.file)
	log.SetOutput(wrt)
}

func close_log_file() {
	LoggingInfo.file.Close()
}

func log_error(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func log_status(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func log_info(section string, format string, a ...any) {
	//TODO: add option to enable log spam
	//log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func log_panic(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}
