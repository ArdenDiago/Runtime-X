package utility

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type DockerHubTagsResponse struct {
	Next    string `json:"next"`
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}

// GetDockerImageTags fetches all tags for a given official image
// Example image: "golang", "python", "nginx"
func GetDockerImageTags(image string, limit int) ([]string, error) {
	var tags []string

	url := fmt.Sprintf(
		"https://hub.docker.com/v2/repositories/library/%s/tags",
		image,
	)
	log.Println(url)
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
			return nil, fmt.Errorf("docker hub API returned %d", resp.StatusCode)
		}

		var data DockerHubTagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, tag := range data.Results {
			tags = append(tags, tag.Name)

			if limit > 0 && len(tags) >= limit {
				return tags, nil
			}
		}

		url = data.Next
	}

	return tags, nil
}
