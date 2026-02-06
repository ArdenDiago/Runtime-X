package docker

import (
	"encoding/json"
	"net/http"
	"strings"

	"runtimex/cmd/api/docker/utility"
)

// GET /docker/images/{image}/tags
func GetImageTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract image name from URL
	// /docker/images/golang/tags → golang
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Accept /images/{image}/tags and /images/{image}/tags/
	if len(parts) < 3 || parts[0] != "images" || parts[2] != "tags" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	image := parts[1]

	tags, err := utility.GetDockerImageTags(image, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}
