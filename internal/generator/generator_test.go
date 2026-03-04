package generator

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"manman/internal/config"
)

func TestNewAndGenerate(t *testing.T) {
	tmp := t.TempDir()
	templates := filepath.Join(tmp, "templates")
	mustMkdir(t, filepath.Join(templates, "_default"))

	templateFiles := map[string]string{
		"dockerfile.tmpl":     "FROM {{.language}}:{{.version}}\nRUN {{.package_manager}} {{.package_manager_version}} {{.additional_system_packages}}\n",
		"migration.yaml.tmpl": "mig {{.command}} {{index .envs \"GLOBAL\"}}\n",
		"api.yaml.tmpl":       "api {{.name}} {{.replicas}} {{.port}} {{index .envs \"GLOBAL\"}} {{index .envs \"LOCAL\"}} {{.current_env}}\n",
		"api_hpa.yaml.tmpl":   "hpa {{.min_replicas}} {{.max_replicas}} {{.target_cpu_utilization_percentage}}\n",
		"ingress.yaml.tmpl":   "ing {{.name}} {{.domain}} {{.proxy_body_size}}\n",
		"cronjob.yaml.tmpl":   "cron {{.name}} {{.schedule}} {{.concurrency}}\n",
		"worker.yaml.tmpl":    "worker {{.name}} {{.replicas}} {{.memory_requests}} {{.cpu_requests}}\n",
		"tolerations.yaml":    "_default: []\n",
		"affinity.yaml":       "_default: {}\n",
	}
	for name, body := range templateFiles {
		mustWrite(t, filepath.Join(templates, "_default", name), body)
	}

	manifest := &config.Manifest{
		Engine: config.Engine{
			Language:                 config.Language{Name: "Go", Version: "1.23"},
			AdditionalSystemPackages: []string{"curl"},
			PackageManager:           &config.PackageManager{Name: "go", Version: "1.23"},
		},
		Envs: map[string]any{
			"GLOBAL": map[string]any{"_default": "base", "production": "prod"},
		},
		DBMigrations: []config.DatabaseMigration{
			{Command: map[string]any{"_default": "migrate-default", "production": "migrate-prod"}},
		},
		Apis: []config.API{
			{
				Name:         "api",
				Command:      "run api",
				Enabled:      true,
				Replicas:     map[string]any{"_default": 1, "production": 3},
				Port:         8080,
				MemoryLimits: "256Mi",
				Requests: &config.Requests{
					Memory: "128Mi",
					CPU:    "100m",
				},
				Envs: map[string]any{
					"LOCAL": map[string]any{"_default": "d", "production": "p"},
				},
				HPA: &config.HPA{
					MinReplicas:                 2,
					MaxReplicas:                 5,
					TargetCPUUtilizationPercent: 70,
				},
				Ingress: &config.Ingress{
					Domain:        map[string]any{"_default": "", "production": "api.example.com"},
					ProxyBodySize: "32M",
				},
			},
			{Name: "disabled", Enabled: false},
		},
		Cronjobs: []config.Cronjob{
			{
				Name:        "job",
				Enabled:     "1",
				Schedule:    "*/5 * * * *",
				Concurrency: "forbid",
			},
		},
		Workers: []config.Worker{
			{
				Name:     "worker",
				Enabled:  1,
				Command:  "run worker",
				Replicas: 2,
				Requests: &config.Requests{Memory: "64Mi", CPU: "50m"},
			},
		},
	}

	g, err := New(Options{
		Manifest:     manifest,
		TemplatesDir: templates,
		Team:         "_default",
		CurrentEnv:   "production",
		Image:        "img",
		ProjectName:  "demo",
		BranchName:   "main",
		Commit:       "abc",
		Release:      "rel",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	dockerfile, err := g.GenerateDockerfile()
	if err != nil {
		t.Fatalf("GenerateDockerfile: %v", err)
	}
	if !strings.Contains(dockerfile, "FROM Go:1.23") {
		t.Fatalf("unexpected dockerfile: %s", dockerfile)
	}

	manifests, err := g.GenerateManifests()
	if err != nil {
		t.Fatalf("GenerateManifests: %v", err)
	}
	for _, expected := range []string{
		"mig migrate-prod prod",
		"api api 3 8080 prod p production",
		"hpa 2 5 70",
		"ing api api.example.com 32M",
		"cron job */5 * * * * Forbid",
		"worker worker 2 64Mi 50m",
	} {
		if !strings.Contains(manifests, expected) {
			t.Fatalf("missing %q in manifests:\n%s", expected, manifests)
		}
	}
	if strings.Contains(manifests, "disabled") {
		t.Fatalf("disabled api should not render: %s", manifests)
	}
}

func TestNewErrors(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error when manifest is nil")
	}

	manifest := &config.Manifest{
		Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
		Secrets: &config.Secrets{
			Envs: map[string]any{"TOKEN": "abc"},
		},
	}

	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "_default"))
	mustWrite(t, filepath.Join(tmp, "_default", "tolerations.yaml"), "_default: []\n")
	mustWrite(t, filepath.Join(tmp, "_default", "affinity.yaml"), "_default: {}\n")

	if _, err := New(Options{
		Manifest:     manifest,
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
	}); err == nil {
		t.Fatal("expected secret key error")
	}

	_, err := New(Options{
		Manifest:     manifest,
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
		SecretKey:    strings.Repeat("0", 32),
	})
	if err == nil || !strings.Contains(err.Error(), "decrypt secret") {
		t.Fatalf("expected decrypt error, got: %v", err)
	}
}

