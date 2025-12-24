package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"manman/internal/config"
	"manman/internal/crypto"
)

// Options describes all knobs for rendering.
type Options struct {
	Manifest     *config.Manifest
	TemplatesDir string
	Team         string
	CurrentEnv   string
	Image        string
	ProjectName  string
	ProjectID    string
	BranchName   string
	Commit       string
	Release      string
	SecretKey    string
}

// Generator renders dockerfiles and manifests using Go text templates.
type Generator struct {
	manifest     *config.Manifest
	templateDir  string
	team         string
	language     string
	currentEnv   string
	image        string
	projectName  string
	projectID    string
	branchName   string
	commit       string
	release      string
	secretKey    string
	envs         map[string]any
	tolerations  any
	affinity     any
	templateDirs []string
	funcs        template.FuncMap
}

// New creates a ready-to-use generator instance.
func New(opts Options) (*Generator, error) {
	if opts.Manifest == nil {
		return nil, errors.New("manifest is required")
	}

	language := strings.ToLower(opts.Manifest.Engine.Language.Name)
	templateDirs := []string{
		"_default",
		filepath.Join(opts.Team, "_default"),
		filepath.Join(opts.Team, language),
	}

	funcs := template.FuncMap{
		"jsonify": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}

	g := &Generator{
		manifest:     opts.Manifest,
		templateDir:  opts.TemplatesDir,
		team:         opts.Team,
		language:     language,
		currentEnv:   opts.CurrentEnv,
		image:        opts.Image,
		projectName:  opts.ProjectName,
		projectID:    opts.ProjectID,
		branchName:   opts.BranchName,
		commit:       opts.Commit,
		release:      opts.Release,
		secretKey:    opts.SecretKey,
		templateDirs: templateDirs,
		funcs:        funcs,
	}

	if err := g.initEnvs(); err != nil {
		return nil, err
	}

	if err := g.initAffinity(); err != nil {
		return nil, err
	}

	if err := g.initTolerations(); err != nil {
		return nil, err
	}

	return g, nil
}

// GenerateDockerfile renders dockerfile using the dockerfile template.
func (g *Generator) GenerateDockerfile() (string, error) {
	path, err := g.findTemplate("dockerfile.tmpl")
	if err != nil {
		return "", err
	}

	pmName, pmVersion := "", ""
	if g.manifest.Engine.PackageManager != nil {
		pmName = g.manifest.Engine.PackageManager.Name
		pmVersion = g.manifest.Engine.PackageManager.Version
	}

	ctx := map[string]any{
		"language":                  g.manifest.Engine.Language.Name,
		"version":                   g.manifest.Engine.Language.Version,
		"additional_system_packages": strings.Join(g.manifest.Engine.AdditionalSystemPackages, " "),
		"package_manager":            pmName,
		"package_manager_version":    pmVersion,
	}

	return g.render(path, ctx)
}

// GenerateManifests renders all k8s manifests and returns a combined payload separated by "---".
func (g *Generator) GenerateManifests() (string, error) {
	var manifests []string

	migration, err := g.renderMigrations()
	if err != nil {
		return "", err
	}
	manifests = append(manifests, migration...)

	apis, err := g.renderApis()
	if err != nil {
		return "", err
	}
	manifests = append(manifests, apis...)

	cronjobs, err := g.renderCronjobs()
	if err != nil {
		return "", err
	}
	manifests = append(manifests, cronjobs...)

	workers, err := g.renderWorkers()
	if err != nil {
		return "", err
	}
	manifests = append(manifests, workers...)

	return strings.Join(manifests, "\n---\n"), nil
}

func (g *Generator) renderMigrations() ([]string, error) {
	templatePath, err := g.findTemplate("migration.yaml.tmpl")
	if err != nil {
		return nil, err
	}

	var manifests []string
	for _, migration := range g.manifest.DBMigrations {
		envs := mergeMaps(g.envs, resolveEnvMap(migration.Envs, g.currentEnv))
		ctx := map[string]any{
			"image":         g.image,
			"project_name":  g.projectName,
			"current_env":   g.currentEnv,
			"tolerations":   g.tolerations,
			"affinity":      g.affinity,
			"command":       resolveForEnv(migration.Command, g.currentEnv),
			"envs":          envs,
			"manman_release": g.release,
			"branch_name":    g.branchName,
			"commit":         g.commit,
			"team":           g.team,
		}

		out, err := g.render(templatePath, ctx)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, out)
	}

	return manifests, nil
}

