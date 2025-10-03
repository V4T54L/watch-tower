package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r *chi.Mux) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Watch Tower homepage"))
	})
}
