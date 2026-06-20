package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// StringSlice is a custom type that can be unmarshaled from either a YAML
// string or a YAML list of strings. This allows "assert: expr" (single)
// and "assert:\n  - expr1\n  - expr2" (multiple) to coexist.
type StringSlice []string

// UnmarshalYAML implements yaml.Unmarshaler so that a single string value
// is treated as a one-element slice, while a list is kept as-is.
func (s *StringSlice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case string:
		*s = []string{v}
	case []interface{}:
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = fmt.Sprintf("%v", item)
		}
		*s = items
	default:
		return fmt.Errorf("StringSlice: expected string or list, got %T", raw)
	}
	return nil
}

// Config is the root configuration for the test runner.
type Config struct {
	DSN      string                 `yaml:"dsn" json:"dsn"`
	Driver   string                 `yaml:"driver" json:"driver"`
	Cases    []CaseConfig           `yaml:"cases" json:"cases"`
	Globals  map[string]interface{} `yaml:"globals" json:"globals"`
	Setup    []string               `yaml:"setup" json:"setup"`
	Teardown []string               `yaml:"teardown" json:"teardown"`
}

// CaseConfig defines one test case.
type CaseConfig struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"desc" json:"desc"`
	Skip        bool                   `yaml:"skip" json:"skip"`
	Steps       []StepConfig           `yaml:"steps" json:"steps"`
	Cases       []CaseConfig           `yaml:"cases" json:"cases"` // nested subcases
	Setup       []string               `yaml:"setup" json:"setup"`
	Teardown    []string               `yaml:"teardown" json:"teardown"`
	BeforeAll   []string               `yaml:"before_all" json:"before_all"`
	AfterAll    []string               `yaml:"after_all" json:"after_all"`
	Vars        map[string]interface{} `yaml:"vars" json:"vars"`
	Retry       int                    `yaml:"retry" json:"retry"`
	Timeout     string                 `yaml:"timeout" json:"timeout"`
}

// StepConfig defines a single test step.
type StepConfig struct {
	Name        string         `yaml:"name" json:"name"`
	Query       string         `yaml:"query" json:"query"`
	Args        []interface{}  `yaml:"args" json:"args"`
	Assert      StringSlice    `yaml:"assert" json:"assert"`
	Assertions  []AssertConfig `yaml:"assertions" json:"assertions"`
	Expect      interface{}    `yaml:"expect" json:"expect"`
	ExpectRows  int            `yaml:"expect_rows" json:"expect_rows"`
	ExpectCols  []string       `yaml:"expect_cols" json:"expect_cols"`
	Skip        bool           `yaml:"skip" json:"skip"`
	Description string         `yaml:"desc" json:"desc"`
	Retry       int            `yaml:"retry" json:"retry"`
	Timeout     string         `yaml:"timeout" json:"timeout"`
}

// AssertConfig defines an assertion to run against query results.
type AssertConfig struct {
	Type   string      `yaml:"type" json:"type"`     // equals, not_equals, contains, gt, lt, matches, is_null, not_null
	Column string      `yaml:"column" json:"column"` // which column or "count"
	Value  interface{} `yaml:"value" json:"value"`
	Row    int         `yaml:"row" json:"row"` // 0-indexed row; -1 means all rows
}

// Load reads a YAML file and returns a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	// Simple $ENV replacement
	content := os.Expand(string(data), func(key string) string {
		return os.Getenv(key)
	})

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	// defaults
	if cfg.Driver == "" {
		cfg.Driver = "postgres"
	}
	if cfg.DSN == "" {
		cfg.DSN = os.Getenv("DATABASE_URL")
		if cfg.DSN == "" {
			cfg.DSN = "host=localhost port=5432 user=postgres dbname=postgres sslmode=disable"
		}
	}

	return &cfg, nil
}

// flattenCases normalizes nested cases into a flat list with dot-separated names.
func flattenCases(parent string, cases []CaseConfig) []CaseConfig {
	var out []CaseConfig
	for _, c := range cases {
		full := c.Name
		if parent != "" {
			full = parent + "." + full
		}
		c.Name = full
		if len(c.Cases) > 0 {
			sub := flattenCases(full, c.Cases)
			c.Cases = nil
			out = append(out, c)
			out = append(out, sub...)
		} else {
			c.Cases = nil
			out = append(out, c)
		}
	}
	return out
}

// Flatten returns a copy of the config with nested cases flattened.
func (c *Config) Flatten() *Config {
	out := *c
	out.Cases = flattenCases("", c.Cases)
	return &out
}

// GetDSNWithDriver returns driver name and DSN, resolving PostgreSQL aliases.
func (c *Config) GetDSNWithDriver() (string, string) {
	driver := strings.ToLower(c.Driver)
	dsn := c.DSN

	// Support pgx alias
	if driver == "pgx" || driver == "pgxpool" {
		driver = "postgres"
	}
	return driver, dsn
}
