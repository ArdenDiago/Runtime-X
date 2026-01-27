package docker

import "net/http"

// Router returns a handler for all /docker routes
func Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/docker/images", GetImages)
	// future:
	// mux.HandleFunc("/docker/tags", GetTags)

	return mux
}