func TestGenerateDockerfileMissingTemplate(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "_default"))

	g, err := New(Options{
		Manifest: &config.Manifest{
			Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
		},
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := g.GenerateDockerfile(); err == nil {
		t.Fatal("expected missing dockerfile template error")
	}
}

func TestHelpers(t *testing.T) {
	if got := normalizeConcurrency(" allow "); got != "Allow" {
		t.Fatalf("normalizeConcurrency allow = %q", got)
	}
	if got := normalizeConcurrency("forbid"); got != "Forbid" {
		t.Fatalf("normalizeConcurrency forbid = %q", got)
	}
	if got := normalizeConcurrency("replace"); got != "Replace" {
		t.Fatalf("normalizeConcurrency replace = %q", got)
	}
	if got := normalizeConcurrency("custom"); got != "custom" {
		t.Fatalf("normalizeConcurrency custom = %q", got)
	}

	if !resolveBool(true, "dev") || !resolveBool("1", "dev") || !resolveBool(1, "dev") || !resolveBool(int64(1), "dev") {
		t.Fatal("resolveBool true cases failed")
	}
	if resolveBool("nope", "dev") || resolveBool(0, "dev") {
		t.Fatal("resolveBool false cases failed")
	}

	if got := resolveForEnv(map[string]any{"_default": "a", "prod": "b"}, "prod"); got != "b" {
		t.Fatalf("resolveForEnv map[string]any = %v", got)
	}
	if got := resolveForEnv(map[interface{}]any{"_default": 1}, "prod"); got != 1 {
		t.Fatalf("resolveForEnv map[interface{}]any = %v", got)
	}
	if got := resolveForEnv("x", "prod"); got != "x" {
		t.Fatalf("resolveForEnv scalar = %v", got)
	}

	merged := mergeMaps(map[string]any{"a": 1}, map[string]any{"a": 2, "b": 3})
	if merged["a"] != 2 || merged["b"] != 3 {
		t.Fatalf("mergeMaps = %#v", merged)
	}

	envMap := resolveEnvMap(map[string]any{
		"A": map[string]any{"_default": "x"},
		"B": map[string]any{"prod": "y"},
	}, "prod")
	if envMap["A"] != "x" || envMap["B"] != "y" {
		t.Fatalf("resolveEnvMap = %#v", envMap)
	}

	if resolveForEnv(map[string]any{"prod": "x"}, "dev") != nil {
		t.Fatal("expected nil when env/default missing")
	}
}

func TestGenerateManifestsErrors(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "_default"))
	mustWrite(t, filepath.Join(tmp, "_default", "tolerations.yaml"), "_default: []\n")
	mustWrite(t, filepath.Join(tmp, "_default", "affinity.yaml"), "_default: {}\n")
	mustWrite(t, filepath.Join(tmp, "_default", "dockerfile.tmpl"), "FROM x\n")

	manifest := &config.Manifest{
		Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
		DBMigrations: []config.DatabaseMigration{
			{Command: "migrate"},
		},
		Apis: []config.API{
			{Name: "api", Enabled: true},
		},
		Cronjobs: []config.Cronjob{
			{Name: "job", Enabled: true, Schedule: "* * * * *", Concurrency: "allow"},
		},
		Workers: []config.Worker{
			{Name: "worker", Enabled: true},
		},
	}

	g, err := New(Options{
		Manifest:     manifest,
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := g.GenerateManifests(); err == nil {
		t.Fatal("expected missing template error")
	}
}

func TestRenderAndLoadYAMLErrors(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "_default"))
	mustWrite(t, filepath.Join(tmp, "_default", "tolerations.yaml"), "_default: []\n")
	mustWrite(t, filepath.Join(tmp, "_default", "affinity.yaml"), "_default: {}\n")
	mustWrite(t, filepath.Join(tmp, "_default", "dockerfile.tmpl"), "FROM x\n")

	g, err := New(Options{
		Manifest: &config.Manifest{
			Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
		},
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Parse error in template.
	mustWrite(t, filepath.Join(tmp, "_default", "bad.tmpl"), "{{")
	if _, err := g.render(filepath.Join("_default", "bad.tmpl"), map[string]any{}); err == nil {
		t.Fatal("expected template parse error")
	}

	// Execute error in template.
	mustWrite(t, filepath.Join(tmp, "_default", "exec.tmpl"), "{{index . \"missing\" \"nested\"}}")
	if _, err := g.render(filepath.Join("_default", "exec.tmpl"), map[string]any{}); err == nil {
		t.Fatal("expected template execute error")
	}

	// read error for yaml (path is directory)
	if err := os.Mkdir(filepath.Join(tmp, "_default", "broken.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir broken.yaml: %v", err)
	}
	if _, err := g.loadYamlFromTemplateDirs("broken.yaml"); err == nil {
		t.Fatal("expected yaml read error")
	}

	// parse error for yaml
	mustWrite(t, filepath.Join(tmp, "_default", "invalid.yaml"), "{")
	if _, err := g.loadYamlFromTemplateDirs("invalid.yaml"); err == nil {
		t.Fatal("expected yaml parse error")
	}
}

func TestNewInitAffinityAndTolerationsErrors(t *testing.T) {
	baseManifest := &config.Manifest{
		Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
	}

	tmpTol := t.TempDir()
	mustMkdir(t, filepath.Join(tmpTol, "_default"))
	mustWrite(t, filepath.Join(tmpTol, "_default", "affinity.yaml"), "_default: {}\n")
	if err := os.Mkdir(filepath.Join(tmpTol, "_default", "tolerations.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir tolerations.yaml: %v", err)
	}
	if _, err := New(Options{
		Manifest:     baseManifest,
		TemplatesDir: tmpTol,
		Team:         "_default",
		CurrentEnv:   "dev",
	}); err == nil {
		t.Fatal("expected initTolerations error")
	}

	tmpAff := t.TempDir()
	mustMkdir(t, filepath.Join(tmpAff, "_default"))
	mustWrite(t, filepath.Join(tmpAff, "_default", "tolerations.yaml"), "_default: []\n")
	if err := os.Mkdir(filepath.Join(tmpAff, "_default", "affinity.yaml"), 0o755); err != nil {
		t.Fatalf("mkdir affinity.yaml: %v", err)
	}
	if _, err := New(Options{
		Manifest:     baseManifest,
		TemplatesDir: tmpAff,
		Team:         "_default",
		CurrentEnv:   "dev",
	}); err == nil {
		t.Fatal("expected initAffinity error")
	}
}

func TestRenderApisCronWorkersAdditionalBranches(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "_default"))
	mustWrite(t, filepath.Join(tmp, "_default", "api.yaml.tmpl"), "api {{.name}}\n")
	mustWrite(t, filepath.Join(tmp, "_default", "cronjob.yaml.tmpl"), "cron {{.concurrency}}\n")
	mustWrite(t, filepath.Join(tmp, "_default", "worker.yaml.tmpl"), "worker {{.name}}\n")
	mustWrite(t, filepath.Join(tmp, "_default", "migration.yaml.tmpl"), "mig\n")
	mustWrite(t, filepath.Join(tmp, "_default", "dockerfile.tmpl"), "FROM x\n")
	mustWrite(t, filepath.Join(tmp, "_default", "tolerations.yaml"), "_default: []\n")
	mustWrite(t, filepath.Join(tmp, "_default", "affinity.yaml"), "_default: {}\n")

	enc := mustEncryptPayload(t, []byte("0123456789abcdef"), "plain")
	g, err := New(Options{
		Manifest: &config.Manifest{
			Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
			Apis: []config.API{
				{
					Name:    "a",
					Enabled: true,
					HPA: &config.HPA{
						MinReplicas: 1, MaxReplicas: 2, TargetCPUUtilizationPercent: 60,
					},
					Ingress: &config.Ingress{
						Domain: map[string]any{"_default": ""},
					},
				},
			},
			Cronjobs: []config.Cronjob{
				{Name: "c", Enabled: true, Schedule: "* * * * *", Concurrency: ""},
			},
			Workers: []config.Worker{
				{Name: "w", Enabled: true},
				{Name: "w2", Enabled: int64(0)},
			},
			Secrets: &config.Secrets{
				Envs: map[string]any{
					"MISSING": map[string]any{"production": enc},
					"EXIST":   enc,
				},
			},
		},
		TemplatesDir: tmp,
		Team:         "_default",
		CurrentEnv:   "dev",
		SecretKey:    hex.EncodeToString([]byte("0123456789abcdef")),
	})
	if err != nil {
		t.Fatalf("New with decrypt success: %v", err)
	}

	apis, err := g.renderApis()
	if err != nil {
		t.Fatalf("renderApis: %v", err)
	}
	if len(apis) != 1 {
		t.Fatalf("expected single api manifest (no hpa/ingress templates), got %d", len(apis))
	}

	cron, err := g.renderCronjobs()
	if err != nil {
		t.Fatalf("renderCronjobs: %v", err)
	}
	if len(cron) != 1 {
		t.Fatalf("expected 1 cron manifest, got %d", len(cron))
	}

	workers, err := g.renderWorkers()
	if err != nil {
		t.Fatalf("renderWorkers: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected disabled worker filtered, got %d", len(workers))
	}
}

