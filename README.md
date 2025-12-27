# M5Stack UDP Terminal

TTY Terminal with display for M5Stack devices (M5 Core and Cardputer) that receives text via UDP and displays it on the screen with ANSI code support.

## Features

- **TTY Display** with automatic scrolling
- **ANSI Codes** for colors, blink, inverse, clear screen
- **SD Card Configuration** (fallback to default values)
- **Key Sending** via UDP broadcast (keyboard/buttons)
- **Screensaver** automatic after 3 min of inactivity
- **Audio Beep** with BEL character

---

## Configuration

### Device Selection

At the top of `cardputer.ino`:

```cpp
#define CARDPUTER    // For M5Stack Cardputer
// #define CARDPUTER  // Comment out for M5Stack Core
```

### Configuration File (SD Card)

Create `/config.txt` on the SD Card:

```
WIFI_SSID=your_network
WIFI_PASSWORD=your_password
LOCAL_IP=192.168.0.14
GATEWAY=192.168.0.1
SUBNET=255.255.255.0
PRIMARY_DNS=8.8.8.8
SECONDARY_DNS=8.8.4.4
UDP_PORT=8888
BROADCAST_PORT=8889
BROADCAST_IP=192.168.0.255
```

If the SD card is missing or the file doesn't exist, code defaults will be used.

### mDNS Discovery

The device registers its hostname via mDNS. You can access it using:

- **Cardputer**: `cardputer.local`
- **M5 Core**: `m5core.local`

```bash
echo "Hello" | nc -w1 -u cardputer.local 8888
```

---

## Supported ANSI Codes

### Text Colors (30-37)

| Code | Color |
|------|-------|
| `ESC[30m` | Black |
| `ESC[31m` | Red |
| `ESC[32m` | Green |
| `ESC[33m` | Yellow |
| `ESC[34m` | Blue |
| `ESC[35m` | Magenta |
| `ESC[36m` | Cyan |
| `ESC[37m` | White |

### Background Colors (40-47)

| Code | Color |
|------|-------|
| `ESC[40m` | Black |
| `ESC[41m` | Red |
| `ESC[42m` | Green |
| `ESC[43m` | Yellow |
| `ESC[44m` | Blue |
| `ESC[45m` | Magenta |
| `ESC[46m` | Cyan |
| `ESC[47m` | White |

### Display Modes

| Code | Effect |
|------|--------|
| `ESC[0m` | Reset (default colors and modes) |
| `ESC[5m` | Blink ON |
| `ESC[25m` | Blink OFF |
| `ESC[7m` | Inverse ON |
| `ESC[27m` | Inverse OFF |

### Controls

| Code | Effect |
|------|--------|
| `ESC[2J` | Clear screen |
| `ESC[K` | Clear current line |
| `ESC[?1h` | Echo ON |
| `ESC[?1l` | Echo OFF |
| `\a` or `\x07` | Beep |

---

## Usage Examples

### Send Simple Text

```bash
echo "Hello World" | nc -w1 -u 192.168.0.14 8888
```

### Colored Text

```bash
# Red
printf '\033[31mRed Text\n' | nc -w1 -u 192.168.0.14 8888

# Green on Blue background
printf '\033[32;44mGreen on Blue\n' | nc -w1 -u 192.168.0.14 8888

# Reset
printf '\033[0mNormal\n' | nc -w1 -u 192.168.0.14 8888
```

### Blinking Text

```bash
printf '\033[5mALERT!\n' | nc -w1 -u 192.168.0.14 8888
printf '\033[25mNormal\n' | nc -w1 -u 192.168.0.14 8888
```

### Inverted Text

```bash
# Inverse ON
printf '\033[7mHighlight\n' | nc -w1 -u 192.168.0.14 8888

# Inverse OFF
printf '\033[27mNormal\n' | nc -w1 -u 192.168.0.14 8888
```

### Clear Screen

```bash
printf '\033[2J' | nc -w1 -u 192.168.0.14 8888
```

### Clear Current Line

```bash
printf '\033[K' | nc -w1 -u 192.168.0.14 8888
```

### Beep

```bash
printf '\a' | nc -w1 -u 192.168.0.14 8888
```

### Progress Bar (using \r)

```bash
#!/bin/sh
for i in $(seq 0 10 100); do
  printf "\rProgress: %3d%%" "$i" | nc -w1 -u 192.168.0.14 8888
  sleep 0.5
done
printf "\n" | nc -w1 -u 192.168.0.14 8888
```

---

## Receiving Keys (Buttons/Keyboard)

The device sends key events via UDP broadcast on port 8889.

### Protocol Format

```
KEY:<key>:<event>
```

Examples: `KEY:A:press`, `KEY:ENTER:press`, `KEY:a:press`

### Listen for Events (Python)

