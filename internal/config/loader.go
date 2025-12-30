package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sagernet/sing-box/option"
)

// Loader handles configuration file loading and validation
type Loader struct {
	path string
}

// NewLoader creates a new configuration loader
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Load reads and parses the configuration file
func (l *Loader) Load() (*option.Options, error) {
	// Check if file exists
	if _, err := os.Stat(l.path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", l.path)
	}

	// Read file content
	content, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse JSON
	var options option.Options
	if err := json.Unmarshal(content, &options); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Validate configuration
	if err := l.validate(&options); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &options, nil
}

// validate performs basic validation on the configuration
func (l *Loader) validate(opts *option.Options) error {
	// Check if at least one inbound is configured
	if len(opts.Inbounds) == 0 {
		return fmt.Errorf("no inbounds configured")
	}

	// Check if at least one outbound is configured
	if len(opts.Outbounds) == 0 {
		return fmt.Errorf("no outbounds configured")
	}

	return nil
}

// LoadFromFile is a convenience function to load configuration from a file
func LoadFromFile(path string) (*option.Options, error) {
	loader := NewLoader(path)
	return loader.Load()
}
