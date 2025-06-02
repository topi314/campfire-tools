package main

import (
	_ "embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/topi314/campfire-raffle/server"
)

var address = os.Getenv("SERVER_ADDRESS")

func main() {
	if address == "" {
		address = ":8085"
	}
	srv := server.New(address)
	go srv.Start()
	defer srv.Stop()

	log.Printf("Server started at %s", address)

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM, syscall.SIGINT)
	<-s
}
