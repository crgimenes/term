package coreterm

import (
	"fmt"
	"image"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten"
)

const (
	rows    = 25
	columns = 80
)

var (
	videoTextMemory  [rows * columns * 2]byte
	cursor           int
	img              *image.RGBA
	currentColor     byte = 0x0F
	updateScreen     bool
	cpx, cpy         int
	cursorBlinkTimer int
	cursorSetBlink   = true

	uTime uint64

	machine int

	//var countaux int
	noKey   bool
	shift   bool
	lastKey = struct {
		Time uint64
		Char byte
	}{
		0,
		0,
	}
)

type Instance struct {
	Border        int
	Height        int
	Width         int
	Scale         float64
	CurrentColor  byte
	uTime         int
	updateScreen  bool
	tmpScreen     *ebiten.Image
	img           *image.RGBA
	ScreenHandler func(*Instance) error
	Title         string
	Font          struct {
		Height int
		Width  int
		Bitmap []byte
	}
}

var ct *Instance

func Get() *Instance {
	ct = &Instance{}
	ct.Width = columns * 9
	ct.Height = rows * 16
	ct.Scale = 1
	ct.ScreenHandler = update
	ct.Title = "term"
	ct.CurrentColor = 0x0
	return ct
}

var Colors = []struct {
	R byte
	G byte
	B byte
}{
	{0, 0, 0},
	{0, 0, 170},
	{0, 170, 0},
	{0, 170, 170},
	{170, 0, 0},
	{170, 0, 170},
	{170, 85, 0},
	{170, 170, 170},
	{85, 85, 85},
	{85, 85, 255},
	{85, 255, 85},
	{85, 255, 255},
	{255, 85, 85},
	{255, 85, 255},
	{255, 255, 85},
	{255, 255, 255},
}

func MergeColorCode(b, f byte) byte {
	return (f & 0xff) | (b << 4)
}

func updateTermScreen(screen *ebiten.Image) error {
	if ct.ScreenHandler != nil {
		err := ct.ScreenHandler(ct)
		if err != nil {
			return err
		}
	}
	if ct.updateScreen {
		ct.tmpScreen.ReplacePixels(ct.img.Pix)
		ct.updateScreen = false
	}
	screen.DrawImage(ct.tmpScreen, nil)
	ct.uTime++
	return nil
}

func (i *Instance) Run() {

		i.Font.Bitmap = bitmap
		i.Font.Height = 16
		i.Font.Width = 9
	i.img = image.NewRGBA(image.Rect(0, 0, i.Width, i.Height))
	i.tmpScreen, _ = ebiten.NewImage(i.Width, i.Height, ebiten.FilterNearest)

	i.Clear()
	i.updateScreen = true
	clearVideoTextMode()

	if err := ebiten.Run(updateTermScreen, i.Width, i.Height, i.Scale, i.Title); err != nil {
		log.Fatal(err)
	}
}

func (i *Instance) DrawPix(x, y int) {
	x += i.Border
	y += i.Border
	if x < i.Border || 
	y < i.Border || 
	x >= i.Width-i.Border || 
	y >= i.Height-i.Border {
		return
	}
	pos := 4*y*i.Width + 4*x
	i.img.Pix[pos] = Colors[i.CurrentColor].R
	i.img.Pix[pos+1] = Colors[i.CurrentColor].G
	i.img.Pix[pos+2] = Colors[i.CurrentColor].B
	i.img.Pix[pos+3] = 0xff
	i.updateScreen = true
}

func (i *Instance) DrawChar(index, fgColor, bgColor byte, x, y int) {
	var a uint
	var b uint
	var lColor byte
	for b = 0; b < 16; b++ {
		for a = 0; a < 9; a++ {
			if a == 8 {
				i.CurrentColor = bgColor
				if index >= 192 && index <= 223 {
					i.CurrentColor = lColor
				}
				i.DrawPix(int(a)+x, int(b)+y)
				continue
			}
			idx := uint(index)*16 + b
			if bitmap[idx]&(0x80>>a) != 0 {
				i.CurrentColor = fgColor
				lColor = fgColor
				i.DrawPix(int(a)+x, int(b)+y)
				continue
			}
			i.CurrentColor = bgColor
			lColor = bgColor
			i.DrawPix(int(a)+x, int(b)+y)
		}
	}
}

