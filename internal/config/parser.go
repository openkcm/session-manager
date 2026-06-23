package config

import (
	"github.com/goccy/go-yaml"
)

// yamlParser implements a yamlParser parser.
type yamlParser struct{}

// Unmarshal parses the given YAML bytes.
func (yamlParser) Unmarshal(b []byte) (map[string]any, error) {
	var out map[string]any
	if err := yaml.UnmarshalWithOptions(b, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// Marshal marshals the given config map to YAML bytes.
func (yamlParser) Marshal(o map[string]any) ([]byte, error) {
	return yaml.Marshal(o)
}
