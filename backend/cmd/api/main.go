package main

import (
	"log"
	"net/http"

	"github.com/V4T54L/goship/pkg/goship/server"
	"github.com/go-chi/chi/v5"
)

type PingMessage struct {
	Type string `json:"type"`
	Time string `json:"time"`
}

func main() {
	server := server.NewChiServer()
	server.AddDefaultMiddleware()
	server.AddPermissiveCORS()
	server.AddDefaultRoutes()

	r, ok := server.GetRouter().(*chi.Mux)
	if !ok {
		log.Fatal("Error obtaining the router")
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Watch Tower homepage"))
	})

	err := server.Run("8000")
	if err != nil {
		log.Println("Error when running the server: ", err)
	}
}
