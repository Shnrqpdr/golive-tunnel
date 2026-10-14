// Package ui draws the console output: numbered steps, status lines, a spinner and boxes.
package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ANSI sequences. They only go out when the console accepts them: on an old terminal or with
// the output redirected they would turn into garbage in the middle of the text.
const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cCyan   = "\x1b[36m"

	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	wipeLine   = "\r\x1b[2K"
)

type glyphs struct {
	ok, warn, info, step string
	tl, tr, bl, br, h, v string
	spinner              []string
}

var wideGlyphs = glyphs{
	ok: "✔", warn: "▲", info: "•", step: "▸",
	tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│",
	spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
}

var plainGlyphs = glyphs{
	ok: "+", warn: "!", info: "-", step: ">",
	tl: "+", tr: "+", bl: "+", br: "+", h: "-", v: "|",
	spinner: []string{"|", "/", "-", "\\"},
}

type UI struct {
	mu      sync.Mutex
	out     *os.File
	rich    bool // accepts color and line rewriting
	g       glyphs
	total   int
	step    int
	live    string
	frame   int
	spinner chan struct{}
	started time.Time
}

func New() *UI {
	ansi, wide := prepareConsole(os.Stdout)
	// no-color.org convention: whoever exports the variable wants plain text everywhere.
	if os.Getenv("NO_COLOR") != "" {
		ansi, wide = false, false
	}

	u := &UI{out: os.Stdout, rich: ansi, g: plainGlyphs, started: time.Now()}
	if wide {
		u.g = wideGlyphs
	}
	return u
}

// Plan sets how many steps the counter in front of each Step shows.
func (u *UI) Plan(total int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.total = total
}

func (u *UI) paint(code, text string) string {
	if !u.rich || code == "" {
		return text
	}
	return code + text + cReset
}

// TryLock on purpose: the spinner must never hold up whoever is printing for real.
func (u *UI) animate() {
	ticker := time.NewTicker(90 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-u.spinner:
			return
		case <-ticker.C:
			if !u.mu.TryLock() {
				continue
			}
			u.frame++
			u.drawLive()
			u.mu.Unlock()
		}
	}
}

func (u *UI) drawLive() {
	if !u.rich || u.live == "" {
		return
	}
	frame := u.g.spinner[u.frame%len(u.g.spinner)]
	fmt.Fprint(u.out, wipeLine+"   "+u.paint(cCyan, frame)+"  "+u.live)
}

func (u *UI) setLive(text string) {
	if u.spinner == nil {
		u.spinner = make(chan struct{})
		fmt.Fprint(u.out, hideCursor)
		go u.animate()
	}
	u.live = text
	u.drawLive()
}

func (u *UI) clearLive() {
	if u.rich && u.live != "" {
		fmt.Fprint(u.out, wipeLine)
	}
	u.live = ""
}

func (u *UI) entry(symbol, code, text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearLive()
	fmt.Fprintf(u.out, "   %s  %s\n", u.paint(code, symbol), text)
}

func (u *UI) Step(format string, args ...any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearLive()
	u.step++

	counter := fmt.Sprintf("%d/%d", u.step, u.total)
	fmt.Fprintf(u.out, "\n %s %s  %s\n",
		u.paint(cCyan, u.g.step),
		u.paint(cDim, counter),
		u.paint(cBold, fmt.Sprintf(format, args...)))
}

func (u *UI) Ok(format string, args ...any) {
	u.entry(u.g.ok, cGreen, fmt.Sprintf(format, args...))
}

func (u *UI) Warn(format string, args ...any) {
	u.entry(u.g.warn, cYellow, fmt.Sprintf(format, args...))
}

func (u *UI) Info(format string, args ...any) {
	u.entry(u.g.info, cCyan, fmt.Sprintf(format, args...))
}

func (u *UI) Detail(format string, args ...any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearLive()
	fmt.Fprintf(u.out, "      %s\n", u.paint(cDim, fmt.Sprintf(format, args...)))
}

