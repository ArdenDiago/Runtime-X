package docker

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"runtimex/internal/core"
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidateDockerfileConfig validates all fields of a DockerfileConfig
func ValidateDockerfileConfig(config *core.DockerfileConfig) ValidationResult {
	var errors []ValidationError

	// Validate image
	if err := ValidateImageName(config.Image); err != nil {
		errors = append(errors, *err)
	}

	// Validate tag
	if err := ValidateTagName(config.Tag); err != nil {
		errors = append(errors, *err)
	}

	// Validate copy paths
	for i, cp := range config.CopyPaths {
		if errs := ValidateCopyPath(cp, i); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	// Validate run commands
	for i, cmd := range config.RunCommands {
		if err := ValidateRunCommand(cmd, i); err != nil {
			errors = append(errors, *err)
		}
	}

	// Validate CMD command
	if errs := ValidateCmdCommand(config.CmdCommand); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate workdir
	if config.WorkDir != "" {
		if err := ValidateWorkDir(config.WorkDir); err != nil {
			errors = append(errors, *err)
		}
	}

	return ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// ValidateImageName checks if image name is valid format
func ValidateImageName(image string) *ValidationError {
	if image == "" {
		return &ValidationError{Field: "image", Message: "image name is required"}
	}

	// Check for invalid characters
	if strings.ContainsAny(image, " \t\n\r") {
		return &ValidationError{Field: "image", Message: "image name cannot contain whitespace"}
	}

	// Check for path traversal
	if strings.Contains(image, "..") {
		return &ValidationError{Field: "image", Message: "image name cannot contain path traversal"}
	}

	return nil
}

// ValidateTagName checks if tag name is valid format
func ValidateTagName(tag string) *ValidationError {
	if tag == "" {
		return nil // Empty tag will default to "latest"
	}

	// Check for invalid characters
	if strings.ContainsAny(tag, " \t\n\r") {
		return &ValidationError{Field: "tag", Message: "tag cannot contain whitespace"}
	}

	// Check length (Docker Hub limit is 128)
	if len(tag) > 128 {
		return &ValidationError{Field: "tag", Message: "tag cannot exceed 128 characters"}
	}

	return nil
}

// ValidateCopyPath checks if source and destination paths are safe
func ValidateCopyPath(cp core.CopyPath, index int) []ValidationError {
	var errors []ValidationError
	field := fmt.Sprintf("copyPaths[%d]", index)

	// Validate source
	if cp.Source == "" {
		errors = append(errors, ValidationError{Field: field + ".source", Message: "source path is required"})
	} else {
		// Check for path traversal in source
		cleanSource := filepath.Clean(cp.Source)
		if strings.HasPrefix(cleanSource, "..") || strings.Contains(cleanSource, "/../") {
			errors = append(errors, ValidationError{Field: field + ".source", Message: "source path cannot contain path traversal"})
		}
	}

	// Validate destination
	if cp.Destination == "" {
		errors = append(errors, ValidationError{Field: field + ".destination", Message: "destination path is required"})
	} else {
		// Destination should be absolute in container
		if !strings.HasPrefix(cp.Destination, "/") && !strings.HasPrefix(cp.Destination, "./") {
			errors = append(errors, ValidationError{Field: field + ".destination", Message: "destination should be an absolute path or relative to WORKDIR"})
		}
	}

	return errors
}

// ValidateRunCommand checks if RUN command is safe
func ValidateRunCommand(cmd string, index int) *ValidationError {
	field := fmt.Sprintf("runCommands[%d]", index)

	if strings.TrimSpace(cmd) == "" {
		return &ValidationError{Field: field, Message: "RUN command cannot be empty"}
	}

	// Check for dangerous commands
	dangerousPatterns := []string{
		"rm -rf /",
		"rm -rf /*",
		":(){:|:&};:",    // Fork bomb
		"> /dev/sda",     // Disk wipe
		"mkfs.",          // Format
		"dd if=/dev/zero", // Disk fill
	}

	lowerCmd := strings.ToLower(cmd)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return &ValidationError{Field: field, Message: fmt.Sprintf("potentially dangerous command detected: %s", pattern)}
		}
	}

	return nil
}

// ValidateCmdCommand checks if CMD array is valid
func ValidateCmdCommand(cmd []string) []ValidationError {
	var errors []ValidationError

	if len(cmd) == 0 {
		return nil // CMD is optional
	}

	// Check first element (executable)
	if strings.TrimSpace(cmd[0]) == "" {
		errors = append(errors, ValidationError{Field: "cmdCommand[0]", Message: "executable cannot be empty"})
	}

	return errors
}

// ValidateWorkDir checks if WORKDIR is valid
func ValidateWorkDir(workDir string) *ValidationError {
	if !strings.HasPrefix(workDir, "/") {
		return &ValidationError{Field: "workDir", Message: "WORKDIR must be an absolute path"}
	}

	// Check for path traversal
	if strings.Contains(workDir, "..") {
		return &ValidationError{Field: "workDir", Message: "WORKDIR cannot contain path traversal"}
	}

	return nil
}

// ValidateImageExists checks if image exists on Docker Hub
func ValidateImageExists(image string) error {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/library/%s/", image)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("image '%s' not found on Docker Hub", image)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to verify image: status %d", resp.StatusCode)
	}

	return nil
}

// ValidateTagExists checks if tag exists for an image on Docker Hub
func ValidateTagExists(image, tag string) error {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/library/%s/tags/%s/", image, tag)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("tag '%s' not found for image '%s'", tag, image)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to verify tag: status %d", resp.StatusCode)
	}

	return nil
}
