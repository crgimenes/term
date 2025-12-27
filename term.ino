#define CARDPUTER // Uncomment for Cardputer, comment for M5 Core

#include <ESPmDNS.h>
#include <FS.h>
#include <SD.h>
#include <WiFi.h>
#include <WiFiUDP.h>

#ifdef CARDPUTER
#include <M5Cardputer.h>
#else
#include <M5Unified.h>
#endif

// ============== DEFAULT SETTINGS (fallback if SD fails) ==============
String cfgWifiSsid = "<SSID>";
String cfgWifiPassword = "<PASSWORD>";
String cfgLocalIP = "192.168.0.14";
String cfgGateway = "192.168.0.1";
String cfgSubnet = "255.255.255.0";
String cfgPrimaryDNS = "8.8.8.8";
String cfgSecondaryDNS = "8.8.4.4";
int cfgUdpPort = 8888;
int cfgBroadcastPort = 8889;
String cfgBroadcastIP = "192.168.0.255";

// NTP and Internet Time
#define TIMEZONE_OFFSET -3 // Sao Paulo, Brazil
String statusTitle = "monitor";
uint16_t statusBarBg = TFT_BLUE;  // Status bar background color
uint16_t statusBarFg = TFT_WHITE; // Status bar text color
// ===========================================================================

IPAddress localIP;
IPAddress gateway;
IPAddress subnet;
IPAddress primaryDNS;
IPAddress secondaryDNS;
IPAddress broadcastIP;

#ifdef CARDPUTER
#define NAME "cardputer"
#define MAX_VISIBLE_LINES 7
#define SCREEN_WIDTH 240
#define SCREEN_HEIGHT 135
#define CHARS_PER_LINE 20 // 240px / 12px per character with textSize(2)
#else
#define NAME "m5core"
#define MAX_VISIBLE_LINES 13
#define SCREEN_WIDTH 320
#define SCREEN_HEIGHT 240
#define CHARS_PER_LINE 26 // 320px / 12px per character with textSize(2)
#endif

// Device abstraction so the same code runs on Cardputer and M5 Core.
#ifdef CARDPUTER
#define DISPLAY M5Cardputer.Display
#define DEVICE M5Cardputer
// Colors calibrated for Cardputer (yellowish display)
#define MY_WHITE 0xF79E // White less red
#define MY_BLACK TFT_BLACK
#else
#define DISPLAY M5.Display
#define DEVICE M5
// Colors calibrated for M5 Core (bluish display)
#define MY_WHITE 0xFFDF // White less blue
#define MY_BLACK TFT_BLACK
#endif

// UDP Configuration
WiFiUDP udp;
char packetBuffer[255];

// TTY Terminal Configuration
#define MAX_LINES 50     // Total line buffer
#define LINE_TEXT_CAP 64 // Max characters stored per line (including NUL)
#define LINE_HEIGHT 18   // Pixels per line with textSize(2)

// Echo control (default off)
bool echoEnabled = false;

// ANSI to M5GFX color mapping
const uint16_t ansiColors[8] = {
    TFT_BLACK,   // 0 - Black
    TFT_RED,     // 1 - Red
    TFT_GREEN,   // 2 - Green
    TFT_YELLOW,  // 3 - Yellow
    TFT_BLUE,    // 4 - Blue
    TFT_MAGENTA, // 5 - Magenta
    TFT_CYAN,    // 6 - Cyan
    MY_WHITE     // 7 - White
};

// Current colors (can be changed by ANSI codes)
uint16_t currentTextColor = MY_WHITE;
uint16_t currentBgColor = TFT_BLACK;

// Rendering modes set by ANSI SGR sequences
bool blinkMode = false;   // ESC[5m / ESC[25m
bool inverseMode = false; // ESC[7m / ESC[27m
unsigned long lastBlinkTime = 0;
bool blinkVisible = true;
uint8_t blinkingLineCount = 0;

// Screensaver
unsigned long lastActivityTime = 0;
bool screensaverActive = false;
bool displayEnabled = true; // Global flag to enable/disable all screen drawing
unsigned long screensaverStartTime = 0;
#define SCREENSAVER_TIMEOUT 180000 // 3 minutes in ms
#define SCREEN_OFF_TIMEOUT 300000  // 5 minutes after screensaver starts
int ssX = 0, ssY = 0;              // Screensaver position
int ssDx = 2, ssDy = 1;            // Movement direction
unsigned long lastSSUpdate = 0;

