package main

import (
	"syscall/js"

	Console "github.com/crgimenes/term/console"
)

func main() {

	ct := Console.New()

	ws := js.Global().Get("WebSocket").New("ws://localhost:8080/ws")

	ws.Call("addEventListener", "open", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		ws.Call("send", "this is a test")
		return nil
	}))

	ws.Call("addEventListener", "error", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return nil
	}))

	ws.Call("addEventListener", "message", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		message := args[0].Get("data").String()
		ct.Write([]byte(message))
		return nil
	}))

	ws.Call("addEventListener", "close", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return nil
	}))
	ct.Run()

}
