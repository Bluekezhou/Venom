package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-runewidth"
	"github.com/mattn/go-tty"
)

var history []string = []string{""}
var logfile string = ".venom_history"
var historyFd *os.File = nil

type ctx struct {
	w       io.Writer
	input   []rune
	last    []rune
	prompt  string
	cursorX int
	oldRow  int
	oldCrow int
	size    int
}

func (c *ctx) redraw(dirty bool, passwordChar rune) error {
	var buf bytes.Buffer

	// buf.WriteString("\x1b[5>h")

	buf.WriteString("\x1b[1G")
	if dirty {
		buf.WriteString("\x1b[0K")
	}
	for i := 0; i < c.oldRow-c.oldCrow; i++ {
		buf.WriteString("\x1b[B")
	}
	for i := 0; i < c.oldRow; i++ {
		if dirty {
			buf.WriteString("\x1b[2K")
		}
		buf.WriteString("\x1b[A")
	}

	var rs []rune
	if passwordChar != 0 {
		for i := 0; i < len(c.input); i++ {
			rs = append(rs, passwordChar)
		}
	} else {
		rs = c.input
	}

	ccol, crow, col, row := -1, 0, 0, 0
	plen := len([]rune(c.prompt))
	for i, r := range []rune(c.prompt + string(rs)) {
		if i == plen+c.cursorX {
			ccol = col
			crow = row
		}
		rw := runewidth.RuneWidth(r)
		if col+rw > c.size {
			col = 0
			row++
			if dirty {
				buf.WriteString("\n\r\x1b[0K")
			}
		}
		if dirty {
			buf.WriteString(string(r))
		}
		col += rw
	}
	if dirty {
		buf.WriteString("\x1b[1G")
		for i := 0; i < row; i++ {
			buf.WriteString("\x1b[A")
		}
	}
	if ccol == -1 {
		ccol = col
		crow = row
	}
	for i := 0; i < crow; i++ {
		buf.WriteString("\x1b[B")
	}
	buf.WriteString(fmt.Sprintf("\x1b[%dG", ccol+1))

	// buf.WriteString("\x1b[5>l")
	io.Copy(c.w, &buf)

	c.oldRow = row
	c.oldCrow = crow

	return nil
}

// SaveHistory to logfile
func SaveHistory(line string) error {
	var err error

	if historyFd == nil {
		historyFd, err = os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
	}

	_, err = historyFd.WriteString(line + "\n")
	if err != nil {
		return err
	}
	historyFd.Sync()
	return nil
}

// ReadLine from console
func ReadLine(tty *tty.TTY, msg string) (string, error) {
	c := new(ctx)
	c.w = colorable.NewColorableStdout()
	quit := false
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		c.input = nil
		quit = true
	}()
	c.size = 80
	c.input = []rune(msg)

	baseSize := len(c.input)
	c.cursorX = baseSize
	dirty := true
	historyIndex := len(history)
loop:
	for !quit {
		err := c.redraw(dirty, 0)
		if err != nil {
			return "", err
		}
		dirty = false

		r, err := tty.ReadRune()
		if err != nil {
			break
		}
		// fmt.Printf("r %d\n", r)
		switch r {
		case 0:
		case 1: // CTRL-A
			c.cursorX = baseSize
		case 2: // CTRL-B
			if c.cursorX > baseSize {
				c.cursorX--
			}
		case 3: // BREAK
			fmt.Println("Ctrl+C")
			os.Exit(0)
			return "", nil
		case 4: // CTRL-D
			if len(c.input) > 0 {
				continue
			}
			return "", io.EOF
		case 5: // CTRL-E
			c.cursorX = len(c.input)
		case 6: // CTRL-F
			if c.cursorX < len(c.input) {
				c.cursorX++
			}
		case 8, 0x7F: // BS
			if c.cursorX > baseSize {
				c.input = append(c.input[0:c.cursorX-1], c.input[c.cursorX:len(c.input)]...)
				c.cursorX--
				dirty = true
			}
		case 27:
			if !tty.Buffered() {
				return "", io.EOF
			}
			r, err = tty.ReadRune()
			if err == nil && r == 0x5b {
				r, err = tty.ReadRune()
				if err != nil {
					panic(err)
				}
				switch r {
				case 'A': // arrow up
					if historyIndex == len(history) && historyIndex >= 2 {
						historyIndex -= 2 // jump over last blank padding string
					} else if historyIndex > 0 {
						historyIndex--
					}
					c.input = append([]rune(msg), []rune(history[historyIndex])...)
					c.cursorX = len(c.input)
					dirty = true
				case 'B': // arrow down
					historyIndex = historyIndex + 1
					if historyIndex > len(history)-1 {
						historyIndex = len(history) - 1
					}
					c.input = append([]rune(msg), []rune(history[historyIndex])...)
					c.cursorX = len(c.input)
					dirty = true
				case 'C': // arrow right
					if c.cursorX < len(c.input) {
						c.cursorX++
					}
				case 'D': // arrow left
					if c.cursorX > baseSize {
						c.cursorX--
					}
				}
			}
		case 10: // LF
			break loop
		case 11: // CTRL-K
			c.input = c.input[:c.cursorX]
			dirty = true
		case 12: // CTRL-L
			dirty = true
		case 13: // CR
			break loop
		case 21: // CTRL-U
			c.input = append(c.input[:baseSize], c.input[c.cursorX:]...)
			c.cursorX = baseSize
			dirty = true
		case 23: // CTRL-W
			for i := len(c.input) - 1; i >= 0; i-- {
				if i == 0 || c.input[i] == ' ' || c.input[i] == '\t' {
					c.input = append(c.input[:i], c.input[c.cursorX:]...)
					c.cursorX = i
					dirty = true
					break
				}
			}
		default:
			if len(c.input) < baseSize {
				// triggered by Ctrl+C
				dirty = true
				continue
			}

			tmp := []rune{}
			tmp = append(tmp, c.input[0:c.cursorX]...)
			tmp = append(tmp, r)
			c.input = append(tmp, c.input[c.cursorX:len(c.input)]...)
			c.cursorX++
			dirty = true
		}
	}
	os.Stdout.WriteString("\n")

	if c.input == nil {
		return "", io.EOF
	}

	inputstr := string(c.input[baseSize:])
	if len(inputstr) == 0 {
		return "", nil
	}

	if len(history) == 1 {
		history[0] = inputstr
		history = append(history, "")
		SaveHistory(inputstr)
	} else if inputstr != history[len(history)-2] {
		history[len(history)-1] = inputstr
		history = append(history, "")
		SaveHistory(inputstr)
	}
	return inputstr, nil
}

// LoadHistory from logfile
func LoadHistory() error {
	fd, err := os.Open(logfile)
	if err != nil {
		// println("logfile not existed")
		return err
	}

	buf := bufio.NewReader(fd)
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		history = append(history, line)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// Clear opened fd
func Clear() error {
	err := historyFd.Close()
	if err != nil {
		return err
	}

	historyFd = nil
	return nil
}
