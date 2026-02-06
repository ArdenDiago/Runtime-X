package docker

import (
	"testing"

	"runtimex/internal/core"
)

func TestValidateImageName(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"valid image", "golang", false},
		{"valid image with org", "library/python", false},
		{"empty image", "", true},
		{"image with whitespace", "golang test", true},
		{"image with path traversal", "golang/../secret", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageName(tt.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageName(%q) error = %v, wantErr %v", tt.image, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTagName(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"valid tag", "latest", false},
		{"valid semver", "1.25.5", false},
		{"empty tag", "", false}, // Empty is valid, defaults to latest
		{"tag with whitespace", "1.0 beta", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTagName(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTagName(%q) error = %v, wantErr %v", tt.tag, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCopyPath(t *testing.T) {
	tests := []struct {
		name     string
		copyPath core.CopyPath
		wantErrs int
	}{
		{"valid paths", core.CopyPath{Source: "./app", Destination: "/app"}, 0},
		{"empty source", core.CopyPath{Source: "", Destination: "/app"}, 1},
		{"empty destination", core.CopyPath{Source: "./app", Destination: ""}, 1},
		{"both empty", core.CopyPath{Source: "", Destination: ""}, 2},
		{"path traversal in source", core.CopyPath{Source: "../../../etc/passwd", Destination: "/app"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateCopyPath(tt.copyPath, 0)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateCopyPath() errors = %d, wantErrs %d", len(errs), tt.wantErrs)
			}
		})
	}
}

func TestValidateRunCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{"valid command", "pip install flask", false},
		{"valid apt command", "apt-get update && apt-get install -y curl", false},
		{"empty command", "", true},
		{"dangerous rm -rf /", "rm -rf /", true},
		{"fork bomb", ":(){:|:&};:", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRunCommand(tt.cmd, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRunCommand(%q) error = %v, wantErr %v", tt.cmd, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorkDir(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
		wantErr bool
	}{
		{"valid absolute path", "/app", false},
		{"nested path", "/usr/src/app", false},
		{"relative path", "app", true},
		{"path traversal", "/app/../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkDir(tt.workDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkDir(%q) error = %v, wantErr %v", tt.workDir, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDockerfileConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *core.DockerfileConfig
		wantValid bool
	}{
		{
			name: "valid config",
			config: &core.DockerfileConfig{
				Image:       "python",
				Tag:         "3.11",
				CopyPaths:   []core.CopyPath{{Source: "./app", Destination: "/app"}},
				RunCommands: []string{"pip install flask"},
				CmdCommand:  []string{"python", "app.py"},
				WorkDir:     "/app",
			},
			wantValid: true,
		},
		{
			name: "invalid - empty image",
			config: &core.DockerfileConfig{
				Image: "",
				Tag:   "latest",
			},
			wantValid: false,
		},
		{
			name: "invalid - dangerous command",
			config: &core.DockerfileConfig{
				Image:       "alpine",
				Tag:         "latest",
				RunCommands: []string{"rm -rf /"},
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDockerfileConfig(tt.config)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateDockerfileConfig() valid = %v, wantValid %v, errors = %v",
					result.Valid, tt.wantValid, result.Errors)
			}
		})
	}
}
