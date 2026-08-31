package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	exit := flag.Bool("exit", false, "exit immediately")
	flag.Parse()
	if *exit {
		return
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
