
all:
	GOOS=js GOARCH=wasm go build -o term.wasm main.go
	#cp $(shell go env GOROOT)/misc/wasm/wasm_exec.* .
