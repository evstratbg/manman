package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads app.yaml content into Manifest struct.
func LoadManifest(path string, currentEnv string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest yaml: %w", err)
	}

	manifest.Secrets = nil
	if err := mergeSecretValues(&manifest, path, currentEnv); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func mergeSecretValues(manifest *Manifest, appPath string, currentEnv string) error {
	secretPath := filepath.Join(filepath.Dir(appPath), "secret-values.yaml")
	data, err := os.ReadFile(secretPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read secret-values.yaml: %w", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal secret-values.yaml: %w", err)
	}
	if raw == nil {
		return nil
	}

	root, ok := toStringMap(raw)
	if !ok {
		return fmt.Errorf("secret-values.yaml: expected map at root")
	}

	flat := map[string]any{}
	flattenSecretValues(flat, "", root, currentEnv)
	if len(flat) == 0 {
		return nil
	}

	if manifest.Secrets == nil {
		manifest.Secrets = &Secrets{Envs: map[string]any{}}
	}
	if manifest.Secrets.Envs == nil {
		manifest.Secrets.Envs = map[string]any{}
	}
	for k, v := range flat {
		manifest.Secrets.Envs[k] = v
	}

	return nil
}

func flattenSecretValues(out map[string]any, prefix string, value any, currentEnv string) {
	m, ok := toStringMap(value)
	if !ok {
		if prefix != "" {
			out[prefix] = value
		}
		return
	}

	if prefix != "" && isEnvOverrideMap(m, currentEnv) {
		out[prefix] = m
		return
	}

	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "__" + k
		}
		if mv, ok := toStringMap(v); ok && isEnvOverrideMap(mv, currentEnv) {
			out[key] = mv
			continue
		}
		flattenSecretValues(out, key, v, currentEnv)
	}
}

func isEnvOverrideMap(m map[string]any, currentEnv string) bool {
	if _, ok := m["_default"]; ok {
		return true
	}
	if currentEnv != "" {
		if _, ok := m[currentEnv]; ok {
			return true
		}
	}
	return false
}

func toStringMap(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case map[string]any:
		return v, true
	case map[interface{}]any:
		out := map[string]any{}
		for key, val := range v {
			out[fmt.Sprint(key)] = val
		}
		return out, true
	default:
		return nil, false
	}
}
