package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func InitWebserver() error {
	return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("[INFO]", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func HandleWebserver(entries chan *LogEntry) {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	HandleMap(r.PathPrefix("/map").Subrouter(), entries)

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8001",
		Handler: r,
	}

	log.Printf("[INFO] Serving on http://localhost:%d", 8001)
	log.Fatalf("%s", l.ListenAndServe())
}
