package utils

import "net/http"

type HttpHandlerFunc func(http.ResponseWriter, *http.Request) error

// Permet à HttpHandlerFunc d'implémenter http.Handler
func (h HttpHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h(w, r)
}

func ExecuteHandler(h http.Handler, w http.ResponseWriter, r *http.Request) error {
	if handler, ok := h.(HttpHandlerFunc); ok {
		return handler(w, r)
	}
	h.ServeHTTP(w, r)
	return nil
}
