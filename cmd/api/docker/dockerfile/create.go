package dockerfile

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"runtimex/internal/core"
	dockerValidator "runtimex/internal/docker"

	"github.com/google/uuid"
)

// CreateDockerfileRequest represents the request body for creating a Dockerfile
type CreateDockerfileRequest struct {
	Image           string          `json:"image"`
	Tag             string          `json:"tag"`
	CopyPaths       []core.CopyPath `json:"copyPaths"`
	RunCommands     []string        `json:"runCommands"`
	CmdCommand      []string        `json:"cmdCommand"`
	WorkDir         string          `json:"workDir,omitempty"`
	SkipHubValidation bool          `json:"skipHubValidation,omitempty"`
}

// CreateDockerfileResponse includes config and any validation warnings
type CreateDockerfileResponse struct {
	*core.DockerfileConfig
	Warnings []string `json:"warnings,omitempty"`
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

	// Default tag to latest
	if req.Tag == "" {
		req.Tag = "latest"
	}

	// Create config for validation
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

	// Validate config fields
	validationResult := dockerValidator.ValidateDockerfileConfig(config)
	if !validationResult.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "validation failed",
			"errors": validationResult.Errors,
		})
		return
	}

	// Collect warnings for non-critical issues
	var warnings []string

	// Validate image exists on Docker Hub (optional)
	if !req.SkipHubValidation {
		if err := dockerValidator.ValidateImageExists(config.Image); err != nil {
			warnings = append(warnings, "image validation: "+err.Error())
		} else {
			// Only check tag if image exists
			if err := dockerValidator.ValidateTagExists(config.Image, config.Tag); err != nil {
				warnings = append(warnings, "tag validation: "+err.Error())
			}
		}
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

	// Send response with warnings
	response := CreateDockerfileResponse{
		DockerfileConfig: config,
		Warnings:         warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

