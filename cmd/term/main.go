package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	Console "github.com/crgimenes/term/console"
	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"golang.org/x/term"
)

var ct = Console.New()

var (
	addr     = flag.String("addr", "localhost:8080", "http service address")
	upgrader = websocket.Upgrader{} // use default options
)

type out struct {
}

var ansiMap = make(map[string]int)
var ansiAux string
var ansiRec bool

func isLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func (o out) Write(p []byte) (n int, err error) {
	n = len(p)

	ct.Write(p)
	fmt.Print(string(p))

	return n, nil
}

var o = out{}

func runCmd() error {
	// Create arbitrary command.
	c := exec.Command(os.Args[1], os.Args[2:]...)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s\r\n", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(o, ptmx)

	return nil
}

func main() {

	go func() {
		err := runCmd()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	ct.Run()

}
