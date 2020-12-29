package main

import (
	Console "github.com/crgimenes/term/console"
)

func main() {

	ct := Console.New()
	ct.Write([]byte("it is a test... 123..."))
	ct.Run()

}
