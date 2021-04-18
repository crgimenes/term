package console

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten"
)

type color struct {
	R byte
	G byte
	B byte
}

type char struct {
	charID  int
	fgColor color
	bgColor color
	blink   bool
}

type Console struct {
	videoTextMemory [25 * 80]char
	fgColor         color
	bgColor         color

	auxCursorPos int
	cursor       int
	height       int
	width        int
	scale        float64

	cursorSetBlink   bool
	cursorBlinkTimer int

	tmpScreen *ebiten.Image
	img       *image.RGBA

	title string

	font struct {
		height int
		width  int
		bitmap []byte
	}
}

func (c *Console) Write(p []byte) (n int, err error) {
	c.Print(string(p))
	return len(p), nil
}

func colorParserFg(i int) (color, bool) {
	fg := make(map[int]color, 16)

	fg[30] = color{0, 0, 0}       // Black
	fg[31] = color{170, 0, 0}     // Red
	fg[32] = color{0, 170, 0}     // Green
	fg[33] = color{170, 85, 0}    // Yellow
	fg[34] = color{0, 0, 170}     // Blue
	fg[35] = color{170, 0, 170}   // Magenta
	fg[36] = color{0, 170, 170}   // Cyan
	fg[37] = color{170, 170, 170} // White
	fg[90] = color{85, 85, 85}    // Bright Black (Gray)
	fg[91] = color{255, 85, 85}   // Bright Red
	fg[92] = color{85, 255, 85}   // Bright Green
	fg[93] = color{255, 255, 85}  // Bright Yellow
	fg[94] = color{85, 85, 255}   // Bright Blue
	fg[95] = color{255, 85, 255}  // Bright Magenta
	fg[96] = color{85, 255, 255}  // Bright Cyan
	fg[97] = color{255, 255, 255} // Bright White

	c, ok := fg[i]
	return c, ok
}

func colorParserBg(i int) (color, bool) {
	bg := make(map[int]color, 16)

	bg[40] = color{0, 0, 0}        // Black
	bg[41] = color{170, 0, 0}      // Red
	bg[42] = color{0, 170, 0}      // Green
	bg[43] = color{170, 85, 0}     // Yellow
	bg[44] = color{0, 0, 170}      // Blue
	bg[45] = color{170, 0, 170}    // Magenta
	bg[46] = color{0, 170, 170}    // Cyan
	bg[47] = color{170, 170, 170}  // White
	bg[100] = color{85, 85, 85}    // Bright Black (Gray)
	bg[101] = color{255, 85, 85}   // Bright Red
	bg[102] = color{85, 255, 85}   // Bright Green
	bg[103] = color{255, 255, 85}  // Bright Yellow
	bg[104] = color{85, 85, 255}   // Bright Blue
	bg[105] = color{255, 85, 255}  // Bright Magenta
	bg[106] = color{85, 255, 255}  // Bright Cyan
	bg[107] = color{255, 255, 255} // Bright White

	c, ok := bg[i]
	return c, ok
}

func New() *Console {
	c := &Console{}
	c.bgColor, _ = colorParserBg(40)
	c.fgColor, _ = colorParserFg(37)
	c.width = 80 * 9
	c.height = 25 * 16
	c.scale = 1.5
	c.title = "term"
	c.cursorSetBlink = true

	c.img = image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	c.tmpScreen, _ = ebiten.NewImage(c.width, c.height, ebiten.FilterNearest)

	c.font.bitmap = bitmap
	c.font.height = 16
	c.font.width = 9

	c.clear()

	return c
}

func (c *Console) Run() (err error) {
	// SetRunnableOnUnfocused
	ebiten.SetRunnableOnUnfocused(true)
	err = ebiten.Run(
		c.update,
		c.width,
		c.height,
		c.scale,
		c.title)
	return err
}

func (c *Console) input() {
	var r rune
	for c := 'A'; c <= 'Z'; c++ {
		if ebiten.IsKeyPressed(ebiten.Key(c) - 'A' + ebiten.KeyA) {
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				r = c
			}
			r = (c + 32) // convert to lowercase
			fmt.Println(string(r))
		}
	}
}

func (c *Console) update(screen *ebiten.Image) error {
	c.input()
	c.drawText()
	c.tmpScreen.ReplacePixels(c.img.Pix)
	screen.DrawImage(c.tmpScreen, nil)
	return nil
}

