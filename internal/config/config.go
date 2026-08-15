// Package config reads .plumbline.toml — the file that lets a project tune
// Plumbline without forking it.
//
// There is exactly one reason this exists: a linter that cannot be tuned gets
// deleted rather than tuned. One rule that fires on a pattern a team has
// decided is fine, with no way to turn it down, is enough to lose the other
// rules too. So a project can lower a rule's severity, switch it off, and keep
// Plumbline out of directories it has no business reading.
//
// What configuration deliberately cannot do is change what a rule looks for.
// Severity, enablement and file selection are policy and live here; what
// counts as a finding is the rule's, and stays in the rule.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// FileName is the name Plumbline looks for in the working directory when no
// configuration file was named on the command line.
const FileName = ".plumbline.toml"

// off is the severity value that switches a rule off entirely. It is not a
// rule.Severity: a finding can never carry it, which is the point.
const off = "off"

// Config is a validated configuration.
//
// The zero value is the default configuration — every rule on, at its declared
// severity, over every discovered file — so a run with no config file behaves
// exactly as it did before configuration existed.
type Config struct {
	// Path is the file this was read from, or "" for the defaults.
	Path string

	// Severity overrides a rule's declared severity, by rule ID.
	Severity map[string]rule.Severity

	// Disabled names the rules that must not run.
	Disabled map[string]bool

	// Exclude holds the path patterns that drop a file from the run.
	Exclude []string
}

// file is the on-disk shape. It is separate from [Config] so that the TOML
// surface can be validated once, on the way in, and the rest of Plumbline only
// ever sees values that are known to be good.
type file struct {
	Exclude []string          `toml:"exclude"`
	Rules   map[string]string `toml:"rules"`
}

// Load reads path and validates it against reg.
//
// Rule IDs are checked against the registry, so a typo in a rule name is an
// error rather than a setting that silently does nothing — which is the
// failure mode that matters here: a misspelt `[rules]` key would look exactly
// like a rule that had been switched off successfully.
func Load(path string, reg *rule.Registry) (*Config, error) {
	var f file
	md, err := toml.DecodeFile(path, &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("%s: unknown setting %s", path, strings.Join(keys, ", "))
	}

	cfg := &Config{
		Path:     path,
		Severity: map[string]rule.Severity{},
		Disabled: map[string]bool{},
		Exclude:  f.Exclude,
	}
	for id, value := range f.Rules {
		if _, known := reg.Get(id); !known {
			return nil, fmt.Errorf("%s: [rules] %s: no such rule; run `plumbline --list-rules` to see them", path, id)
		}
		switch s := rule.Severity(value); {
		case value == off:
			cfg.Disabled[id] = true
		case s == rule.SeverityError, s == rule.SeverityWarning, s == rule.SeverityNote:
			cfg.Severity[id] = s
		default:
			return nil, fmt.Errorf("%s: [rules] %s = %q: expected \"error\", \"warning\", \"note\" or \"off\"", path, id, value)
		}
	}
	for _, pattern := range cfg.Exclude {
		if strings.TrimSpace(pattern) == "" {
			return nil, fmt.Errorf("%s: exclude: empty pattern", path)
		}
	}
	return cfg, nil
}

// Discover loads the configuration for a run rooted at dir.
//
// A missing file is not an error: Plumbline is meant to be useful the moment
// the action is added to a workflow, before anyone has written a config. It
// returns the default configuration and a Path of "".
func Discover(dir string, reg *rule.Registry) (*Config, error) {
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return Load(path, reg)
}

// Apply configures e.
//
// The dependency runs this way round — config knows about the engine, the
// engine knows nothing about config — so that rules and rule execution have no
// opinion about where their settings came from.
func (c *Config) Apply(e *engine.Engine) {
	for id, s := range c.Severity {
		e.SetSeverity(id, s)
	}
	for id := range c.Disabled {
		e.Disable(id)
	}
	if len(c.Exclude) > 0 {
		patterns := c.Exclude
		e.SetExcluded(func(path string) bool { return Excluded(patterns, path) })
	}
}
