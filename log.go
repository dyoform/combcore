package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func SetLogFile(file string) {
	f, _ := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	wrt := io.MultiWriter(os.Stdout, f)
	log.SetOutput(wrt)
}

func LogError(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func LogStatus(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func LogInfo(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}

func LogPanic(section string, format string, a ...any) {
	log.Printf(fmt.Sprintf("(%s) %s", section, format), a...)
}