func (c *Console) clear() {
	c.cursor = 0
	for i := 0; i < len(c.videoTextMemory); i++ {
		c.videoTextMemory[i].charID = ' '
		c.videoTextMemory[i].bgColor = c.bgColor
		c.videoTextMemory[i].fgColor = c.fgColor
		c.videoTextMemory[i].blink = false
	}
}

func (c *Console) drawText() {
	i := 0
	rows := 25
	columns := 80
	for row := 0; row < rows; row++ {
		for col := 0; col < columns; col++ {
			v := c.videoTextMemory[i]
			if i == c.cursor {
				c.drawCursor(v.charID, v.fgColor, v.bgColor, col, row)
			} else {
				c.drawChar(v.charID, v.fgColor, v.bgColor, col, row)
			}
			i++
		}
	}
}

func (c *Console) moveUp() {
	columns := 80
	copy(c.videoTextMemory[0:], c.videoTextMemory[columns:])
	for i := len(c.videoTextMemory) - columns; i < len(c.videoTextMemory); i++ {
		c.videoTextMemory[i].charID = ' '
		c.videoTextMemory[i].bgColor = c.bgColor
		c.videoTextMemory[i].fgColor = c.fgColor
		c.videoTextMemory[i].blink = false
	}
}

func (c *Console) put(charID int) {
	c.videoTextMemory[c.cursor].fgColor = c.fgColor
	c.videoTextMemory[c.cursor].bgColor = c.bgColor
	c.videoTextMemory[c.cursor].charID = charID
	c.cursor++
	c.cursorLimit()
}

func (c *Console) cursorLimit() {
	if c.cursor < 0 {
		c.cursor = 0
		return
	}
	columns := 80
	rows := 25
	for c.cursor >= rows*columns {
		c.cursor -= columns
		c.moveUp()
	}
}