// Status bar update timer
unsigned long lastStatusUpdate = 0;
#define STATUS_UPDATE_INTERVAL 1000 // Update every 1 second

// ANSI escape sequence parser
enum EscapeState
{
  ESC_NONE,      // Normal text mode
  ESC_START,     // Received ESC (0x1B)
  ESC_BRACKET,   // Received ESC[
  ESC_PARAMS,    // Reading parameters
  ESC_OSC,       // Received ESC] (Operating System Command)
  ESC_OSC_PARAMS // Reading OSC parameters
};

EscapeState escState = ESC_NONE;
String escParams = ""; // Buffer for escape sequence parameters
String oscBuffer = ""; // Buffer for OSC (title) data
int oscCommand = 0;    // OSC Command (0=title, 1=colors+title)

// Text Line structure (stores text with its colors and modes)
struct TextLine
{
  char text[LINE_TEXT_CAP];
  uint16_t textColor;
  uint16_t bgColor;
  bool blink;
  bool inverse;
};

// Text Buffer
TextLine lineBuffer[MAX_LINES];
int headIndex = 0;        // Ring buffer head (oldest line)
int lineCount = 0;        // Number of valid lines in buffer
int displayStartLine = 0; // First visible line on display (logical index)
String currentLine = "";  // Line being built

static inline int mapLogicalToSlot(int logicalIndex)
{
  // logicalIndex: 0..lineCount-1, where 0 is oldest.
  int idx = headIndex + logicalIndex;
  if (idx >= MAX_LINES)
    idx -= MAX_LINES;
  return idx;
}

static inline int clampInt(int v, int lo, int hi)
{
  if (v < lo)
    return lo;
  if (v > hi)
    return hi;
  return v;
}

// Function declarations
void addLine(const String &text, bool render = true);
void renderDisplay();
void processChar(char c);
void processEscapeSequence(char command, const String &params);
void clearScreen();
void sendKeyEvent(const String &key, const String &event);
void checkInput();
bool loadConfig();
void initConfigFromStrings();
void runScreensaver();
void setupNTP();
float getInternetTime();
float getCachedInternetTime();
void renderStatusBar();

// Converts String to IPAddress
IPAddress parseIPAddress(const String &ipStr)
{
  int parts[4];
  int partIndex = 0;
  int lastDot = -1;

  for (int i = 0; i <= ipStr.length() && partIndex < 4; i++)
  {
    if (i == ipStr.length() || ipStr.charAt(i) == '.')
    {
      parts[partIndex++] = ipStr.substring(lastDot + 1, i).toInt();
      lastDot = i;
    }
  }

  return IPAddress(parts[0], parts[1], parts[2], parts[3]);
}

// Applies configuration string values to IPAddress
void initConfigFromStrings()
{
  localIP = parseIPAddress(cfgLocalIP);
  gateway = parseIPAddress(cfgGateway);
  subnet = parseIPAddress(cfgSubnet);
  primaryDNS = parseIPAddress(cfgPrimaryDNS);
  secondaryDNS = parseIPAddress(cfgSecondaryDNS);
  broadcastIP = parseIPAddress(cfgBroadcastIP);

// Adjust IP based on device if still default
#ifdef CARDPUTER
  // Cardputer keeps .14
#else
  if (cfgLocalIP == "192.168.0.14")
  {
    cfgLocalIP = "192.168.0.13";
    localIP = parseIPAddress(cfgLocalIP);
  }
#endif
}