// Busy shows what is happening right now; the next line printed replaces it.
func (u *UI) Busy(format string, args ...any) {
	text := fmt.Sprintf(format, args...)

	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.rich {
		fmt.Fprintf(u.out, "   %s  %s\n", u.g.info, text)
		return
	}
	u.setLive(u.paint(cDim, text))
}

func runeLen(s string) int {
	return len([]rune(s))
}

// Widths come from the raw text: measuring after coloring would count the ANSI codes.
func (u *UI) box(border, title string, rows [][2]string) {
	labels := 0
	for _, row := range rows {
		if n := runeLen(row[0]); n > labels {
			labels = n
		}
	}

	gap := "  "
	if labels == 0 {
		gap = ""
	}

	inner := runeLen(title) + 6
	for _, row := range rows {
		if n := 2 + labels + runeLen(gap) + runeLen(row[1]) + 2; n > inner {
			inner = n
		}
	}

	g := u.g
	head := g.h + g.h
	if title != "" {
		head = g.h + " " + title + " "
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearLive()

	fmt.Fprintf(u.out, "\n %s\n", u.paint(border, g.tl+head+strings.Repeat(g.h, inner-runeLen(head))+g.tr))
	for _, row := range rows {
		width := 2 + labels + runeLen(gap) + runeLen(row[1])
		cell := "  " + u.paint(cDim, row[0]) + strings.Repeat(" ", labels-runeLen(row[0])) +
			gap + u.paint(cBold, row[1]) + strings.Repeat(" ", inner-width)
		fmt.Fprintf(u.out, " %s%s%s\n", u.paint(border, g.v), cell, u.paint(border, g.v))
	}
	fmt.Fprintf(u.out, " %s\n", u.paint(border, g.bl+strings.Repeat(g.h, inner)+g.br))
}

func (u *UI) Banner(version string) {
	name := "golive-tunnel"
	if version != "" && version != "dev" {
		name += " " + version
	}

	u.box(cCyan, "", [][2]string{
		{"", name},
		{"", "abre o Discord por um tunel WireGuard ate o seu servidor"},
	})
}

// Notice is a yellow box for something the person has to act on.
func (u *UI) Notice(title string, lines []string) {
	rows := make([][2]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, [2]string{"", line})
	}
	u.box(cYellow, title, rows)
}

// Summary is the green box at the end, with label/value rows plus the elapsed time.
func (u *UI) Summary(title string, rows [][2]string) {
	u.box(cGreen, title, append(rows, [2]string{"tempo", u.elapsed()}))
}

func (u *UI) elapsed() string {
	elapsed := time.Since(u.started).Round(time.Second)
	if elapsed < time.Minute {
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}
	return fmt.Sprintf("%dmin %ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
}

// Word wrap: a long error message inside the box would break its width.
func wrap(text string, width int) []string {
	var lines []string
	current := ""

	for _, word := range strings.Fields(text) {
		switch {
		case current == "":
			current = word
		case runeLen(current)+1+runeLen(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

// Fatal shows the error in a red box. Whoever runs the executable has neither the README nor the
// code: help comes from whoever handed them the program, so the footer says to send it there.
func (u *UI) Fatal(err error) {
	u.Close()

	var rows [][2]string
	for _, line := range wrap(err.Error(), 66) {
		rows = append(rows, [2]string{"", line})
	}
	rows = append(rows,
		[2]string{"", ""},
		[2]string{"", "se nao resolver, mande esta mensagem para quem te passou o programa"})

	u.box(cRed, "nao deu certo", rows)
	fmt.Fprintln(u.out)
}

// Close stops the spinner and gives the cursor back. Without it, leaving in the middle of a
// Busy line would keep the cursor hidden after the program exits.
func (u *UI) Close() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.spinner != nil {
		close(u.spinner)
		u.spinner = nil
	}
	u.clearLive()
	if u.rich {
		fmt.Fprint(u.out, showCursor)
	}
}

// Print writes a plain line, outside any box or step.
func (u *UI) Print(text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearLive()
	fmt.Fprintln(u.out, text)
}
