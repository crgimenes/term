package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := "ws://localhost:8080/ws"

	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err) {
				return
			}
			log.Println("read:", err)
			return
		}
		os.Stdout.Write(message)
	}

}