// Loads configuration from SD card
bool loadConfig()
{
  // Initialize SD card (pin varies by device)
#ifdef CARDPUTER
  if (!SD.begin())
  {
    return false; // SD card not found
  }
#else
  // M5 Core uses specific pin for SD
  if (!SD.begin(4, SPI, 25000000))
  {
    return false;
  }
#endif

  // Open configuration file
  File configFile = SD.open("/config.txt", FILE_READ);
  if (!configFile)
  {
    return false; // File not found
  }

  // Read line by line
  while (configFile.available())
  {
    String line = configFile.readStringUntil('\n');
    line.trim();

    // Ignore empty lines and comments
    if (line.length() == 0 || line.startsWith("#"))
    {
      continue;
    }

    // Find '='
    int eqPos = line.indexOf('=');
    if (eqPos == -1)
      continue;

    String key = line.substring(0, eqPos);
    String value = line.substring(eqPos + 1);
    key.trim();
    value.trim();

    // Apply values
    if (key == "WIFI_SSID")
      cfgWifiSsid = value;
    else if (key == "WIFI_PASSWORD")
      cfgWifiPassword = value;
    else if (key == "LOCAL_IP")
      cfgLocalIP = value;
    else if (key == "GATEWAY")
      cfgGateway = value;
    else if (key == "SUBNET")
      cfgSubnet = value;
    else if (key == "PRIMARY_DNS")
      cfgPrimaryDNS = value;
    else if (key == "SECONDARY_DNS")
      cfgSecondaryDNS = value;
    else if (key == "UDP_PORT")
      cfgUdpPort = value.toInt();
    else if (key == "BROADCAST_PORT")
      cfgBroadcastPort = value.toInt();
    else if (key == "BROADCAST_IP")
      cfgBroadcastIP = value;
  }

  configFile.close();
  return true;
}

void setup()
{
  // Reduce CPU frequency to save power (default is 240MHz)
  setCpuFrequencyMhz(80);

#ifdef CARDPUTER
  M5Cardputer.begin();
#else
  auto cfg = M5.config();
  M5.begin(cfg);
#endif

  DISPLAY.setEpdMode(m5gfx::epd_fastest);
  DISPLAY.setTextSize(2);
  DISPLAY.setTextColor(currentTextColor, currentBgColor);
  DISPLAY.clear();
  DISPLAY.setBrightness(128 / 3);

  // Pre-allocate frequently grown strings to reduce heap churn.
  currentLine.reserve(LINE_TEXT_CAP + 1);
  escParams.reserve(32);
  oscBuffer.reserve(96);
  statusTitle.reserve(32);

  // Initialize line buffer
  for (int i = 0; i < MAX_LINES; i++)
  {
    lineBuffer[i].text[0] = '\0';
    lineBuffer[i].textColor = MY_WHITE;
    lineBuffer[i].bgColor = TFT_BLACK;
    lineBuffer[i].blink = false;
    lineBuffer[i].inverse = false;
  }

  headIndex = 0;
  lineCount = 0;
  displayStartLine = 0;

  addLine(NAME);

  // Try to load SD card config
  if (loadConfig())
  {
    addLine("CFG: SD OK");
  }
  else
  {
    addLine("CFG: defaults");
  }

  // Apply configurations (convert strings to IPAddress)
  initConfigFromStrings();

  // Show configured IP
  addLine("IP:" + cfgLocalIP);

  if (!WiFi.config(localIP, gateway, subnet, primaryDNS, secondaryDNS))
  {
    addLine("Static IP config failed");
  }

  WiFi.begin(cfgWifiSsid.c_str(), cfgWifiPassword.c_str());
  String dots = "Connecting";
  dots.reserve(32);
  for (int i = 20; i && WiFi.status() != WL_CONNECTED; --i)
  {
    dots += ".";
    delay(500);
  }
  addLine(dots);

  if (WiFi.status() == WL_CONNECTED)
  {
    String sip = WiFi.localIP().toString();
    addLine("IP: " + sip);

    // Start mDNS
    if (MDNS.begin(NAME))
    {
      addLine("mDNS responder started");
      MDNS.addService("http", "tcp", 80);
    }
    else
    {
      addLine("mDNS responder failed");
    }

    // Start UDP server
    udp.begin(cfgUdpPort);
    addLine("UDP: " + String(cfgUdpPort));

    // Sync clock via NTP
    setupNTP();
    addLine("NTP syncing...");
  }
  else
  {
    addLine("WiFi connection failed");
  }

  // Initialize screensaver timer
  lastActivityTime = millis();
}

