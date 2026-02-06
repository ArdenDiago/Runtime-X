package core

// DockerfileConfig represents the configuration for generating a Dockerfile
type DockerfileConfig struct {
	ID          string     `json:"id"`
	Image       string     `json:"image"`
	Tag         string     `json:"tag"`
	CopyPaths   []CopyPath `json:"copyPaths"`
	RunCommands []string   `json:"runCommands"`
	CmdCommand  []string   `json:"cmdCommand"`
	WorkDir     string     `json:"workDir,omitempty"`
	Status      JobStatus  `json:"status"`
	Error       string     `json:"error,omitempty"`
	FilePath    string     `json:"filePath,omitempty"`
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