func (g *Generator) renderApis() ([]string, error) {
	templatePath, err := g.findTemplate("api.yaml.tmpl")
	if err != nil {
		return nil, err
	}

	var hpaPath string
	hpaPath, err = g.findTemplate("api_hpa.yaml.tmpl")
	if err != nil {
		hpaPath = ""
	}

	var ingressPath string
	ingressPath, err = g.findTemplate("ingress.yaml.tmpl")
	if err != nil {
		ingressPath = ""
	}

	var manifests []string
	for _, api := range g.manifest.Apis {
		if !resolveBool(api.Enabled, g.currentEnv) {
			continue
		}

		envs := mergeMaps(g.envs, resolveEnvMap(api.Envs, g.currentEnv))
		requests := api.Requests
		var memoryReq any
		var cpuReq any
		if requests != nil {
			memoryReq = resolveForEnv(requests.Memory, g.currentEnv)
			cpuReq = resolveForEnv(requests.CPU, g.currentEnv)
		}

		ctx := map[string]any{
			"name":             api.Name,
			"command":          api.Command,
			"image":            g.image,
			"replicas":         resolveForEnv(api.Replicas, g.currentEnv),
			"port":             resolveForEnv(api.Port, g.currentEnv),
			"project_name":     g.projectName,
			"is_hpa_enabled":   api.HPA != nil,
			"current_env":      g.currentEnv,
			"tolerations":      g.tolerations,
			"affinity":         g.affinity,
			"envs":             envs,
			"memory_limits":    resolveForEnv(api.MemoryLimits, g.currentEnv),
			"memory_requests":  memoryReq,
			"cpu_requests":     cpuReq,
			"manman_release":   g.release,
			"branch_name":      g.branchName,
			"commit":           g.commit,
			"team":             g.team,
		}

		out, err := g.render(templatePath, ctx)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, out)

		if api.HPA != nil && hpaPath != "" {
			hpaCtx := map[string]any{
				"project_name":                  g.projectName,
				"min_replicas":                  resolveForEnv(api.HPA.MinReplicas, g.currentEnv),
				"max_replicas":                  resolveForEnv(api.HPA.MaxReplicas, g.currentEnv),
				"target_cpu_utilization_percentage": resolveForEnv(api.HPA.TargetCPUUtilizationPercent, g.currentEnv),
			}
			hpaOut, err := g.render(hpaPath, hpaCtx)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, hpaOut)
		}

		if api.Ingress != nil && ingressPath != "" {
			domain := resolveForEnv(api.Ingress.Domain, g.currentEnv)
			if domain != nil && fmt.Sprint(domain) != "" {
				ingressCtx := map[string]any{
					"name":             api.Name,
					"project_name":     g.projectName,
					"domain":           domain,
					"proxy_body_size":  resolveForEnv(api.Ingress.ProxyBodySize, g.currentEnv),
				}
				ingressOut, err := g.render(ingressPath, ingressCtx)
				if err != nil {
					return nil, err
				}
				manifests = append(manifests, ingressOut)
			}
		}
	}

	return manifests, nil
}

func (g *Generator) renderCronjobs() ([]string, error) {
	templatePath, err := g.findTemplate("cronjob.yaml.tmpl")
	if err != nil {
		return nil, err
	}

	var manifests []string
	for _, cronjob := range g.manifest.Cronjobs {
		if !resolveBool(cronjob.Enabled, g.currentEnv) {
			continue
		}

		envs := mergeMaps(g.envs, resolveEnvMap(cronjob.Envs, g.currentEnv))
		concurrency := normalizeConcurrency(fmt.Sprint(cronjob.Concurrency))
		if resolved := resolveForEnv(cronjob.Concurrency, g.currentEnv); resolved != nil {
			concurrency = normalizeConcurrency(fmt.Sprint(resolved))
		}

		ctx := map[string]any{
			"image":           g.image,
			"project_name":    g.projectName,
			"current_env":     g.currentEnv,
			"tolerations":     g.tolerations,
			"affinity":        g.affinity,
			"command":         cronjob.Command,
			"envs":            envs,
			"name":            cronjob.Name,
			"schedule":        resolveForEnv(cronjob.Schedule, g.currentEnv),
			"concurrency":     concurrency,
			"manman_release":  g.release,
			"branch_name":     g.branchName,
			"commit":          g.commit,
			"team":            g.team,
		}

		out, err := g.render(templatePath, ctx)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, out)
	}

	return manifests, nil
}

func normalizeConcurrency(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow":
		return "Allow"
	case "forbid":
		return "Forbid"
	case "replace":
		return "Replace"
	default:
		return value
	}
}