void loop()
{
#ifndef CARDPUTER
  // M5 Core: update button state
  M5.update();
#endif

  // Apply any local input events.
  checkInput();

  // Check if UDP packets are available
  int packetSize = udp.parsePacket();
  if (packetSize)
  {
    // Disable screensaver and reset timer
    wakeDisplay();
    lastActivityTime = millis();

    // Read the packet and apply all changes before repainting.
    int len = udp.read(packetBuffer, sizeof(packetBuffer));
    if (len > 0)
    {
      // Apply the incoming terminal byte stream.
      for (int i = 0; i < len; i++)
      {
        processChar(packetBuffer[i]);
      }

      // Paint once per packet to keep throughput high on bursts.
      renderDisplay();
    }
  }

  // Blink timer: toggle visibility every 500ms (only if something is blinking)
  if (millis() - lastBlinkTime > 500)
  {
    lastBlinkTime = millis();
    blinkVisible = !blinkVisible;

    // Only redraw when blink can affect what is shown.
    if (blinkMode || (blinkingLineCount > 0))
    {
      renderDisplay();
    }
  }

  // Screensaver: activate after inactivity timeout
  if (!screensaverActive &&
      (millis() - lastActivityTime > SCREENSAVER_TIMEOUT))
  {
    screensaverActive = true;
    screensaverStartTime = millis();
    DISPLAY.fillScreen(TFT_BLACK);
  }

  // Turn off screen after 5 minutes in screensaver to save power
  if (screensaverActive && displayEnabled &&
      (millis() - screensaverStartTime > SCREEN_OFF_TIMEOUT))
  {
    displayEnabled = false;
    DISPLAY.setBrightness(0); // Turn off backlight completely
  }

  // Run screensaver if active (but not if screen is off)
  if (screensaverActive && displayEnabled)
  {
    runScreensaver();
  }
  else
  {
    // Update status bar periodically when not in screensaver
    if (millis() - lastStatusUpdate > STATUS_UPDATE_INTERVAL)
    {
      lastStatusUpdate = millis();
      renderStatusBar();
    }
  }

  // Yield a tiny slice for background tasks (WiFi, timers) without hurting UX.
  delay(1);
}

// Send key event via UDP broadcast
void sendKeyEvent(const String &key, const String &event)
{
  udp.beginPacket(broadcastIP, cfgBroadcastPort);
  udp.print("KEY:");
  udp.print(key);
  udp.print(":");
  udp.print(event);
  udp.endPacket();

  // Local echo if enabled - process as received input
  if (echoEnabled)
  {
    if (key == "ENTER")
    {
      processChar('\n');
    }
    else if (key == "DEL")
    {
      // Backspace edits the current input line.
      if (currentLine.length() > 0)
      {
        currentLine.remove(currentLine.length() - 1);
      }
    }
    else if (key == "TAB")
    {
      // Tab as 2 spaces, using the same path as incoming text so wrapping stays
      // consistent.
      processChar(' ');
      processChar(' ');
    }
    else if (key.length() == 1)
    {
      // Normal single character key
      processChar(key.charAt(0));
    }
    // Echo is user-driven and low-volume; render immediately for
    // responsiveness.
    renderDisplay();
  }
}

// Wake display from screensaver or sleep mode
// Returns true if display was asleep and woke up
bool wakeDisplay()
{
  if (!screensaverActive)
    return false;

  // Wake up from screensaver
  screensaverActive = false;
  lastActivityTime = millis();

  // Restore screen if it was off
  if (!displayEnabled)
  {
    displayEnabled = true;
    DISPLAY.setBrightness(128 / 3); // Restore to normal brightness
  }

  // Reset colors to default
  currentTextColor = MY_WHITE;
  currentBgColor = TFT_BLACK;
  renderDisplay();

  return true; // Display was asleep, now awake
}

