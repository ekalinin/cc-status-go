// Command cc-status renders a Claude Code status line. It reads the session
// JSON from stdin and writes a single formatted line to stdout, shaped by a
// YAML config (see internal/config/default.yaml).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/shorrty/cc-status-go/internal/config"
	"github.com/shorrty/cc-status-go/internal/input"
	"github.com/shorrty/cc-status-go/internal/render"
	"github.com/shorrty/cc-status-go/internal/style"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// A status line must never crash the host UI: recover from any panic and
	// fall back to printing nothing.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "cc-status: panic: %v\n", r)
			os.Exit(0)
		}
	}()

	var (
		configPath  = flag.String("config", "", "path to config file (default: discovered)")
		noColor     = flag.Bool("no-color", false, "disable ANSI colors")
		printConfig = flag.Bool("print-config", false, "print the built-in default config and exit")
		showVersion = flag.Bool("version", false, "print version and exit")
		debug       = flag.Bool("debug", false, "report segment template errors to stderr")
	)
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println(version)
		return
	case *printConfig:
		os.Stdout.Write(config.DefaultYAML())
		return
	}

	if err := run(*configPath, *noColor, *debug); err != nil {
		// Still exit 0 so Claude Code doesn't treat the status line as fatal.
		fmt.Fprintf(os.Stderr, "cc-status: %v\n", err)
		os.Exit(0)
	}
}

func run(configPath string, noColor, debug bool) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	in, err := input.Parse(raw)
	if err != nil && debug {
		fmt.Fprintf(os.Stderr, "cc-status: parse input: %v\n", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	sty := style.Renderer{Disabled: noColor || cfg.NoColor || os.Getenv("NO_COLOR") != ""}
	line := render.Render(in, cfg, sty, render.Options{Debug: debug})

	fmt.Fprintln(os.Stdout, line)
	return nil
}
