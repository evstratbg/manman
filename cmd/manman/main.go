package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"manman/internal/config"
	"manman/internal/generator"
)

var exitFunc = os.Exit

func main() {
	exitFunc(mainExitCode(os.Args[1:], os.Stdout, os.Stderr))
}

func mainExitCode(args []string, stdout io.Writer, stderr io.Writer) int {
	if err := run(args, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string, stdout io.Writer) error {
	var (
		appPath     string
		templateDir string
		mode        string
		output      string
		team        string
		currentEnv  string
		image       string
		projectName string
		projectID   string
		branchName  string
		commit      string
		release     string
		secretKey   string
	)

	fs := flag.NewFlagSet("manman", flag.ContinueOnError)
	fs.StringVar(&appPath, "app", "", "Path to app.yaml")
	fs.StringVar(&templateDir, "templates", "", "Path to templates directory (root that contains _default and team folders)")
	fs.StringVar(&mode, "mode", "", "What to generate: helm, dockerfile, generate-values")
	fs.StringVar(&output, "output", "", "Output file name (default: manifests.yaml or Dockerfile)")
	fs.StringVar(&team, "team", "_default", "Team name used to resolve template folders")
	fs.StringVar(&currentEnv, "env", "dev", "Current environment name (used for _default overrides)")
	fs.StringVar(&image, "image", "", "Container image for manifests generation")
	fs.StringVar(&projectName, "project-name", "", "Project name passed to templates")
	fs.StringVar(&projectID, "project-id", "", "Project id passed to templates")
	fs.StringVar(&branchName, "branch", "", "Branch name passed to templates")
	fs.StringVar(&commit, "commit", "", "Commit hash passed to templates")
	fs.StringVar(&release, "release", "local", "Release identifier used in templates")
	fs.StringVar(&secretKey, "secret-key", "", "Hex-encoded AES key used to decrypt secrets.envs")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if appPath == "" || mode == "" {
		return errors.New("app and mode are required")
	}

	mode = strings.ToLower(mode)

	if mode == "generate-values" {
		files, err := config.GenerateValuesFiles(appPath)
		if err != nil {
			return fmt.Errorf("failed to generate values files: %w", err)
		}
		for _, file := range files {
			fmt.Fprintf(stdout, "generated %s\n", file)
		}
		return nil
	}

	if templateDir == "" {
		return errors.New("templates is required")
	}

	if mode != "helm" && mode != "dockerfile" {
		return fmt.Errorf("unsupported mode %q, use helm, dockerfile or generate-values", mode)
	}

	manifest, err := config.LoadManifest(appPath, currentEnv)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	absTemplates, err := filepath.Abs(templateDir)
	if err != nil {
		return fmt.Errorf("resolve templates dir: %w", err)
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
		return fmt.Errorf("failed to init generator: %w", err)
	}

	var content string
	switch mode {
	case "helm":
		content, err = gen.GenerateManifests()
		if err != nil {
			return fmt.Errorf("failed to generate manifests: %w", err)
		}
		if output == "" {
			output = "manifests.yaml"
		}
	case "dockerfile":
		content, err = gen.GenerateDockerfile()
		if err != nil {
			return fmt.Errorf("failed to generate dockerfile: %w", err)
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
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Fprintf(stdout, "generated %s\n", output)
	return nil
}