```python
#!/usr/bin/env python3
import socket

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(('0.0.0.0', 8889))
print("Listening for keys on port 8889...")

while True:
    data, addr = sock.recvfrom(1024)
    print(f"{addr}: {data.decode()}")
```

### Cardputer Keys

- All alphanumeric keys
- `ENTER`, `DEL`, `TAB`, `SPACE`
- `FN`, `OPT`, `ALT`, `CTRL`

### M5 Core Buttons

- `A`, `B`, `C` (press and release)

---

## Display Settings

| Device | Width | Height | Lines | Chars/Line |
|--------|-------|--------|-------|------------|
| Cardputer | 240px | 135px | 7 | 20 |
| M5 Core | 320px | 240px | 13 | 26 |

---

## Screensaver

- **Activates after**: 3 minutes of inactivity
- **Animation**: Internet Time bouncing (DVD style)
- **Deactivates**: Any UDP packet or key press
- **Screen off**: After 5 minutes in screensaver, display turns off completely to save power

To change timeouts, edit:

```cpp
#define SCREENSAVER_TIMEOUT 180000  // ms (180000 = 3 min)
#define SCREEN_OFF_TIMEOUT 300000   // ms (300000 = 5 min after screensaver)
```

---

## Status Bar

The first line is a blue status bar with:

- **Left**: Internet Time (beats) - updates every 1 second
- **Right**: Title (default: "monitor")

### Internet Time (Beats)

Internet Time divides the day into 1000 "beats":

- 1 beat = 1 minute and 26.4 seconds
- @000 = local midnight
- @500 = local noon
- Synced via NTP (timezone -3, Sao Paulo)

### Change Title

Use ANSI OSC (Operating System Command) code:

```bash
printf '\033]0;server\007' | nc -w1 -u 192.168.0.14 8888
```

**Note**: `\007` (BEL) is the **terminator** for OSC sequence, not a beep.

### Change Status Bar Colors

Use OSC 1 with format `bg;fg;title`:

```bash
# Red background (1), white text (7), title "ALERT"
printf '\033]1;1;7;ALERT\007' | nc -w1 -u 192.168.0.14 8888

# Green background (2), black text (0), title "OK"
printf '\033]1;2;0;OK\007' | nc -w1 -u 192.168.0.14 8888

# Yellow background (3), black text (0), title "WARNING"
printf '\033]1;3;0;WARNING\007' | nc -w1 -u 192.168.0.14 8888

# Reset to default: blue (4), white (7)
printf '\033]1;4;7;monitor\007' | nc -w1 -u 192.168.0.14 8888
```

**ANSI Color Indices (0-7):**

| Index | Color |
|-------|-------|
| 0 | Black |
| 1 | Red |
| 2 | Green |
| 3 | Yellow |
| 4 | Blue |
| 5 | Magenta |
| 6 | Cyan |
| 7 | White |

> [!WARNING]
> **Only ASCII characters are supported.** The display does not support UTF-8 (chars like ç, ã, é will not be displayed correctly). Use only A-Z, a-z, 0-9 and basic symbols.

---

## Go Client (ctrlmsg)

A command-line tool for sending messages and receiving key events, ideal for bash scripting.

### Build

```bash
cd ctrlmsg && go build
```

### Usage Examples

```bash
# Send message
./ctrlmsg -host cardputer.local "Hello World"

# Colored message
./ctrlmsg -fg red "Error!"
./ctrlmsg -fg white -bg blue "Info"

# Wait for any key
./ctrlmsg -wait "Press any key to continue"

# Wait for specific key (exit code = position in list)
./ctrlmsg -wait -key Y,N "Confirm? [Y/N]"
if [ $? -eq 0 ]; then echo "Yes"; else echo "No"; fi

# Change status bar: red bg, white text, title "ALERT"
./ctrlmsg -bar "1;7;ALERT"

# Clear screen and show message
./ctrlmsg -clear "Fresh start"

# Listen for key events (continuous)
./ctrlmsg -listen

# With timeout
./ctrlmsg -wait -timeout 10s "Press key within 10s"
```

### All Options

| Flag | Description | Default |
|------|-------------|---------|
| `-host` | Device hostname or IP | `cardputer.local` |
| `-port` | UDP send port | `8888` |
| `-lport` | UDP listen port | `8889` |
| `-wait` | Wait for key press | `false` |
| `-key` | Keys to wait for (comma-separated) | all |
| `-listen` | Listen mode | `false` |
| `-timeout` | Wait timeout (0=forever) | `0` |
| `-clear` | Clear screen first | `false` |
| `-bar` | Set status bar (`bg;fg;title`) | - |
| `-title` | Set title only | - |
| `-fg` | Text color | - |
| `-bg` | Background color | - |
| `-n` | No newline | `false` |
| `-beep` | Play beep | `false` |

---

## Required Libraries

- M5Unified (for M5 Core)
- M5Cardputer (for Cardputer)
- M5GFX
- WiFi
- SD

---

## License

MIT
