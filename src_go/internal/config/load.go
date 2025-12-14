package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads app.yaml content into Manifest struct.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest yaml: %w", err)
	}

	return &manifest, nil
}
