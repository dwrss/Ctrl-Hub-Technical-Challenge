package http

import "net/http"

func NewRouter(exposureHandler *ExposureHandler, summaryHandler *ExposureSummaryHandler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /exposure", exposureHandler.List)
	mux.HandleFunc("POST /exposure", exposureHandler.Create)
	mux.HandleFunc("GET /exposure/{exposureId}", exposureHandler.Get)
	mux.HandleFunc("GET /users/{userId}/exposure-summary", summaryHandler.Get)

	return mux
}
