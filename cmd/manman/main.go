package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"manman/internal/config"
	"manman/internal/generator"
)

func main() {
	var (
		appPath      string
		templateDir  string
		mode         string
		output       string
		team         string
		currentEnv   string
		image        string
		projectName  string
		projectID    string
		branchName   string
		commit       string
		release      string
		secretKey    string
	)

	flag.StringVar(&appPath, "app", "", "Path to app.yaml")
	flag.StringVar(&templateDir, "templates", "", "Path to templates directory (root that contains _default and team folders)")
	flag.StringVar(&mode, "mode", "", "What to generate: helm or dockerfile")
	flag.StringVar(&output, "output", "", "Output file name (default: manifests.yaml or Dockerfile)")
	flag.StringVar(&team, "team", "_default", "Team name used to resolve template folders")
	flag.StringVar(&currentEnv, "env", "dev", "Current environment name (used for _default overrides)")
	flag.StringVar(&image, "image", "", "Container image for manifests generation")
	flag.StringVar(&projectName, "project-name", "", "Project name passed to templates")
	flag.StringVar(&projectID, "project-id", "", "Project id passed to templates")
	flag.StringVar(&branchName, "branch", "", "Branch name passed to templates")
	flag.StringVar(&commit, "commit", "", "Commit hash passed to templates")
	flag.StringVar(&release, "release", "local", "Release identifier used in templates")
	flag.StringVar(&secretKey, "secret-key", "", "Hex-encoded AES key used to decrypt secrets.envs")
	flag.Parse()

	if appPath == "" || templateDir == "" || mode == "" {
		flag.Usage()
		os.Exit(1)
	}

	mode = strings.ToLower(mode)
	if mode != "helm" && mode != "dockerfile" {
		log.Fatalf("unsupported mode %q, use helm or dockerfile", mode)
	}

	manifest, err := config.LoadManifest(appPath)
	if err != nil {
		log.Fatalf("failed to load manifest: %v", err)
	}

	absTemplates, err := filepath.Abs(templateDir)
	if err != nil {
		log.Fatalf("resolve templates dir: %v", err)
	}

	gen, err := generator.New(generator.Options{
		Manifest:     manifest,
		TemplatesDir: absTemplates,
		Team:         team,
		CurrentEnv:   currentEnv,
		Image:        image,
		ProjectName:  projectName,
		ProjectID:    projectID,
		BranchName:   branchName,
		Commit:       commit,
		Release:      release,
		SecretKey:    secretKey,
	})
	if err != nil {
		log.Fatalf("failed to init generator: %v", err)
	}

	var content string
	switch mode {
	case "helm":
		content, err = gen.GenerateManifests()
		if err != nil {
			log.Fatalf("failed to generate manifests: %v", err)
		}
		if output == "" {
			output = "manifests.yaml"
		}
	case "dockerfile":
		content, err = gen.GenerateDockerfile()
		if err != nil {
			log.Fatalf("failed to generate dockerfile: %v", err)
		}
		if output == "" {
			output = "Dockerfile"
		}
	}

	if !filepath.IsAbs(output) {
		cwd, _ := os.Getwd()
		output = filepath.Join(cwd, output)
	}

	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
		log.Fatalf("write output: %v", err)
	}

	fmt.Printf("generated %s\n", output)
}
