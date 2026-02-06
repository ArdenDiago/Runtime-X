package docker

import (
	"net/http"

	dockerfileRoutes "runtimex/cmd/api/docker/dockerfile"
	imageRoutes "runtimex/cmd/api/docker/image"
	tagRoutes "runtimex/cmd/api/docker/tags"
)

func Router() http.Handler {
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
		mux.HandleFunc("/dockerfile", dockerfileRoutes.CreateDockerfile)
		mux.HandleFunc("/dockerfile/", dockerfileRoutes.GetDockerfile)
		mux.ServeHTTP(w, r)
	})
}
