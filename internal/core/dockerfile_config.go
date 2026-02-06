package core

// DockerfileConfig represents the configuration for generating a Dockerfile
type DockerfileConfig struct {
	ID          string            `json:"id"`
	Image       string            `json:"image"`
	Tag         string            `json:"tag"`
	GitRepo     string            `json:"gitRepo,omitempty"`     // GitHub/Git repository URL to clone
	GitBranch   string            `json:"gitBranch,omitempty"`   // Branch to clone (default: main)
	CopyPaths   []CopyPath        `json:"copyPaths"`
	EnvVars     map[string]string `json:"envVars,omitempty"`     // Environment variables
	RunCommands []string          `json:"runCommands"`
	CmdCommand  []string          `json:"cmdCommand"`
	WorkDir     string            `json:"workDir,omitempty"`
	ExposePort  int               `json:"exposePort,omitempty"`  // Port to expose
	Status      JobStatus         `json:"status"`
	Error       string            `json:"error,omitempty"`
	FilePath    string            `json:"filePath,omitempty"`
}

// CopyPath represents a source to destination mapping for COPY instruction
type CopyPath struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// DockerfileConfigStatus constants
const (
	ConfigPending   JobStatus = "PENDING"
	ConfigBuilding  JobStatus = "BUILDING"
	ConfigRunning   JobStatus = "RUNNING"
	ConfigCompleted JobStatus = "COMPLETED"
	ConfigFailed    JobStatus = "FAILED"
)