func (i *Instance) Clear() {
	for idx := 0; idx < i.Height*i.Width*4; idx += 4 {
		i.img.Pix[idx] = Colors[i.CurrentColor].R
		i.img.Pix[idx+1] = Colors[i.CurrentColor].G
		i.img.Pix[idx+2] = Colors[i.CurrentColor].B
		i.img.Pix[idx+3] = 0xff
	}
}

func (i *Instance) DrawCursor(index, fgColor, bgColor byte, x, y int) {
	if cursorSetBlink {
		if cursorBlinkTimer < 15 {
			fgColor, bgColor = bgColor, fgColor
		}
		i.DrawChar(index, fgColor, bgColor, x, y)
		cursorBlinkTimer++
		if cursorBlinkTimer > 30 {
			cursorBlinkTimer = 0
		}
		return
	}
	i.DrawChar(index, bgColor, fgColor, x, y)
}

func (i *Instance) DrawVideoTextMode() {
	idx := 0
	for r := 0; r < rows; r++ {
		for c := 0; c < columns; c++ {
			color := videoTextMemory[idx]
			f := color & 0x0f
			b := color & 0xf0 >> 4
			if idx == cursor {
				idx++
				i.DrawCursor(videoTextMemory[idx], f, b, c*9, r*16)
			} else {
				idx++
				i.DrawChar(videoTextMemory[idx], f, b, c*9, r*16)
			}
			idx++
		}
	}
}

func clearVideoTextMode() {
	copy(videoTextMemory[:], make([]byte, len(videoTextMemory)))
	for i := 0; i < len(videoTextMemory); i += 2 {
		videoTextMemory[i] = currentColor
	}
}

func moveLineUp() {
	copy(videoTextMemory[0:], videoTextMemory[columns*2:])
	copy(videoTextMemory[len(videoTextMemory)-columns*2:], make([]byte, columns*2))
	for i := len(videoTextMemory) - columns*2; i < len(videoTextMemory); i += 2 {
		videoTextMemory[i] = currentColor
	}
}

func correctVideoCursor() {
	if cursor < 0 {
		cursor = 0
	}
	for cursor >= rows*columns*2 {
		cursor -= columns * 2
		moveLineUp()
	}
}

func putChar(c byte) {
	correctVideoCursor()
	videoTextMemory[cursor] = currentColor
	cursor++
	correctVideoCursor()
	videoTextMemory[cursor] = c
	cursor++
	correctVideoCursor()
}

func bPrint(msg string) {
	for i := 0; i < len(msg); i++ {
		c := msg[i]

		switch c {
		case 13:
			cursor += columns * 2
			continue
		case 10:
			aux := cursor / (columns * 2)
			aux = aux * (columns * 2)
			cursor = aux
			continue
		}
		putChar(msg[i])
	}
}

func bPrintln(msg string) {
	msg += "\r\n"
	bPrint(msg)
}

func keyTreatment(c byte, f func(c byte)) {
	if noKey || lastKey.Char != c || lastKey.Time+20 < uTime {
		f(c)
		noKey = false
		lastKey.Char = c
		lastKey.Time = uTime
	}
}

func getLine() string {
	aux := cursor / (columns * 2)
	var ret string
	for i := aux*(columns*2) + 1; i < aux*(columns*2)+columns*2; i += 2 {
		c := videoTextMemory[i]
		if c == 0 {
			break
		}
		ret += string(videoTextMemory[i])
	}

	ret = strings.TrimSpace(ret)
	return ret
}

func eval(cmd string) {
	fmt.Println("eval:", cmd)
}

