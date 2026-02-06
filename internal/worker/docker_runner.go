package worker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"runtimex/internal/core"
	"runtimex/internal/logging"
)

// DockerRunner handles Docker container lifecycle for jobs
type DockerRunner struct {
	timeout time.Duration
}

// NewDockerRunner creates a new DockerRunner with specified timeout
func NewDockerRunner(timeout time.Duration) *DockerRunner {
	return &DockerRunner{timeout: timeout}
}

// RunDockerJob builds and runs a Docker container from a DockerfileConfig
func (dr *DockerRunner) RunDockerJob(config *core.DockerfileConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), dr.timeout)
	defer cancel()

	// Create a job for logging
	job := &core.Job{
		ID:        config.ID,
		Command:   fmt.Sprintf("docker build & run from %s", config.FilePath),
		Status:    core.StatusRunning,
		StartedAt: time.Now(),
	}
	config.Status = core.ConfigBuilding
	logging.LogJob(job)

	// Step 1: Build the Docker image
	imageName := "runtimex-job-" + config.ID
	if err := dr.buildImage(ctx, config, imageName); err != nil {
		config.Status = core.ConfigFailed
		config.Error = "build failed: " + err.Error()
		job.Status = core.StatusFailed
		job.Error = config.Error
		job.EndedAt = time.Now()
		logging.LogJob(job)
		return err
	}

	// Step 2: Run the container
	config.Status = core.ConfigRunning
	containerID, err := dr.runContainer(ctx, imageName)
	if err != nil {
		config.Status = core.ConfigFailed
		config.Error = "run failed: " + err.Error()
		job.Status = core.StatusFailed
		job.Error = config.Error
		job.EndedAt = time.Now()
		logging.LogJob(job)

		// Cleanup image on failure
		dr.cleanup(imageName, "")
		return err
	}

	// Step 3: Wait for container to complete and capture logs
	logs, exitCode, err := dr.waitAndGetLogs(ctx, containerID)
	if err != nil {
		config.Status = core.ConfigFailed
		config.Error = "execution failed: " + err.Error()
		job.Status = core.StatusFailed
		job.Error = config.Error
	} else if exitCode != 0 {
		config.Status = core.ConfigFailed
		config.Error = fmt.Sprintf("container exited with code %d: %s", exitCode, logs)
		job.Status = core.StatusFailed
		job.Error = config.Error
	} else {
		config.Status = core.ConfigCompleted
		job.Status = core.StatusCompleted
		log.Printf("Job %s completed successfully. Logs:\n%s", config.ID, logs)
	}

	job.EndedAt = time.Now()
	logging.LogJob(job)

	// Step 4: Cleanup
	dr.cleanup(imageName, containerID)

	return nil
}

// buildImage builds a Docker image from the Dockerfile
func (dr *DockerRunner) buildImage(ctx context.Context, config *core.DockerfileConfig, imageName string) error {
	// Get the directory containing the Dockerfile for build context
	dockerfileDir := filepath.Dir(config.FilePath)
	dockerfileName := filepath.Base(config.FilePath)

	// Build command
	cmd := exec.CommandContext(ctx, "docker", "build",
		"-t", imageName,
		"-f", dockerfileName,
		".",
	)
	cmd.Dir = dockerfileDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("Building Docker image: %s from %s", imageName, config.FilePath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w - %s", err, stderr.String())
	}

	log.Printf("Successfully built image: %s", imageName)
	return nil
}

// runContainer runs a Docker container from the built image
func (dr *DockerRunner) runContainer(ctx context.Context, imageName string) (string, error) {
	// Run container in detached mode
	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",           // Detached mode
		"--rm=false",   // Don't auto-remove (we need logs)
		imageName,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker run failed: %w - %s", err, stderr.String())
	}

	containerID := bytes.TrimSpace(stdout.Bytes())
	log.Printf("Container started: %s", string(containerID))
	return string(containerID), nil
}

// waitAndGetLogs waits for container to finish and returns logs
func (dr *DockerRunner) waitAndGetLogs(ctx context.Context, containerID string) (string, int, error) {
	// Wait for container to complete
	waitCmd := exec.CommandContext(ctx, "docker", "wait", containerID)
	var waitStdout, waitStderr bytes.Buffer
	waitCmd.Stdout = &waitStdout
	waitCmd.Stderr = &waitStderr

	if err := waitCmd.Run(); err != nil {
		return "", -1, fmt.Errorf("docker wait failed: %w - %s", err, waitStderr.String())
	}

	// Parse exit code
	exitCode := 0
	fmt.Sscanf(waitStdout.String(), "%d", &exitCode)

	// Get container logs
	logsCmd := exec.CommandContext(ctx, "docker", "logs", containerID)
	var logsStdout, logsStderr bytes.Buffer
	logsCmd.Stdout = &logsStdout
	logsCmd.Stderr = &logsStderr

	if err := logsCmd.Run(); err != nil {
		return "", exitCode, fmt.Errorf("docker logs failed: %w", err)
	}

	logs := logsStdout.String()
	if logsStderr.Len() > 0 {
		logs += "\n[STDERR]\n" + logsStderr.String()
	}

	return logs, exitCode, nil
}

// cleanup removes the container and image
func (dr *DockerRunner) cleanup(imageName, containerID string) {
	// Remove container if exists
	if containerID != "" {
		rmCmd := exec.Command("docker", "rm", "-f", containerID)
		if err := rmCmd.Run(); err != nil {
			log.Printf("Warning: failed to remove container %s: %v", containerID, err)
		} else {
			log.Printf("Removed container: %s", containerID)
		}
	}

	// Remove image
	rmiCmd := exec.Command("docker", "rmi", "-f", imageName)
	if err := rmiCmd.Run(); err != nil {
		log.Printf("Warning: failed to remove image %s: %v", imageName, err)
	} else {
		log.Printf("Removed image: %s", imageName)
	}
}

// Unused import prevention
var _ = os.Getenv
