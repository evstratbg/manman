package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateValuesFiles_PerEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")

	manifest := `envs:
  LOG_LEVEL:
    _default: INFO
    dev: DEBUG
    production: WARN
apis:
  - name: api
    replicas:
      _default: 1
      production: 3
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	files, err := GenerateValuesFiles(appPath)
	if err != nil {
		t.Fatalf("GenerateValuesFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}

	dev := readYAMLMap(t, filepath.Join(tmpDir, "values.dev.yaml"))
	assertHasGeneratedHeader(t, filepath.Join(tmpDir, "values.dev.yaml"))
	if got := dev["envs"].(map[string]any)["LOG_LEVEL"]; got != "DEBUG" {
		t.Fatalf("dev LOG_LEVEL = %v, want DEBUG", got)
	}

	prod := readYAMLMap(t, filepath.Join(tmpDir, "values.production.yaml"))
	assertHasGeneratedHeader(t, filepath.Join(tmpDir, "values.production.yaml"))
	if got := prod["envs"].(map[string]any)["LOG_LEVEL"]; got != "WARN" {
		t.Fatalf("production LOG_LEVEL = %v, want WARN", got)
	}
	if got := prod["apis"].([]any)[0].(map[string]any)["replicas"]; got != 3 {
		t.Fatalf("production replicas = %v, want 3", got)
	}
}

func TestGenerateValuesFiles_DefaultOnly(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")

	manifest := `envs:
  LOG_LEVEL:
    _default: INFO
apis:
  - name: api
    replicas:
      _default: 1
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	files, err := GenerateValuesFiles(appPath)
	if err != nil {
		t.Fatalf("GenerateValuesFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d (%v)", len(files), files)
	}

	valuesPath := filepath.Join(tmpDir, "values.yaml")
	if files[0] != valuesPath {
		t.Fatalf("output = %s, want %s", files[0], valuesPath)
	}

	values := readYAMLMap(t, valuesPath)
	assertHasGeneratedHeader(t, valuesPath)
	if got := values["envs"].(map[string]any)["LOG_LEVEL"]; got != "INFO" {
		t.Fatalf("default LOG_LEVEL = %v, want INFO", got)
	}
	if got := values["apis"].([]any)[0].(map[string]any)["replicas"]; got != 1 {
		t.Fatalf("default replicas = %v, want 1", got)
	}
}

func TestGenerateValuesFiles_Errors(t *testing.T) {
	if _, err := GenerateValuesFiles("/does/not/exist/app.yaml"); err == nil {
		t.Fatal("expected read error")
	}

	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(appPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid app: %v", err)
	}
	if _, err := GenerateValuesFiles(appPath); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestNormalizeYAMLValue_MapAnyAndArray(t *testing.T) {
	in := map[any]any{
		"root": []any{
			map[any]any{"k": "v"},
		},
	}
	got := normalizeYAMLValue(in)
	want := map[string]any{
		"root": []any{
			map[string]any{"k": "v"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeYAMLValue = %#v, want %#v", got, want)
	}
}

func TestGenerateValuesFiles_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0o500); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o700)
	})

	appPath := filepath.Join(readOnlyDir, "app.yaml")
	manifest := `envs:
  LOG_LEVEL:
    _default: INFO
`
	// Need temporary write permission only for creating the app file.
	if err := os.Chmod(readOnlyDir, 0o700); err != nil {
		t.Fatalf("chmod writable: %v", err)
	}
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0o500); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}

	if _, err := GenerateValuesFiles(appPath); err == nil {
		t.Fatal("expected write error")
	}

	if err := os.Chmod(readOnlyDir, 0o700); err != nil {
		t.Fatalf("chmod writable: %v", err)
	}
	if err := os.WriteFile(appPath, []byte("envs:\n  LOG_LEVEL:\n    _default: INFO\n    dev: DEBUG\n"), 0o644); err != nil {
		t.Fatalf("rewrite app with envs: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0o500); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}
	if _, err := GenerateValuesFiles(appPath); err == nil {
		t.Fatal("expected write error for values.<env>.yaml path")
	}
}

func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return parsed
}

func assertHasGeneratedHeader(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.HasPrefix(string(data), generatedValuesHeader) {
		t.Fatalf("%s does not have generated header", path)
	}
}