func TestGenerateManifests_ErrorStages(t *testing.T) {
	t.Run("apis", func(t *testing.T) {
		tmp := t.TempDir()
		writeBaseTemplates(t, tmp)
		mustWrite(t, filepath.Join(tmp, "_default", "api.yaml.tmpl"), "{{")

		g := mustNewBasicGenerator(t, tmp)
		g.manifest.Apis = []config.API{{Name: "a", Enabled: true}}
		if _, err := g.GenerateManifests(); err == nil {
			t.Fatal("expected apis stage error")
		}
	})

	t.Run("cronjobs", func(t *testing.T) {
		tmp := t.TempDir()
		writeBaseTemplates(t, tmp)
		mustWrite(t, filepath.Join(tmp, "_default", "cronjob.yaml.tmpl"), "{{")

		g := mustNewBasicGenerator(t, tmp)
		g.manifest.Cronjobs = []config.Cronjob{{Name: "c", Enabled: true, Schedule: "* * * * *", Concurrency: "allow"}}
		if _, err := g.GenerateManifests(); err == nil {
			t.Fatal("expected cronjobs stage error")
		}
	})

	t.Run("workers", func(t *testing.T) {
		tmp := t.TempDir()
		writeBaseTemplates(t, tmp)
		mustWrite(t, filepath.Join(tmp, "_default", "worker.yaml.tmpl"), "{{")

		g := mustNewBasicGenerator(t, tmp)
		g.manifest.Workers = []config.Worker{{Name: "w", Enabled: true}}
		if _, err := g.GenerateManifests(); err == nil {
			t.Fatal("expected workers stage error")
		}
	})
}

