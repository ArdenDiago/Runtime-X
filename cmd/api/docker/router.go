package docker

import (
	"net/http"

	dockerfileRoutes "runtimex/cmd/api/docker/dockerfile"
	imageRoutes "runtimex/cmd/api/docker/image"
	tagRoutes "runtimex/cmd/api/docker/tags"
	"runtimex/internal/queue"
)

// Router creates the docker routes handler
func Router() http.Handler {
	return RouterWithQueue(nil)
}

// RouterWithQueue creates the docker routes handler with an optional job queue
func RouterWithQueue(dockerQueue *queue.DockerJobQueue) http.Handler {
	dockerfileHandler := dockerfileRoutes.NewDockerfileHandler(dockerQueue)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /docker prefix
		path := r.URL.Path
		if len(path) >= 7 && path[:7] == "/docker" {
			path = path[7:]
			if path == "" {
				path = "/"
			}
		}
		r.URL.Path = path
		mux := http.NewServeMux()
		mux.HandleFunc("/images", imageRoutes.GetImages)
		mux.HandleFunc("/images/", tagRoutes.GetImageTags)
		mux.HandleFunc("/dockerfile", dockerfileHandler.CreateDockerfile)
		mux.HandleFunc("/dockerfile/", dockerfileHandler.GetDockerfile)
		mux.ServeHTTP(w, r)
	})
}