void checkInput()
{
#ifdef CARDPUTER
  // Cardputer: Keyboard - check for change
  M5Cardputer.update();
  if (M5Cardputer.Keyboard.isChange())
  {
    if (M5Cardputer.Keyboard.isPressed())
    {
      // If display is asleep, wake it and consume this keypress
      if (wakeDisplay())
        return;

      // Get pressed key directly
      String pressedKey = "";
      Keyboard_Class::KeysState status = M5Cardputer.Keyboard.keysState();

      // Check normal keys
      for (auto key : status.word)
      {
        if (key != 0)
        {
          pressedKey = String((char)key);
          sendKeyEvent(pressedKey, "press");
        }
      }

      // Special keys
      if (status.del)
        sendKeyEvent("DEL", "press");
      if (status.enter)
        sendKeyEvent("ENTER", "press");
      if (status.fn)
        sendKeyEvent("FN", "press");
      if (status.opt)
        sendKeyEvent("OPT", "press");
      if (status.alt)
        sendKeyEvent("ALT", "press");
      if (status.ctrl)
        sendKeyEvent("CTRL", "press");
      if (status.space)
        sendKeyEvent(" ", "press");
      if (status.tab)
        sendKeyEvent("TAB", "press");
    }
  }
#else
  // M5 Core: Buttons A, B, C
  bool anyPressed =
      M5.BtnA.wasPressed() || M5.BtnB.wasPressed() || M5.BtnC.wasPressed();

  // If display is asleep and any button pressed, wake it and return
  if (anyPressed && wakeDisplay())
    return;

  if (M5.BtnA.wasPressed())
    sendKeyEvent("A", "press");
  if (M5.BtnA.wasReleased())
    sendKeyEvent("A", "release");
  if (M5.BtnB.wasPressed())
    sendKeyEvent("B", "press");
  if (M5.BtnB.wasReleased())
    sendKeyEvent("B", "release");
  if (M5.BtnC.wasPressed())
    sendKeyEvent("C", "press");
  if (M5.BtnC.wasReleased())
    sendKeyEvent("C", "release");
#endif
}

// Process one character at a time with ANSI escape code support
void processChar(char c)
{
  // Handle ESC sequence state machine
  switch (escState)
  {
  case ESC_NONE:
    if (c == 0x1B)
    { // ESC character (27 decimal, 0x1B hex)
      escState = ESC_START;
      return;
    }
    // Normal character processing
    if (c == '\n')
    {
      // Line feed - always commit a line (serial-like), even if empty.
      addLine(currentLine, false); // Don't render yet
      currentLine = "";
    }
    else if (c == '\r')
    {
      // Carriage return - return to start of line (allows overwrite)
      // Don't add to buffer, just clear currentLine to start over
      currentLine = "";
    }
    else if (c == 0x07)
    {
      // BEL - Bell sound
      DEVICE.Speaker.tone(1000, 100); // 1kHz for 100ms
    }
    else if (c >= 32 && c <= 126)
    { // Printable characters
      const int wrapWidth = min(CHARS_PER_LINE, (int)LINE_TEXT_CAP - 1);
      currentLine += c;

      // If line reaches limit, wrap automatically (terminal-like continuation).
      if ((int)currentLine.length() >= wrapWidth)
      {
        addLine(currentLine, false); // Don't render yet
        currentLine = "";
      }
    }
    break;

  case ESC_START:
    if (c == '[')
    {
      escState = ESC_BRACKET;
      escParams = "";
    }
    else if (c == ']')
    {
      // OSC - Operating System Command (used for title)
      escState = ESC_OSC;
      oscBuffer = "";
    }
    else
    {
      escState = ESC_NONE; // Invalid sequence, reset
    }
    break;

  case ESC_OSC:
    // Waiting for OSC command number (0 = title, 1 = colors+title)
    if (c >= '0' && c <= '9')
    {
      oscBuffer += c;
    }
    else if (c == ';')
    {
      // Save command and start parameters
      oscCommand = oscBuffer.toInt();
      escState = ESC_OSC_PARAMS;
      oscBuffer = "";
    }
    else
    {
      escState = ESC_NONE;
    }
    break;

  case ESC_OSC_PARAMS:
    // Reading parameters until BEL (0x07) or ST (ESC \)
    if (c == 0x07 || c == 0x1B)
    {
      // Process based on OSC command
      if (oscCommand == 0)
      {
        // OSC 0: title only
        statusTitle = oscBuffer;
      }
      else if (oscCommand == 1)
      {
        // OSC 1: update status bar colors and title
        int bg = 0, fg = 0;
        int semiCount = 0;
        String title = "";
        title.reserve(32);
        for (int i = 0; i < oscBuffer.length(); i++)
        {
          char c = oscBuffer.charAt(i);
          if (c == ';')
          {
            semiCount++;
          }
          else if (semiCount < 2)
          {
            if (semiCount == 0 && c >= '0' && c <= '9')
            {
              bg = bg * 10 + (c - '0');
            }
            else if (semiCount == 1 && c >= '0' && c <= '9')
            {
              fg = fg * 10 + (c - '0');
            }
          }
          else
          {
            title += c;
          }
        }

        // Apply colors (ANSI index 0-7)
        if (bg >= 0 && bg <= 7)
          statusBarBg = ansiColors[bg];
        if (fg >= 0 && fg <= 7)
          statusBarFg = ansiColors[fg];
        if (title.length() > 0)
          statusTitle = title;
      }
      escState = ESC_NONE;
      oscBuffer = "";
    }
    else
    {
      oscBuffer += c;
    }
    break;

  case ESC_BRACKET:
  case ESC_PARAMS:
    if ((c >= '0' && c <= '9') || c == ';' || c == '?')
    {
      escParams += c;
      escState = ESC_PARAMS;
    }
    else if (c == 'm' || c == 'J' || c == 'K' || c == 'h' || c == 'l')
    {
      processEscapeSequence(c, escParams);
      escState = ESC_NONE;
      escParams = "";
    }
    else
    {
      escState = ESC_NONE; // Unknown sequence, ignore
    }
    break;
  }
}

