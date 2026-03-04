package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest_MergesSecretValues(t *testing.T) {
	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	app := `engine:
  language:
    name: go
    version: "1.23"
apis: []
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(app), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}

	secrets := `videosdk:
  api_key: ENC1
db:
  url:
    _default: ENC_DEV
    production: ENC_PROD
`
	if err := os.WriteFile(filepath.Join(tmp, "secret-values.yaml"), []byte(secrets), 0o644); err != nil {
		t.Fatalf("write secret-values: %v", err)
	}

	manifest, err := LoadManifest(appPath, "production")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if manifest.Secrets == nil {
		t.Fatal("expected secrets to be merged")
	}

	if got := manifest.Secrets.Envs["videosdk__api_key"]; got != "ENC1" {
		t.Fatalf("videosdk__api_key = %v", got)
	}

	dbURL, ok := manifest.Secrets.Envs["db__url"].(map[string]any)
	if !ok {
		t.Fatalf("db__url type = %T", manifest.Secrets.Envs["db__url"])
	}
	if dbURL["_default"] != "ENC_DEV" || dbURL["production"] != "ENC_PROD" {
		t.Fatalf("db__url = %#v", dbURL)
	}
}

func TestLoadManifest_NoSecretValues(t *testing.T) {
	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	app := `engine:
  language:
    name: go
    version: "1.23"
apis: []
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(app), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}

	manifest, err := LoadManifest(appPath, "dev")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if manifest.Secrets != nil {
		t.Fatalf("expected no secrets, got %#v", manifest.Secrets)
	}
}

func TestLoadManifest_InvalidSecretValuesRoot(t *testing.T) {
	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	app := `engine:
  language:
    name: go
    version: "1.23"
apis: []
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(app), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "secret-values.yaml"), []byte("- one\n- two\n"), 0o644); err != nil {
		t.Fatalf("write secret-values: %v", err)
	}

	if _, err := LoadManifest(appPath, "dev"); err == nil {
		t.Fatal("expected invalid root error")
	}
}

func TestLoadManifest_ReadAndUnmarshalErrors(t *testing.T) {
	if _, err := LoadManifest("/does/not/exist/app.yaml", "dev"); err == nil {
		t.Fatal("expected read manifest error")
	}

	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	if err := os.WriteFile(appPath, []byte(":\n"), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}
	if _, err := LoadManifest(appPath, "dev"); err == nil {
		t.Fatal("expected unmarshal manifest error")
	}
}

func TestMergeSecretValues_ReadAndUnmarshalErrors(t *testing.T) {
	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	if err := os.WriteFile(appPath, []byte("envs: {}\n"), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}

	// Make secret-values.yaml a directory to force read error.
	if err := os.Mkdir(filepath.Join(tmp, "secret-values.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir secret-values.yaml: %v", err)
	}
	if _, err := LoadManifest(appPath, "dev"); err == nil || !strings.Contains(err.Error(), "read secret-values.yaml") {
		t.Fatalf("expected read secret-values error, got: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(tmp, "secret-values.yaml")); err != nil {
		t.Fatalf("remove secret-values.yaml dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "secret-values.yaml"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad secret-values: %v", err)
	}
	if _, err := LoadManifest(appPath, "dev"); err == nil || !strings.Contains(err.Error(), "unmarshal secret-values.yaml") {
		t.Fatalf("expected unmarshal secret-values error, got: %v", err)
	}
}

func TestHelpersInLoad(t *testing.T) {
	out := map[string]any{}
	flattenSecretValues(out, "", "value", "dev")
	if len(out) != 0 {
		t.Fatalf("expected no values for empty prefix, got %#v", out)
	}

	flattenSecretValues(out, "plain", "value", "dev")
	if out["plain"] != "value" {
		t.Fatalf("plain flatten = %#v", out)
	}

	if !isEnvOverrideMap(map[string]any{"dev": 1}, "dev") {
		t.Fatal("expected env map by current env")
	}

	converted, ok := toStringMap(map[interface{}]any{"a": 1})
	if !ok || converted["a"] != 1 {
		t.Fatalf("toStringMap map[interface{}]any failed: %#v, %v", converted, ok)
	}

	if _, ok := toStringMap("nope"); ok {
		t.Fatal("expected toStringMap false for scalar")
	}
}

func TestMergeSecretValues_EmptyCases(t *testing.T) {
	tmp := t.TempDir()
	appPath := filepath.Join(tmp, "app.yaml")
	app := `engine:
  language:
    name: go
    version: "1.23"
apis: []
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(app), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}

	// empty file -> raw=nil branch
	if err := os.WriteFile(filepath.Join(tmp, "secret-values.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("write secret-values: %v", err)
	}
	manifest, err := LoadManifest(appPath, "dev")
	if err != nil {
		t.Fatalf("LoadManifest empty secret-values: %v", err)
	}
	if manifest.Secrets != nil {
		t.Fatalf("expected nil secrets for empty file, got %#v", manifest.Secrets)
	}

	// empty map -> len(flat)==0 branch
	if err := os.WriteFile(filepath.Join(tmp, "secret-values.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write secret-values map: %v", err)
	}
	manifest, err = LoadManifest(appPath, "dev")
	if err != nil {
		t.Fatalf("LoadManifest empty map: %v", err)
	}
	if manifest.Secrets != nil {
		t.Fatalf("expected nil secrets for empty map, got %#v", manifest.Secrets)
	}
}

func TestFlattenSecretValues_EnvOverrideImmediate(t *testing.T) {
	out := map[string]any{}
	flattenSecretValues(out, "", map[string]any{
		"db": map[string]any{
			"url": map[string]any{
				"_default": "a",
				"prod":     "b",
			},
		},
	}, "prod")

	v, ok := out["db__url"].(map[string]any)
	if !ok {
		t.Fatalf("db__url type = %T", out["db__url"])
	}
	if v["_default"] != "a" || v["prod"] != "b" {
		t.Fatalf("db__url = %#v", v)
	}

	out = map[string]any{}
	flattenSecretValues(out, "service", map[string]any{"_default": "x", "dev": "y"}, "dev")
	if _, ok := out["service"].(map[string]any); !ok {
		t.Fatalf("service should stay env-map, got %T", out["service"])
	}
}
