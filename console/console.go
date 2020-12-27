package console

import (
	"image"

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

	cursor int
	height int
	width  int
	scale  float64

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

func New() *Console {
	c := &Console{}
	c.bgColor = color{0, 0, 0}
	c.fgColor = color{255, 255, 255}
	c.width = 80 * 9
	c.height = 25 * 16
	c.scale = 1
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

func (c *Console) update(screen *ebiten.Image) error {
	c.drawText()
	c.tmpScreen.ReplacePixels(c.img.Pix)
	screen.DrawImage(c.tmpScreen, nil)
	return nil
}

func (c *Console) clear() {
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
	columns := 80
	rows := 25
	for c.cursor >= rows*columns {
		c.cursor -= columns
		c.moveUp()
	}
}

func (c *Console) Print(msg string) {
	columns := 80
	for i := 0; i < len(msg); i++ {
		switch msg[i] {
		case 10: // Line Feed, \n
			c.cursor += columns
			c.cursorLimit()
			continue
		case 13: // Carriage Return, \r
			c.cursor = int(c.cursor/columns) * columns
			c.cursorLimit()
			continue
		}

		c.put(int(msg[i]))
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