// Process complete ANSI escape sequence
void processEscapeSequence(char command, const String &params)
{
  switch (command)
  {
  case 'J':
    if (params == "2")
    {
      // ESC[2J - Clear screen
      clearScreen();
    }
    break;

  case 'm':
    // SGR (Select Graphic Rendition) - Set colors

    // Flush pending text so style changes apply cleanly.
    if (currentLine.length() > 0)
    {
      addLine(currentLine, false);
      currentLine = "";
    }

    if (params == "" || params == "0")
    {
      // ESC[0m - Reset to defaults
      currentTextColor = MY_WHITE;
      currentBgColor = TFT_BLACK;
      blinkMode = false;
      inverseMode = false;
    }
    else
    {
      // Parse numeric SGR codes without allocating temporary substrings.
      const int n = params.length();
      int code = 0;
      for (int i = 0; i <= n; i++)
      {
        const bool atEnd = (i == n);
        const char ch = atEnd ? ';' : params.charAt(i);
        if (atEnd || ch == ';')
        {
          // Process code when we hit end or semicolon
          if (code >= 30 && code <= 37)
          {
            // Foreground color (30-37)
            currentTextColor = ansiColors[code - 30];
          }
          else if (code >= 40 && code <= 47)
          {
            // Background color (40-47)
            currentBgColor = ansiColors[code - 40];
          }
          else
          {
            switch (code)
            {
            case 5: // ESC[5m - Blink on
              blinkMode = true;
              break;
            case 25: // ESC[25m - Blink off
              blinkMode = false;
              break;
            case 7: // ESC[7m - Inverse on
              inverseMode = true;
              break;
            case 27: // ESC[27m - Inverse off
              inverseMode = false;
              break;
            }
          }
          code = 0;
        }
        else if (ch >= '0' && ch <= '9')
        {
          code = code * 10 + (ch - '0');
        }
      }
    }

    DISPLAY.setTextColor(currentTextColor, currentBgColor);
    break;

  case 'K':
    // ESC[K or ESC[2K - Clear current line
    currentLine = "";
    break;

  case 'h':
    if (params == "?1")
    {
      // ESC[?1h - Echo ON
      echoEnabled = true;
    }
    break;

  case 'l':
    if (params == "?1")
    {
      // ESC[?1l - Echo OFF
      echoEnabled = false;
    }
    break;
  }
}

// Clears screen and resets buffer
void clearScreen()
{
  // Reset terminal state and clear all visible content.
  for (int i = 0; i < MAX_LINES; i++)
  {
    lineBuffer[i].text[0] = '\0';
    lineBuffer[i].textColor = MY_WHITE;
    lineBuffer[i].bgColor = TFT_BLACK;
    lineBuffer[i].blink = false;
    lineBuffer[i].inverse = false;
  }
  headIndex = 0;
  lineCount = 0;
  displayStartLine = 0;
  currentLine = "";
  blinkingLineCount = 0;

  // Reset the frame to the current background.
  DISPLAY.fillScreen(currentBgColor);
}

