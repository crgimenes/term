
all:
	GOOS=js GOARCH=wasm go build -o term.wasm cmd/wasm/main.go
	go build -o term cmd/term/main.go
	#cp $(shell go env GOROOT)/misc/wasm/wasm_exec.* .
