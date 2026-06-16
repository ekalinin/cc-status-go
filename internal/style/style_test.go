package style

import "testing"

func TestApply(t *testing.T) {
	r := Renderer{}
	tests := []struct {
		name  string
		style string
		text  string
		want  string
	}{
		{"named fg", "cyan", "x", "\x1b[36mx\x1b[0m"},
		{"bold + named", "bold cyan", "x", "\x1b[1;36mx\x1b[0m"},
		{"bright fg", "bright_red", "x", "\x1b[91mx\x1b[0m"},
		{"bg named", "bg:blue", "x", "\x1b[44mx\x1b[0m"},
		{"256 color", "240", "x", "\x1b[38;5;240mx\x1b[0m"},
		{"hex truecolor", "#ff8800", "x", "\x1b[38;2;255;136;0mx\x1b[0m"},
		{"purple alias", "purple", "x", "\x1b[35mx\x1b[0m"},
		{"empty style passthrough", "", "x", "x"},
		{"unknown token ignored", "wat", "x", "x"},
		{"empty text", "cyan", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Apply(tt.style, tt.text); got != tt.want {
				t.Errorf("Apply(%q, %q) = %q, want %q", tt.style, tt.text, got, tt.want)
			}
		})
	}
}

func TestApplyDisabled(t *testing.T) {
	r := Renderer{Disabled: true}
	if got := r.Apply("bold cyan", "x"); got != "x" {
		t.Errorf("disabled Apply = %q, want %q", got, "x")
	}
}