// Adds a line to the buffer and optionally updates the display
void addLine(const String &text, bool render)
{
  const int wrapWidth = min(CHARS_PER_LINE, (int)LINE_TEXT_CAP - 1);

  // Snapshot styling so split chunks are consistent.
  const uint16_t snapTextColor = currentTextColor;
  const uint16_t snapBgColor = currentBgColor;
  const bool snapBlink = blinkMode;
  const bool snapInverse = inverseMode;

  // Split into terminal-width chunks (serial-like continuation).
  const int n = text.length();
  int pos = 0;
  do
  {
    const int chunkLen = (n == 0) ? 0 : min(wrapWidth, n - pos);

    int slot;
    if (lineCount < MAX_LINES)
    {
      slot = mapLogicalToSlot(lineCount);
      lineCount++;
    }
    else
    {
      // Overwrite the oldest line.
      slot = headIndex;
      if (lineBuffer[slot].blink && blinkingLineCount > 0)
      {
        blinkingLineCount--;
      }
      headIndex++;
      if (headIndex >= MAX_LINES)
        headIndex = 0;
    }

    // Copy text chunk safely into fixed buffer.
    if (chunkLen > 0)
    {
      const int copyLen = min(chunkLen, (int)LINE_TEXT_CAP - 1);
      memcpy(lineBuffer[slot].text, text.c_str() + pos, copyLen);
      lineBuffer[slot].text[copyLen] = '\0';
    }
    else
    {
      lineBuffer[slot].text[0] = '\0';
    }

    lineBuffer[slot].textColor = snapTextColor;
    lineBuffer[slot].bgColor = snapBgColor;
    lineBuffer[slot].blink = snapBlink;
    lineBuffer[slot].inverse = snapInverse;

    if (snapBlink && blinkingLineCount < MAX_LINES)
    {
      blinkingLineCount++;
    }

    pos += chunkLen;
  } while (pos < n);

  // Auto-scroll: always show the last content lines (status bar consumes 1).
  const int maxContentLines = MAX_VISIBLE_LINES - 1;
  const int virtualLineCount = lineCount; // currentLine is rendered as an extra
                                          // virtual line in renderDisplay().
  displayStartLine = max(0, virtualLineCount - maxContentLines);

  if (render)
  {
    renderDisplay();
  }
}

// Renders display with visible lines
void renderDisplay()
{
  if (!displayEnabled)
    return; // Skip all drawing when display is disabled
  DISPLAY.fillScreen(currentBgColor);

  // Renders status bar on first line
  renderStatusBar();

  // Content Y offset (below status bar)
  int contentStartY = LINE_HEIGHT;

  // Adjust visible lines (one less due to status bar)
  int maxContentLines = MAX_VISIBLE_LINES - 1;
  const bool currentLinePresent = (currentLine.length() > 0);
  const int virtualLineCount = lineCount + (currentLinePresent ? 1 : 0);
  const int maxStart = max(0, virtualLineCount - maxContentLines);
  // Terminal-like behavior: always follow the tail (including the in-progress
  // currentLine).
  displayStartLine = maxStart;

  const int endVirtual =
      min(virtualLineCount, displayStartLine + maxContentLines);

  for (int logical = displayStartLine; logical < endVirtual; logical++)
  {
    const int y = contentStartY + (logical - displayStartLine) * LINE_HEIGHT;
    if ((y + LINE_HEIGHT) > SCREEN_HEIGHT)
      break;

    if (logical < lineCount)
    {
      const int slot = mapLogicalToSlot(logical);

      // Apply inverse mode if active for this line
      const uint16_t fg = lineBuffer[slot].inverse ? lineBuffer[slot].bgColor
                                                   : lineBuffer[slot].textColor;
      const uint16_t bg = lineBuffer[slot].inverse ? lineBuffer[slot].textColor
                                                   : lineBuffer[slot].bgColor;

      DISPLAY.fillRect(0, y, SCREEN_WIDTH, LINE_HEIGHT, bg);
      DISPLAY.setCursor(0, y + 1);
      DISPLAY.setTextColor(fg, bg);

      if (!lineBuffer[slot].blink || blinkVisible)
      {
        DISPLAY.print(lineBuffer[slot].text);
      }
    }
    else
    {
      // currentLine as the last virtual line (never disappears when the screen
      // is full)
      const uint16_t fg = inverseMode ? currentBgColor : currentTextColor;
      const uint16_t bg = inverseMode ? currentTextColor : currentBgColor;

      DISPLAY.fillRect(0, y, SCREEN_WIDTH, LINE_HEIGHT, bg);
      DISPLAY.setCursor(0, y + 1);
      DISPLAY.setTextColor(fg, bg);

      if (!blinkMode || blinkVisible)
      {
        DISPLAY.print(currentLine);
      }
    }
  }
}

