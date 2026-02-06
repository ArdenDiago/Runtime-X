package utility

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DockerHubRepoResponse represents Docker Hub API response
type DockerHubRepoResponse struct {
	Count    int     `json:"count"`
	Next     string  `json:"next"`
	Previous *string `json:"previous"`
	Results  []struct {
		Name string `json:"name"`
	} `json:"results"`
}

// GetOfficialDockerImages fetches official Docker images
// Example results: python, golang, node, openjdk, nginx, redis
//
// limit = 0  → fetch all available official images
// limit > 0  → fetch only N images
func GetOfficialDockerImages(limit int) ([]string, error) {
	var images []string

	url := "https://hub.docker.com/v2/repositories/library/?page_size=100"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for url != "" {
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("docker hub API returned status %d", resp.StatusCode)
		}

		var data DockerHubRepoResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, repo := range data.Results {
			images = append(images, repo.Name)

			if limit > 0 && len(images) >= limit {
				return images, nil
			}
		}

		url = data.Next
	}

	return images, nil
}