func input() {
	for c := 'A'; c <= 'Z'; c++ {
		if ebiten.IsKeyPressed(ebiten.Key(c) - 'A' + ebiten.KeyA) {
			keyTreatment(byte(c), func(c byte) {
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					putChar(c)
					return
				}
				putChar(c + 32) // convert to lowercase
			})
			return
		}
	}

	for c := '0'; c <= '9'; c++ {
		if ebiten.IsKeyPressed(ebiten.Key(c) - '0' + ebiten.Key0) {
			keyTreatment(byte(c), func(c byte) {
				putChar(c)
			})
			return
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeySpace) {
		keyTreatment(byte(' '), func(c byte) {
			putChar(c)
		})
		return
	}

	if ebiten.IsKeyPressed(ebiten.KeyComma) {
		keyTreatment(byte(','), func(c byte) {
			putChar(c)
		})
		return
	}

	if ebiten.IsKeyPressed(ebiten.KeyEnter) {
		keyTreatment(0, func(c byte) {
			eval(getLine())
			cursor += columns * 2
			aux := cursor / (columns * 2)
			aux = aux * (columns * 2)
			cursor = aux
			correctVideoCursor()
		})
		return
	}

	if ebiten.IsKeyPressed(ebiten.KeyBackspace) {
		keyTreatment(0, func(c byte) {
			cursor -= 2
			line := cursor / (columns * 2)
			lineEnd := line*columns*2 + columns*2
			if cursor < 0 {
				cursor = 0
			}

			copy(videoTextMemory[cursor:lineEnd], videoTextMemory[cursor+2:lineEnd])
			videoTextMemory[lineEnd-2] = currentColor
			videoTextMemory[lineEnd-1] = 0

			correctVideoCursor()
		})
		return
	}

	/*
	   KeyMinus: -
	   KeyEqual: =
	   KeyLeftBracket: [
	   KeyRightBracket: ]
	   KeyBackslash:
	   KeySemicolon: ;
	   KeyApostrophe: '
	   KeySlash: /
	   KeyGraveAccent: `
	*/

	shift = ebiten.IsKeyPressed(ebiten.KeyShift)

	if ebiten.IsKeyPressed(ebiten.KeyEqual) {
		if shift {
			keyTreatment('+', func(c byte) {
				putChar(c)
				println("+")
			})
			return
		} else {
			keyTreatment('=', func(c byte) {
				putChar(c)
				println("=")
			})
			return
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		keyTreatment(0, func(c byte) {
			cursor -= columns * 2
			correctVideoCursor()
		})
		return
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		keyTreatment(0, func(c byte) {
			cursor += columns * 2
			correctVideoCursor()
		})
		return
	}
	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		keyTreatment(0, func(c byte) {
			cursor -= 2
			correctVideoCursor()
		})
		return
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		keyTreatment(0, func(c byte) {
			cursor += 2
			correctVideoCursor()
		})
		return
	}

	// When the "left mouse button" is pressed...
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		//ebitenutil.DebugPrint(screen, "You're pressing the 'LEFT' mouse button.")
	}
	// When the "right mouse button" is pressed...
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		//ebitenutil.DebugPrint(screen, "\nYou're pressing the 'RIGHT' mouse button.")
	}
	// When the "middle mouse button" is pressed...
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		//ebitenutil.DebugPrint(screen, "\n\nYou're pressing the 'MIDDLE' mouse button.")
	}

	cpx, cpy = ebiten.CursorPosition()
	//fmt.Printf("X: %d, Y: %d\n", x, y)

	// Display the information with "X: xx, Y: xx" format
	//ebitenutil.DebugPrint(screen, fmt.Sprintf("X: %d, Y: %d", x, y))

	noKey = true

}

func update(screen *Instance) error {
	uTime++

	if machine == 0 {
		machine++
		bPrintln("          1         2         3         4         5         6         7")
		bPrintln("01234567890123456789012345678901234567890123456789012345678901234567890123456789")
		bPrintln("terminal v0.01")
		bPrintln("https://crg.eti.br")
		bPrintln(fmt.Sprintf("Width: %v, Height: %v", screen.Width, screen.Height))
		bPrintln("")

		var i byte
		for ; i < 246; i++ {
			putChar(i)
		}

	}

	ct.DrawVideoTextMode()

	input()
	return nil
}
