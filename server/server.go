package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"golang.org/x/term"
)

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

	// collect ansi code
	for _, v := range p {
		if v == '\x1b' {
			ansiAux = "ESC:"
			ansiRec = true
			continue
		}
		/*
			if ansiRec {
				ansiAux += string(v)
			}
		*/
		if ansiRec && isLetter(v) {
			ansiAux += string(v)
			x := ansiMap[ansiAux]
			x++
			ansiMap[ansiAux] = x
			ansiAux = ""
			ansiRec = false
		}
	}

	fmt.Print(string(p))
	if conn != nil {
		conn.WriteMessage(websocket.TextMessage, p)
	}
	return n, nil
}

var o = out{}
var conn *websocket.Conn

func handle(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
	}
	defer c.Close()

	conn = c
	/*
		go func() {
			for {
				<-time.After(2 * time.Second)
				c.WriteMessage(websocket.TextMessage, []byte("teste de mensagem periodica\r\n"))
			}
		}()
	*/
	for {
		//mt, message, err := c.ReadMessage()
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("read: %v\r\n", err)
			break
		}
		log.Printf("recv: %s\r\n", message)
		if string(message) == "quit" {
			c.Close()
			break
		}

	}
	log.Println("cliente desconectado")
}

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
		for k, v := range ansiMap {
			fmt.Printf("%q\t%v\r\n", k, v)
		}
		os.Exit(0)
	}()

	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/ws", handle)
	f := http.FileServer(http.Dir("./"))
	http.Handle("/", f)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
