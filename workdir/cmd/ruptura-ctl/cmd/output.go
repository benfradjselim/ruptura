package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"

	bgRed   = "\033[41m"
)

func color(c, s string) string {
	if noColor {
		return s
	}
	return c + s + colorReset
}

func bold(s string) string    { return color(colorBold, s) }
func dim(s string) string     { return color(colorDim+colorGray, s) }
func green(s string) string   { return color(colorGreen, s) }
func yellow(s string) string  { return color(colorYellow, s) }
func red(s string) string     { return color(colorRed, s) }
func cyan(s string) string    { return color(colorCyan, s) }
func magenta(s string) string { return color(colorMagenta, s) }
func blue(s string) string    { return color(colorBlue, s) }

func healthColor(score float64) string {
	switch {
	case score >= 80:
		return green(fmt.Sprintf("%.0f", score))
	case score >= 40:
		return yellow(fmt.Sprintf("%.0f", score))
	default:
		return red(fmt.Sprintf("%.0f", score))
	}
}

// kpiHealth renders a health score, or a dimmed n/a when its inputs are not
// measured, so an unmeasured workload never looks like a failing one.
func kpiHealth(k models.KPI) string {
	if k.Unavailable {
		return dim("n/a")
	}
	return healthColor(k.Value)
}

func fusedRColor(r float64) string {
	s := fmt.Sprintf("%.2f", r)
	switch {
	case r >= 5.0:
		return red(bold(s + " !!!"))
	case r >= 3.0:
		return red(s + " ⚠")
	case r >= 1.5:
		return yellow(s + " ·")
	default:
		return green(s)
	}
}

func stateIcon(state string) string {
	switch state {
	case "active":
		return green("●")
	case "calibrating":
		return yellow("◐")
	case "warning":
		return yellow("▲")
	case "critical", "emergency":
		return red("■")
	default:
		return dim("○")
	}
}

func calibBar(pct int) string {
	if pct >= 100 {
		return green("100%")
	}
	return yellow(fmt.Sprintf("%3d%%", pct))
}

func etaStr(minutes int) string {
	if minutes <= 0 {
		return dim("—")
	}
	if minutes < 15 {
		return red(fmt.Sprintf("%dm", minutes))
	}
	if minutes < 30 {
		return yellow(fmt.Sprintf("%dm", minutes))
	}
	return fmt.Sprintf("%dm", minutes)
}

func uptimeStr(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func fmtNum(n int64) string {
	if n == 0 {
		return dim("0")
	}
	s := fmt.Sprintf("%d", n)
	// insert thousands separators
	out := []byte{}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// printJSON prints v as indented JSON.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- Table renderer ---

type table struct {
	headers []string
	rows    [][]string
	aligns  []int // -1 left, 0 center, 1 right
}

func newTable(headers ...string) *table {
	return &table{headers: headers, aligns: make([]int, len(headers))}
}

func (t *table) alignRight(col int) { t.aligns[col] = 1 }

func (t *table) add(cols ...string) {
	t.rows = append(t.rows, cols)
}

func (t *table) print() {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = visLen(h)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) {
				w := visLen(cell)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	sep := strings.Repeat("─", totalWidth(widths)+len(widths)*2)
	fmt.Println()

	// header
	line := "  "
	for i, h := range t.headers {
		line += pad(bold(h), widths[i], t.aligns[i]) + "  "
	}
	fmt.Println(line)
	fmt.Println("  " + dim(sep))

	// rows
	for _, row := range t.rows {
		line = "  "
		for i := range t.headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			line += pad(cell, widths[i], t.aligns[i]) + "  "
		}
		fmt.Println(line)
	}
	fmt.Println("  " + dim(sep))
}

// visLen returns the visible length of a string (strips ANSI codes).
func visLen(s string) int {
	inEsc := false
	l := 0
	for _, c := range s {
		if inEsc {
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		if c == '\033' {
			inEsc = true
			continue
		}
		l++
	}
	return l
}

func totalWidth(widths []int) int {
	total := 0
	for _, w := range widths {
		total += w
	}
	return total
}

// pad pads a string (with ANSI codes) to target visible width.
func pad(s string, width, align int) string {
	vis := visLen(s)
	gap := width - vis
	if gap <= 0 {
		return s
	}
	spaces := strings.Repeat(" ", gap)
	if align == 1 {
		return spaces + s
	}
	return s + spaces
}

// mini progress bar [████░░░░] for 0-100
func progressBar(pct int, width int) string {
	filled := int(math.Round(float64(pct) / 100.0 * float64(width)))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	switch {
	case pct >= 80:
		return green(bar)
	case pct >= 40:
		return yellow(bar)
	default:
		return red(bar)
	}
}

func warnBanner(msg string) {
	fmt.Println()
	fmt.Println(red("  ⚠  " + msg))
	fmt.Println()
}

func successLine(msg string) {
	fmt.Println(green("  ✓  ") + msg)
}

func infoLine(msg string) {
	fmt.Println(cyan("  →  ") + msg)
}

func errLine(msg string) {
	fmt.Fprintln(os.Stderr, red("  ✗  ")+msg)
}
