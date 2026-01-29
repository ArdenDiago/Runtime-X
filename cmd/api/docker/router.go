package docker

import (
	"net/http"

	imageRoutes "runtimex/cmd/api/docker/image"
	tagRoutes "runtimex/cmd/api/docker/tags"
)

func Router() http.Handler {
	mux := http.NewServeMux()

	// IMPORTANT: NO /docker prefix here
	mux.HandleFunc("/images", imageRoutes.GetImages)
	mux.HandleFunc("/images/", tagRoutes.GetImageTags)

	return mux
}
