// Package config loads the YAML statusline configuration and merges each
// segment with the built-in defaults for its type.
package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level statusline configuration.
type Config struct {
	// Separator is inserted between non-empty segments.
	Separator string `yaml:"separator"`
	// NoColor disables all ANSI styling when true.
	NoColor bool `yaml:"no_color"`
	// Segments are rendered left-to-right.
	Segments []Segment `yaml:"segments"`
}

// Segment is one piece of the status line. Type selects built-in defaults for
// Template/When/Style/Symbol; any field set here overrides the corresponding
// default.
type Segment struct {
	Type     string  `yaml:"type"`
	Template string  `yaml:"template"`
	When     string  `yaml:"when"`
	Style    string  `yaml:"style"`
	Symbol   Symbols `yaml:"symbol"`
}

// Symbols holds a segment's overridable symbols, which its template reaches
// through the "symbol" helper. Keeping the glyph out of the template text is
// what lets a user swap 💰 for "$" without restating the whole template.
type Symbols map[string]string

// UnmarshalYAML accepts both shapes of the "symbol" field: a scalar for the
// common one-symbol segment ("symbol: $"), stored under the unnamed key, and a
// mapping for segments that pick between several ("symbol: {branch: …,
// worktree: …}"). A null value leaves the map nil, i.e. unset, so the type's
// defaults apply.
func (s *Symbols) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var str string
		if err := value.Decode(&str); err != nil {
			return err
		}
		*s = Symbols{"": str}
		return nil
	case yaml.MappingNode:
		m := map[string]string{}
		if err := value.Decode(&m); err != nil {
			return err
		}
		*s = m
		return nil
	}
	return fmt.Errorf("symbol: want a string or a mapping")
}

// Load resolves and reads the configuration. If path is empty it is discovered
// via the CC_STATUS_CONFIG env var and the standard XDG locations; when nothing
// is found the embedded default config is used. Either way every segment is
// merged with the defaults for its type before returning.
func Load(path string) (*Config, error) {
	data, src, err := readConfig(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", src, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// readConfig returns the raw config bytes and a label describing their source.
func readConfig(path string) (data []byte, src string, err error) {
	if p := resolvePath(path); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, p, fmt.Errorf("read config: %w", err)
		}
		return b, p, nil
	}
	return defaultYAML, "<built-in default>", nil
}

// resolvePath returns the first existing config path, or "" to signal that the
// embedded default should be used.
func resolvePath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if env := os.Getenv("CC_STATUS_CONFIG"); env != "" {
		return env
	}
	for _, p := range candidatePaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func candidatePaths() []string {
	var dirs []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dirs = append(dirs, xdg)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config"))
	}
	paths := make([]string, 0, len(dirs)*2)
	for _, d := range dirs {
		paths = append(paths,
			filepath.Join(d, "cc-status", "config.yaml"),
			filepath.Join(d, "cc-status", "config.yml"),
		)
	}
	return paths
}

// applyDefaults fills empty Template/When/Style/Symbol on each segment from the
// registry entry for its Type.
func (c *Config) applyDefaults() {
	for i := range c.Segments {
		s := &c.Segments[i]
		def, ok := defaults[s.Type]
		if !ok {
			continue
		}
		if s.Template == "" {
			s.Template = def.Template
		}
		if s.When == "" {
			s.When = def.When
		}
		if s.Style == "" {
			s.Style = def.Style
		}
		s.Symbol = mergeSymbols(s.Symbol, def.Symbol)
	}
}

// mergeSymbols fills the symbols a segment left unset from its type's defaults.
// Named symbols merge key by key, so overriding one of the git glyphs keeps the
// other; a scalar override (the unnamed key) replaces the whole set instead, so
// "symbol: ⎇" on a multi-symbol type reads as "use this one symbol" rather than
// adding a key nothing looks up. The defaults map is shared, so it is copied,
// never written to.
func mergeSymbols(dst, def Symbols) Symbols {
	if len(def) == 0 {
		return dst
	}
	if dst == nil {
		return maps.Clone(def)
	}
	if _, scalar := dst[""]; scalar {
		return dst
	}
	out := maps.Clone(def)
	maps.Copy(out, dst)
	return out
}