// Screensaver: Internet Time bouncing around screen
void runScreensaver()
{
  if (!displayEnabled)
    return; // Skip drawing when display is disabled
  // Update every 50ms
  if (millis() - lastSSUpdate < 50)
    return;
  lastSSUpdate = millis();

  // Internet Time text size: @XXX.XX = 7 chars (12px each)
  int textWidth = 7 * 12; // 84px
  int textHeight = 18;

  // Screensaver color (separate from terminal color)
  static uint16_t ssColor = TFT_GREEN;

  // Erase and redraw for the bouncing animation.
  DISPLAY.fillRect(ssX, ssY, textWidth, textHeight, TFT_BLACK);

  ssX += ssDx;
  ssY += ssDy;

  if (ssX <= 0 || ssX >= SCREEN_WIDTH - textWidth)
  {
    ssDx = -ssDx;
    ssX = constrain(ssX, 0, SCREEN_WIDTH - textWidth);
    ssColor = ansiColors[random(1, 8)];
  }
  if (ssY <= 0 || ssY >= SCREEN_HEIGHT - textHeight)
  {
    ssDy = -ssDy;
    ssY = constrain(ssY, 0, SCREEN_HEIGHT - textHeight);
    ssColor = ansiColors[random(1, 8)];
  }

  // Draw Internet Time at new position
  DISPLAY.setCursor(ssX, ssY);
  DISPLAY.setTextColor(ssColor, TFT_BLACK);

  // Show Internet Time on screensaver
  float beat = getCachedInternetTime();
  char beatStr[12];
  if (beat < 0)
  {
    sprintf(beatStr, "------");
  }
  else
  {
    sprintf(beatStr, "@%06.2f", beat);
  }
  DISPLAY.print(beatStr);
}

// Configure NTP to sync clock
void setupNTP()
{
  // Configure timezone and NTP server
  configTime(TIMEZONE_OFFSET * 3600, 0, "pool.ntp.org", "time.nist.gov");
}

// Calculate Internet Time (beats)
// 1 day = 1000 beats, based on local time
// Returns -1 if NTP not yet synced
float getInternetTime()
{
  struct tm timeinfo;

  // Get local time with 0 timeout (non-blocking)
  if (!getLocalTime(&timeinfo, 0))
  {
    return -1.0; // Returns -1 if NTP not yet synced
  }

  // Calculate total seconds since local midnight
  int totalSeconds =
      timeinfo.tm_hour * 3600 + timeinfo.tm_min * 60 + timeinfo.tm_sec;

  // Convert to beats (1 beat = 86.4 seconds)
  float beats = totalSeconds / 86.4;

  return beats;
}

// Cache time for UI rendering so we don't call into the time subsystem every
// frame.
float getCachedInternetTime()
{
  static unsigned long lastUpdate = 0;
  static float cachedBeat = -1.0;

  unsigned long now = millis();
  if (now - lastUpdate >= 1000)
  {
    cachedBeat = getInternetTime();
    lastUpdate = now;
  }

  return cachedBeat;
}

// Render status bar on first line
void renderStatusBar()
{
  if (!displayEnabled)
    return; // Skip drawing when display is disabled
  // Fill status bar background
  DISPLAY.fillRect(0, 0, SCREEN_WIDTH, LINE_HEIGHT, statusBarBg);

  // Internet Time on left
  float beat = getCachedInternetTime();
  char beatStr[12];
  if (beat < 0)
  {
    sprintf(beatStr, "------"); // NTP not yet synced
  }
  else
  {
    sprintf(beatStr, "@%06.2f", beat);
  }

  DISPLAY.setCursor(2, 1);
  DISPLAY.setTextColor(statusBarFg, statusBarBg);
  DISPLAY.print(beatStr);

  // Title on right
  int titleWidth = statusTitle.length() * 12; // 12px per character
  int titleX = SCREEN_WIDTH - titleWidth - 2;
  DISPLAY.setCursor(titleX, 1);
  DISPLAY.print(statusTitle);
}