func (c *Console) Print(msg string) {
	columns := 80
	parseMode := false
	csi := false
	s := ""

	for i := 0; i < len(msg); i++ {
		v := msg[i]
		switch {
		case v == 7: // bell
			// not implemented
		case v == 8: // Backspace
			c.cursor--
			c.cursorLimit()
		case v == 9: // tab \t
			lin := int(c.cursor / columns)
			col := int(c.cursor % columns)
			ncol := int(col/4)*4 + 4 // tab size 4 and remove mod
			c.cursor = lin*columns + ncol
			c.cursorLimit()
		case v == 10: // Line Feed, \n
			c.cursor += columns
			c.cursorLimit()
		case v == 11: // Vertical tab
			// not implemented
		case v == 12: //  Formfeed
			// not implemented
		case v == 13: // Carriage return \r
			c.cursor = int(c.cursor/columns) * columns
			c.cursorLimit()
		case v == 27:
			parseMode = true
		case v == '7' && parseMode: // DEC primitive save cursor position
			c.auxCursorPos = c.cursor // Save cursor position
			parseMode = false
			csi = false
		case v == '8' && parseMode: // DEC primitive restore cursor position
			c.cursor = c.auxCursorPos // Restore cursor position
			parseMode = false
			csi = false
		case v == '[' && parseMode: // Control Sequence Introducer
			csi = true
			s = ""
		case v == 'c' && csi: // Reset display to initial state
			c.clear()
			c.bgColor, _ = colorParserBg(40)
			c.fgColor, _ = colorParserFg(37)
			//bold = false
			parseMode = false
			csi = false
			continue
		case v == 'm' && csi:
			sv := strings.Split(s, ";")
			//bold := false
			for _, j := range sv {
				if j == "" {
					continue
				} else if j == "0" {
					c.bgColor, _ = colorParserBg(40)
					c.fgColor, _ = colorParserFg(37)
					//bold = false
					continue
				} else if j == "1" {
					//bool = true
					continue
				} else if j == "39" { // Default foreground color
					c.fgColor, _ = colorParserFg(37)
					continue
				} else if j == "49" { // Default background color
					c.bgColor, _ = colorParserBg(37)
					continue
				} else {
					i, err := strconv.Atoi(j)
					if err != nil {
						fmt.Println(err, "code:", s)
						continue
					}
					fgColor, ok := colorParserFg(i)
					if ok {
						c.fgColor = fgColor
						continue
					}
					bgColor, ok := colorParserBg(i)
					if ok {
						c.bgColor = bgColor
						continue
					}
					fmt.Println("ANSI code not implemented:", i)
				}
			}
			parseMode = false
			csi = false
		case v == 'd' && csi:
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			cpos := i * columns
			if cpos < 2000 {
				c.cursor = cpos
			}
			parseMode = false
			csi = false
		case v == 's' && csi:
			c.auxCursorPos = c.cursor // Save cursor position
			parseMode = false
			csi = false
		case v == 'u' && csi:
			c.cursor = c.auxCursorPos // Restore cursor position
			parseMode = false
			csi = false
		case v == 'A' && csi: // Cursor up
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			c.cursor -= i * columns
			c.cursorLimit()
			parseMode = false
			csi = false
		case v == 'B' && csi: // Cursor down
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			c.cursor += i * columns
			c.cursorLimit()
			parseMode = false
			csi = false
		case v == 'C' && csi: // Cursor forward
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			c.cursor += i
			c.cursorLimit()
			parseMode = false
			csi = false
		case v == 'D' && csi: // Cursor back
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			c.cursor -= i
			c.cursorLimit()
			parseMode = false
			csi = false
		case v == 'G' && csi:
			i := 1
			if s != "" {
				var err error
				i, err = strconv.Atoi(s)
				if err != nil {
					fmt.Println(err)
				}
			}
			lin := int(c.cursor / columns)
			cpos := lin*columns + i
			if cpos < 2000 {
				c.cursor = cpos
			}
			parseMode = false
			csi = false
		case v == 'f' && csi: // the same as H
			fallthrough
		case v == 'H' && csi: // set horizontal and vertical position
			if s == "" {
				c.cursor = 0
			} else {
				sv := strings.Split(s, ";")
				if len(sv) == 2 {
					lin, _ := strconv.Atoi(sv[0])
					col, _ := strconv.Atoi(sv[1])
					cpos := lin*columns + col
					if cpos <= 2000 { // 25*80
						c.cursor = cpos
					}
				}
			}
			parseMode = false
			csi = false
		case v == 'X' && csi: // Erase n characters from the current position
			cpos := c.cursor
			i := 1
			if s != "" {
				i, _ = strconv.Atoi(s)
			}
			for x := 1; x <= i; x++ {
				if cpos+x < 2000 {
					c.videoTextMemory[cpos+x].charID = ' '
				}
			}
			c.cursor = cpos
			parseMode = false
			csi = false
		case v == 'J' && csi:
			if len(s) > 0 {
				if s[0] == '2' {
					c.clear()
				}
			}
			parseMode = false
			csi = false
		case v >= 'a' &&
			v <= 'z' &&
			v <= 'A' &&
			v <= 'Z' &&
			parseMode:
			parseMode = false
			csi = false
		case csi || parseMode:
			s += string(v)
		default:
			c.put(int(msg[i]))
		}
	}
}

func (c *Console) set(x, y int, color color) {
	p := 4*y*c.width + 4*x
	c.img.Pix[p] = color.R
	c.img.Pix[p+1] = color.G
	c.img.Pix[p+2] = color.B
	c.img.Pix[p+3] = 0xff
}

func (c *Console) drawCursor(index int, fgColor, bgColor color, x, y int) {
	if c.cursorSetBlink {
		if c.cursorBlinkTimer < 15 {
			fgColor, bgColor = bgColor, fgColor
		}
		c.drawChar(index, fgColor, bgColor, x, y)
		c.cursorBlinkTimer++
		if c.cursorBlinkTimer > 30 {
			c.cursorBlinkTimer = 0
		}
		return
	}
	c.drawChar(index, bgColor, fgColor, x, y)
}

func (c *Console) drawChar(index int, fgColor, bgColor color, x, y int) {
	var (
		a      int
		b      int
		lColor color
	)
	x = x * 9
	y = y * 16
	for b = 0; b < 16; b++ {
		for a = 0; a < 9; a++ {
			if a == 8 {
				color := bgColor
				if index >= 192 && index <= 223 {
					color = lColor
				}
				c.set(a+x, b+y, color)
				continue
			}
			i := index*16 + b
			if bitmap[i]&(0x80>>a) != 0 {
				lColor = fgColor
				c.set(a+x, b+y, lColor)
				continue
			}
			lColor = bgColor
			c.set(a+x, b+y, lColor)
		}
	}
}