func TestRenderCronjobsAndResolveBoolInt64Zero(t *testing.T) {
	tmp := t.TempDir()
	writeBaseTemplates(t, tmp)
	mustWrite(t, filepath.Join(tmp, "_default", "cronjob.yaml.tmpl"), "cron {{.concurrency}}\n")

	g := mustNewBasicGenerator(t, tmp)
	g.manifest.Cronjobs = []config.Cronjob{
		{
			Name:        "c",
			Enabled:     true,
			Schedule:    "* * * * *",
			Concurrency: "",
		},
		{
			Name:        "disabled",
			Enabled:     false,
			Schedule:    "* * * * *",
			Concurrency: "allow",
		},
	}
	out, err := g.renderCronjobs()
	if err != nil {
		t.Fatalf("renderCronjobs: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 cronjob, got %d", len(out))
	}
	if !strings.Contains(out[0], "cron ") {
		t.Fatalf("expected cron output, got %q", out[0])
	}

	if resolveBool(int64(0), "dev") {
		t.Fatal("resolveBool int64(0) should be false")
	}
}

func mustEncryptPayload(t *testing.T, key []byte, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("rand iv: %v", err)
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	raw := make([]byte, 2+len(iv)+len(ciphertext))
	binary.LittleEndian.PutUint16(raw[:2], uint16(len(iv)))
	copy(raw[2:], iv)
	copy(raw[2+len(iv):], ciphertext)
	return hex.EncodeToString(raw)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	if padLen == 0 {
		padLen = blockSize
	}
	pad := make([]byte, padLen)
	for i := range pad {
		pad[i] = byte(padLen)
	}
	return append(data, pad...)
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWrite(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeBaseTemplates(t *testing.T, root string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, "_default"))
	mustWrite(t, filepath.Join(root, "_default", "dockerfile.tmpl"), "FROM x\n")
	mustWrite(t, filepath.Join(root, "_default", "migration.yaml.tmpl"), "mig\n")
	mustWrite(t, filepath.Join(root, "_default", "api.yaml.tmpl"), "api\n")
	mustWrite(t, filepath.Join(root, "_default", "cronjob.yaml.tmpl"), "cron\n")
	mustWrite(t, filepath.Join(root, "_default", "worker.yaml.tmpl"), "worker\n")
	mustWrite(t, filepath.Join(root, "_default", "tolerations.yaml"), "_default: []\n")
	mustWrite(t, filepath.Join(root, "_default", "affinity.yaml"), "_default: {}\n")
}

func mustNewBasicGenerator(t *testing.T, root string) *Generator {
	t.Helper()
	g, err := New(Options{
		Manifest: &config.Manifest{
			Engine: config.Engine{Language: config.Language{Name: "go", Version: "1.23"}},
		},
		TemplatesDir: root,
		Team:         "_default",
		CurrentEnv:   "dev",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}
