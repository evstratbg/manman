package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGenerateValues(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	manifest := `envs:
  LOG_LEVEL:
    _default: INFO
    dev: DEBUG
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	var out bytes.Buffer
	err := run([]string{"-app", appPath, "-mode", "generate-values"}, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "values.dev.yaml")); err != nil {
		t.Fatalf("generated values file: %v", err)
	}
	if !strings.Contains(out.String(), "generated") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunHelm(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	templates := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(filepath.Join(templates, "_default"), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	files := map[string]string{
		"dockerfile.tmpl":     "FROM {{.language}}:{{.version}}\n",
		"migration.yaml.tmpl": "{{.project_name}}\n",
		"api.yaml.tmpl":       "{{.name}} {{.replicas}} {{.port}}\n",
		"cronjob.yaml.tmpl":   "{{.name}} {{.schedule}} {{.concurrency}}\n",
		"worker.yaml.tmpl":    "{{.name}} {{.replicas}}\n",
		"tolerations.yaml":    "_default: []\n",
		"affinity.yaml":       "_default: {}\n",
	}
	for name, body := range files {
		full := filepath.Join(templates, "_default", name)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}

	manifest := `engine:
  language:
    name: go
    version: "1.23"
  additional_system_packages: []
apis:
  - name: api
    enabled: true
    replicas: 1
    port: 8080
    command: run
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	outFile := filepath.Join(tmpDir, "manifests.yaml")
	var out bytes.Buffer
	err := run([]string{
		"-app", appPath,
		"-templates", templates,
		"-mode", "helm",
		"-output", outFile,
		"-project-name", "demo",
	}, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "api 1 8080") {
		t.Fatalf("unexpected manifests: %s", string(data))
	}
}

func TestRunValidation(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-mode", "generate-values"}, &out); err == nil {
		t.Fatal("expected validation error")
	}
	if err := run([]string{"-app", "/missing", "-mode", "generate-values"}, &out); err == nil {
		t.Fatal("expected generate-values error")
	}
	if err := run([]string{"-app", "x", "-mode", "unknown"}, &out); err == nil {
		t.Fatal("expected unsupported mode error")
	}
	if err := run([]string{"-app", "x", "-mode", "helm"}, &out); err == nil {
		t.Fatal("expected templates required error")
	}
	if err := run([]string{"-app", "x", "-mode", "helm", "-bad-flag"}, &out); err == nil {
		t.Fatal("expected parse error for bad flag")
	}
}

func TestRunDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	templates := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(filepath.Join(templates, "_default"), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	files := map[string]string{
		"dockerfile.tmpl":  "FROM {{.language}}:{{.version}}\n",
		"tolerations.yaml": "_default: []\n",
		"affinity.yaml":    "_default: {}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(templates, "_default", name), []byte(body), 0o644); err != nil {
			t.Fatalf("write template: %v", err)
		}
	}
	manifest := `engine:
  language:
    name: python
    version: "3.11"
  additional_system_packages: []
apis: []
cronjobs: []
workers: []
db_migrations: []
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var out bytes.Buffer
	err = run([]string{
		"-app", appPath,
		"-templates", templates,
		"-mode", "dockerfile",
	}, &out)
	if err != nil {
		t.Fatalf("run dockerfile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "Dockerfile")); err != nil {
		t.Fatalf("expected default Dockerfile output: %v", err)
	}
}

func TestMainSuccessPath(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	manifest := `envs:
  LOG_LEVEL:
    _default: INFO
`
	if err := os.WriteFile(appPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write app.yaml: %v", err)
	}

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"manman", "-app", appPath, "-mode", "generate-values"}

	restoreExit := stubExit(t)
	defer restoreExit()
	code := callMainAndCaptureExit(t)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "values.yaml")); err != nil {
		t.Fatalf("expected values.yaml: %v", err)
	}
}

func TestMainExitCodeAndMainErrorPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := mainExitCode([]string{"-app", "/missing", "-mode", "generate-values"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "failed to generate values files") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"manman", "-app", "/missing", "-mode", "generate-values"}

	restoreExit := stubExit(t)
	defer restoreExit()
	code = callMainAndCaptureExit(t)
	if code != 1 {
		t.Fatalf("expected main exit code 1, got %d", code)
	}
}

var errExitCalled = errors.New("exit called")
var exitCodeCaptured int

func stubExit(t *testing.T) func() {
	t.Helper()
	old := exitFunc
	exitFunc = func(code int) {
		exitCodeCaptured = code
		if code != 0 && code != 1 {
			panic(errors.New("unexpected exit code"))
		}
		panic(errExitCalled)
	}
	return func() {
		exitFunc = old
	}
}

func TestMainExitCodeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(appPath, []byte("envs:\n  A:\n    _default: 1\n"), 0o644); err != nil {
		t.Fatalf("write app: %v", err)
	}
	code := mainExitCode([]string{"-app", appPath, "-mode", "generate-values"}, io.Discard, io.Discard)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}

func callMainAndCaptureExit(t *testing.T) (code int) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from stubbed exit")
		}
		if !errors.Is(r.(error), errExitCalled) {
			t.Fatalf("unexpected panic: %v", r)
		}
		code = exitCodeCaptured
	}()
	main()
	t.Fatal("expected main to call exit")
	return -1
}
