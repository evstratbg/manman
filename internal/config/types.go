package config

// PackageManager describes the package manager used in the project.
type PackageManager struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Language describes the programming language used in the project.
type Language struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Engine holds build specific configuration.
type Engine struct {
	Language                 Language       `yaml:"language"`
	AdditionalSystemPackages []string       `yaml:"additional_system_packages"`
	PackageManager           *PackageManager `yaml:"package_manager"`
}

// Requests captures per-environment resource requests.
type Requests struct {
	Memory any `yaml:"memory"`
	CPU    any `yaml:"cpu"`
}

// HPA defines autoscaling parameters.
type HPA struct {
	MinReplicas                 any `yaml:"min_replicas"`
	MaxReplicas                 any `yaml:"max_replicas"`
	TargetCPUUtilizationPercent any `yaml:"target_cpu_utilization_percent"`
}

// API describes a web server deployment.
type API struct {
	Command      string            `yaml:"command"`
	Name         string            `yaml:"name"`
	Enabled      any               `yaml:"enabled"`
	Replicas     any               `yaml:"replicas"`
	MemoryLimits any               `yaml:"memory_limits"`
	Requests     *Requests         `yaml:"requests"`
	Envs         map[string]any    `yaml:"envs"`
	HPA          *HPA              `yaml:"hpa"`
	Ingress      *Ingress          `yaml:"ingress"`
}

// Ingress describes optional ingress configuration for an API.
type Ingress struct {
	Domain        any `yaml:"domain"`
	ProxyBodySize any `yaml:"proxy-body-size"`
}

// Cronjob describes cron job configuration.
type Cronjob struct {
	Concurrency string         `yaml:"concurrency"`
	Command     string         `yaml:"command"`
	Name        string         `yaml:"name"`
	Enabled     any            `yaml:"enabled"`
	Schedule    any            `yaml:"schedule"`
	Envs        map[string]any `yaml:"envs"`
}

// Worker describes a background/queue worker configuration.
type Worker struct {
	Name         string            `yaml:"name"`
	Enabled      any               `yaml:"enabled"`
	Command      string            `yaml:"command"`
	Replicas     any               `yaml:"replicas"`
	Envs         map[string]any    `yaml:"envs"`
	MemoryLimits any               `yaml:"memory_limits"`
	Requests     *Requests         `yaml:"requests"`
}

// DatabaseMigration describes a single migration task.
type DatabaseMigration struct {
	Command any            `yaml:"command"`
	Envs    map[string]any `yaml:"envs"`
}

// Secrets holds encrypted secret values.
type Secrets struct {
	Envs map[string]any `yaml:"envs"`
}

// Manifest matches the layout of app.yaml.
type Manifest struct {
	Engine       Engine               `yaml:"engine"`
	Apis         []API                `yaml:"apis"`
	DBMigrations []DatabaseMigration   `yaml:"db_migrations"`
	Cronjobs     []Cronjob            `yaml:"cronjobs"`
	Workers      []Worker             `yaml:"workers"`
	Secrets      *Secrets             `yaml:"secrets"`
	Envs         map[string]any       `yaml:"envs"`
}
