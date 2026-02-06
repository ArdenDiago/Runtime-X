package dockerfile

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"runtimex/internal/core"

	"github.com/google/uuid"
)

// CreateDockerfileRequest represents the request body for creating a Dockerfile
type CreateDockerfileRequest struct {
	Image       string          `json:"image"`
	Tag         string          `json:"tag"`
	CopyPaths   []core.CopyPath `json:"copyPaths"`
	RunCommands []string        `json:"runCommands"`
	CmdCommand  []string        `json:"cmdCommand"`
	WorkDir     string          `json:"workDir,omitempty"`
}

// CreateDockerfile handles POST /docker/dockerfile
func CreateDockerfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateDockerfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}
	if req.Tag == "" {
		req.Tag = "latest"
	}

	// Create config
	config := &core.DockerfileConfig{
		ID:          uuid.New().String(),
		Image:       req.Image,
		Tag:         req.Tag,
		CopyPaths:   req.CopyPaths,
		RunCommands: req.RunCommands,
		CmdCommand:  req.CmdCommand,
		WorkDir:     req.WorkDir,
		Status:      core.ConfigPending,
	}

	// Generate Dockerfile content
	content := GenerateDockerfile(config)

	// Save to file
	filename := config.ID + ".dockerfile"
	filePath := filepath.Join("docker_files", filename)

	if err := os.MkdirAll("docker_files", 0755); err != nil {
		http.Error(w, "failed to create docker_files directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		http.Error(w, "failed to write Dockerfile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	config.FilePath = filePath

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// GetDockerfile handles GET /docker/dockerfile/{id}
func GetDockerfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := r.URL.Path
	// Path format: /dockerfile/{id}
	id := filepath.Base(path)
	if id == "" || id == "dockerfile" {
		http.Error(w, "dockerfile id is required", http.StatusBadRequest)
		return
	}

	// Try to read the dockerfile
	filePath := filepath.Join("docker_files", id+".dockerfile")
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "dockerfile not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to read dockerfile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

// GenerateDockerfile creates Dockerfile content from config
func GenerateDockerfile(config *core.DockerfileConfig) string {
	var content string

	// FROM instruction
	content += "FROM " + config.Image + ":" + config.Tag + "\n\n"

	// WORKDIR instruction (optional)
	if config.WorkDir != "" {
		content += "WORKDIR " + config.WorkDir + "\n\n"
	}

	// COPY instructions
	for _, cp := range config.CopyPaths {
		content += "COPY " + cp.Source + " " + cp.Destination + "\n"
	}
	if len(config.CopyPaths) > 0 {
		content += "\n"
	}

	// RUN instructions
	for _, cmd := range config.RunCommands {
		content += "RUN " + cmd + "\n"
	}
	if len(config.RunCommands) > 0 {
		content += "\n"
	}

	// CMD instruction
	if len(config.CmdCommand) > 0 {
		content += "CMD ["
		for i, arg := range config.CmdCommand {
			if i > 0 {
				content += ", "
			}
			content += "\"" + arg + "\""
		}
		content += "]\n"
	}

	return content
}

// Unused but needed to avoid import error
var _ = time.Now
