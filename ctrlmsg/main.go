// ctrlmsg - UDP client for M5Stack devices (Cardputer/M5 Core)
// Sends messages and receives key events via UDP.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// ANSI color codes (foreground 30-37, background 40-47)
var colorCodes = map[string]int{
	"black":   0,
	"red":     1,
	"green":   2,
	"yellow":  3,
	"blue":    4,
	"magenta": 5,
	"cyan":    6,
	"white":   7,
}

// Exit codes
const (
	exitSuccess = 0
	exitError   = 1
	exitTimeout = 124
)

func main() {
	// CLI flags
	host := flag.String("host", "cardputer.local", "Device hostname or IP")
	port := flag.Int("port", 8888, "UDP send port")
	lport := flag.Int("lport", 8889, "UDP listen port for key events")
	wait := flag.Bool("wait", false, "Wait for key press after sending")
	keys := flag.String("key", "", "Specific keys to wait for (comma-separated)")
	listen := flag.Bool("listen", false, "Listen mode (print key events)")
	timeout := flag.Duration("timeout", 0, "Timeout for waiting (0=forever)")
	clear := flag.Bool("clear", false, "Clear screen before message")
	bar := flag.String("bar", "", "Set status bar (format: bg;fg;title)")
	fg := flag.String("fg", "", "Text foreground color")
	bg := flag.String("bg", "", "Text background color")
	noNewline := flag.Bool("n", false, "No newline at end of message")
	title := flag.String("title", "", "Set status bar title only")
	beep := flag.Bool("beep", false, "Play beep sound")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [message]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nColors: black, red, green, yellow, blue, magenta, cyan, white\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -host cardputer.local \"Hello World\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -wait \"Press any key\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -wait -key Y,N \"Confirm? [Y/N]\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -fg red \"Error!\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -bar \"1;7;ALERT\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -listen\n", os.Args[0])
	}

	flag.Parse()

	// Resolve hostname to IP
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", *host, *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving host: %v\n", err)
		os.Exit(exitError)
	}

	// Create UDP connection for sending
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting: %v\n", err)
		os.Exit(exitError)
	}
	defer conn.Close()

	// Listen mode - just print key events
	if *listen {
		listenForKeys(*lport)
		return
	}

	// Build message
	var msg strings.Builder

	// Clear screen first if requested
	if *clear {
		msg.WriteString("\033[2J")
	}

	// Set status bar if requested
	if *bar != "" {
		msg.WriteString(fmt.Sprintf("\033]1;%s\007", *bar))
	}

	// Set title only if requested
	if *title != "" {
		msg.WriteString(fmt.Sprintf("\033]0;%s\007", *title))
	}

	// Apply colors
	if *fg != "" || *bg != "" {
		msg.WriteString("\033[")
		codes := []string{}
		if *fg != "" {
			if code, ok := colorCodes[strings.ToLower(*fg)]; ok {
				codes = append(codes, fmt.Sprintf("%d", 30+code))
			}
		}
		if *bg != "" {
			if code, ok := colorCodes[strings.ToLower(*bg)]; ok {
				codes = append(codes, fmt.Sprintf("%d", 40+code))
			}
		}
		msg.WriteString(strings.Join(codes, ";"))
		msg.WriteString("m")
	}

	// Add message text
	if flag.NArg() > 0 {
		msg.WriteString(strings.Join(flag.Args(), " "))
	}

	// Reset colors after message
	if *fg != "" || *bg != "" {
		msg.WriteString("\033[0m")
	}

	// Add newline unless -n flag
	if !*noNewline && flag.NArg() > 0 {
		msg.WriteString("\n")
	}

	// Add beep if requested
	if *beep {
		msg.WriteString("\007")
	}

	// Send message if there's anything to send
	if msg.Len() > 0 {
		_, err = conn.Write([]byte(msg.String()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending: %v\n", err)
			os.Exit(exitError)
		}
	}

	// Wait for key if requested
	if *wait {
		exitCode := waitForKey(*lport, *keys, *timeout)
		os.Exit(exitCode)
	}
}

// listenForKeys continuously listens for key events and prints them
func listenForKeys(port int) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving address: %v\n", err)
		os.Exit(exitError)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening: %v\n", err)
		os.Exit(exitError)
	}
	defer conn.Close()

	fmt.Fprintf(os.Stderr, "Listening for key events on port %d...\n", port)

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
			continue
		}

		msg := string(buf[:n])
		// Parse KEY:X:event format
		if strings.HasPrefix(msg, "KEY:") {
			parts := strings.Split(msg, ":")
			if len(parts) >= 3 {
				key := parts[1]
				event := parts[2]
				fmt.Printf("%s %s %s\n", remoteAddr.IP, key, event)
			}
		}
	}
}

// waitForKey waits for a key press and returns exit code
func waitForKey(port int, allowedKeys string, timeout time.Duration) int {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving address: %v\n", err)
		return exitError
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening: %v\n", err)
		return exitError
	}
	defer conn.Close()

	// Parse allowed keys
	var keyList []string
	if allowedKeys != "" {
		keyList = strings.Split(strings.ToUpper(allowedKeys), ",")
		for i := range keyList {
			keyList[i] = strings.TrimSpace(keyList[i])
		}
	}

	// Set timeout if specified
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}

	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return exitTimeout
			}
			fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
			return exitError
		}

		msg := string(buf[:n])
		// Parse KEY:X:event format, only react to press events
		if strings.HasPrefix(msg, "KEY:") {
			parts := strings.Split(msg, ":")
			if len(parts) >= 3 && parts[2] == "press" {
				key := strings.ToUpper(parts[1])

				// If no key filter, accept any key
				if len(keyList) == 0 {
					fmt.Println(key)
					return exitSuccess
				}

				// Check if key is in allowed list
				for i, allowed := range keyList {
					if key == allowed {
						fmt.Println(key)
						return i // Return position as exit code
					}
				}
				// Key not in list, keep waiting
			}
		}
	}
}
