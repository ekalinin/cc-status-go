// Package style turns human-readable style strings (e.g. "bold cyan",
// "fg:#ff8800 bg:240 underline") into ANSI SGR escape sequences.
package style

import (
	"strconv"
	"strings"
)

// Renderer applies styles to text. When Disabled is true (NO_COLOR, the
// --no-color flag, or no_color in the config) Apply returns text untouched.
type Renderer struct {
	Disabled bool
}

const reset = "\x1b[0m"

// named maps color names to their base ANSI 16-color SGR codes. Foreground
// uses the code directly; background adds 10. Bright variants add 60.
var named = map[string]int{
	"black":   30,
	"red":     31,
	"green":   32,
	"yellow":  33,
	"blue":    34,
	"magenta": 35,
	"purple":  35, // alias, matches starship
	"cyan":    36,
	"white":   37,
}

// Apply wraps text in the SGR codes described by styleStr. Unknown tokens are
// ignored. An empty style (or a disabled renderer) returns text unchanged.
func (r Renderer) Apply(styleStr, text string) string {
	if r.Disabled || strings.TrimSpace(styleStr) == "" || text == "" {
		return text
	}
	codes := parse(styleStr)
	if len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + reset
}

// parse converts a style string into a slice of SGR parameter strings.
func parse(styleStr string) []string {
	var codes []string
	for tok := range strings.FieldsSeq(styleStr) {
		switch strings.ToLower(tok) {
		case "bold":
			codes = append(codes, "1")
		case "dim", "dimmed", "faint":
			codes = append(codes, "2")
		case "italic":
			codes = append(codes, "3")
		case "underline":
			codes = append(codes, "4")
		case "blink":
			codes = append(codes, "5")
		case "reverse", "inverted":
			codes = append(codes, "7")
		case "strikethrough", "strikethrough_text":
			codes = append(codes, "9")
		default:
			if c := colorCodes(tok, false); c != "" {
				codes = append(codes, c)
			}
		}
	}
	return codes
}

// colorCodes resolves a single color token to its SGR parameters. The token
// may be a bare color (foreground) or prefixed with "fg:"/"bg:". Supported
// color forms: named colors (+ "bright_" prefix), hex "#rrggbb" (truecolor),
// and a plain integer 0-255 (256-color palette).
func colorCodes(tok string, _ bool) string {
	bg := false
	if rest, ok := strings.CutPrefix(tok, "fg:"); ok {
		tok = rest
	} else if rest, ok := strings.CutPrefix(tok, "bg:"); ok {
		tok = rest
		bg = true
	}
	tok = strings.ToLower(tok)

	// Hex truecolor: #rrggbb
	if hex, ok := strings.CutPrefix(tok, "#"); ok && len(hex) == 6 {
		r, g, b, ok := parseHex(hex)
		if !ok {
			return ""
		}
		lead := "38"
		if bg {
			lead = "48"
		}
		return lead + ";2;" + itoa(r) + ";" + itoa(g) + ";" + itoa(b)
	}

	// Named colors, optionally bright_*.
	name := tok
	bright := false
	if rest, ok := strings.CutPrefix(name, "bright_"); ok {
		name = rest
		bright = true
	}
	if base, ok := named[name]; ok {
		code := base
		if bg {
			code += 10
		}
		if bright {
			code += 60
		}
		return itoa(code)
	}

	// 256-color palette index.
	if n, err := strconv.Atoi(tok); err == nil && n >= 0 && n <= 255 {
		lead := "38"
		if bg {
			lead = "48"
		}
		return lead + ";5;" + itoa(n)
	}

	return ""
}

func parseHex(hex string) (r, g, b int, ok bool) {
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff, true
}

func itoa(n int) string { return strconv.Itoa(n) }