func (g *Generator) renderWorkers() ([]string, error) {
	templatePath, err := g.findTemplate("worker.yaml.tmpl")
	if err != nil {
		return nil, err
	}

	var manifests []string
	for _, worker := range g.manifest.Workers {
		if !resolveBool(worker.Enabled, g.currentEnv) {
			continue
		}

		envs := mergeMaps(g.envs, resolveEnvMap(worker.Envs, g.currentEnv))
		requests := worker.Requests
		var memoryReq any
		var cpuReq any
		if requests != nil {
			memoryReq = resolveForEnv(requests.Memory, g.currentEnv)
			cpuReq = resolveForEnv(requests.CPU, g.currentEnv)
		}

		ctx := map[string]any{
			"image":            g.image,
			"project_name":     g.projectName,
			"current_env":      g.currentEnv,
			"tolerations":      g.tolerations,
			"affinity":         g.affinity,
			"envs":             envs,
			"name":             worker.Name,
			"command":          worker.Command,
			"replicas":         resolveForEnv(worker.Replicas, g.currentEnv),
			"memory_limits":    resolveForEnv(worker.MemoryLimits, g.currentEnv),
			"memory_requests":  memoryReq,
			"cpu_requests":     cpuReq,
			"manman_release":   g.release,
			"branch_name":      g.branchName,
			"commit":           g.commit,
			"team":             g.team,
		}

		out, err := g.render(templatePath, ctx)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, out)
	}

	return manifests, nil
}

func (g *Generator) findTemplate(name string) (string, error) {
	var lastFound string
	for _, dir := range g.templateDirs {
		path := filepath.Join(dir, name)
		full := filepath.Join(g.templateDir, path)
		if _, err := os.Stat(full); err == nil {
			lastFound = path
		}
	}

	if lastFound == "" {
		return "", fmt.Errorf("no %s template found for team %q and language %q", name, g.team, g.language)
	}

	return lastFound, nil
}

func (g *Generator) render(path string, ctx map[string]any) (string, error) {
	full := filepath.Join(g.templateDir, path)
	tpl, err := template.New(filepath.Base(path)).Funcs(g.funcs).ParseFiles(full)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (g *Generator) initTolerations() error {
	value, err := g.loadYamlFromTemplateDirs("tolerations.yaml")
	if err != nil {
		return err
	}
	g.tolerations = resolveForEnv(value, g.currentEnv)
	return nil
}

func (g *Generator) initAffinity() error {
	value, err := g.loadYamlFromTemplateDirs("affinity.yaml")
	if err != nil {
		return err
	}
	g.affinity = resolveForEnv(value, g.currentEnv)
	return nil
}

func (g *Generator) initEnvs() error {
	envs := resolveEnvMap(g.manifest.Envs, g.currentEnv)
	envs["CURRENT_ENV"] = g.currentEnv
	if g.commit != "" {
		envs["COMMIT"] = g.commit
	}

	if g.manifest.Secrets != nil {
		if g.secretKey == "" {
			return errors.New("secret key is required to decrypt secrets")
		}
		for key, value := range g.manifest.Secrets.Envs {
			resolved := resolveForEnv(value, g.currentEnv)
			if resolved == nil {
				continue
			}
			plaintext, err := crypto.DecryptCBCPKCS7(g.secretKey, fmt.Sprint(resolved))
			if err != nil {
				return fmt.Errorf("decrypt secret %q: %w", key, err)
			}
			envs[key] = plaintext
		}
	}

	g.envs = envs
	return nil
}

func (g *Generator) loadYamlFromTemplateDirs(filename string) (any, error) {
	var last any
	for _, dir := range g.templateDirs {
		full := filepath.Join(g.templateDir, dir, filename)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", full, err)
		}
		var parsed any
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("parse %s: %w", full, err)
		}
		last = parsed
	}
	return last, nil
}

func mergeMaps(base map[string]any, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func resolveEnvMap(values map[string]any, env string) map[string]any {
	out := map[string]any{}
	for k, v := range values {
		resolved := resolveForEnv(v, env)
		if resolved != nil {
			out[k] = resolved
		}
	}
	return out
}

func resolveBool(value any, env string) bool {
	resolved := resolveForEnv(value, env)
	switch v := resolved.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(v) == "true" || v == "1"
	case int:
		return v != 0
	case int64:
		return v != 0
	}
	return false
}

func resolveForEnv(value any, env string) any {
	switch v := value.(type) {
	case map[string]any:
		if val, ok := v[env]; ok {
			return val
		}
		if val, ok := v["_default"]; ok {
			return val
		}
	case map[interface{}]any:
		converted := map[string]any{}
		for key, val := range v {
			keyStr := fmt.Sprint(key)
			converted[keyStr] = val
		}
		return resolveForEnv(converted, env)
	default:
		return v
	}
	return nil
}
