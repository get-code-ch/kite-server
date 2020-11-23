package main

import (
	"os"
	"time"
)

func main() {
	channel := make(chan bool)

	configFile := ""
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}
	_ = configFile

	go func() {
		time.Sleep(5 * time.Second)
		channel <- false
	}()

	<-channel
}
