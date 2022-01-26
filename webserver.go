package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

var tmpls *template.Template

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "index.gohtml", "")
}

func handleMap(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "map.gohtml", "")
}

func handleDistributions(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "distributions.gohtml", "chris")
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "history.gohtml", "")
}

func handleSoftware(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "software.gohtml", "")
}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "statistics.gohtml", "")
}

func InitWebserver() error {
	var err error
	tmpls, err = template.ParseGlob("templates/*")

	if err == nil {
		log.Println("[INFO] Webserver", tmpls.DefinedTemplates())
		return err
	} else {
		log.Println("\x1B[31m[Error]\x1B[0m InitWebserver", err)
		tmpls = nil
	}

	return nil
}

// Logs request Method and request URI
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("[INFO]", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func HandleWebserver(entries chan *LogEntry) {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.HandleFunc("/", handleIndex)
	MapRouter(r.PathPrefix("/map").Subrouter(), entries)

	r.HandleFunc("/map", handleMap)
	r.HandleFunc("/distributions", handleDistributions)
	r.HandleFunc("/software", handleSoftware)
	r.HandleFunc("/history", handleHistory)
	r.HandleFunc("/stats", handleStatistics)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8001",
		Handler: r,
	}

	log.Printf("[INFO] Serving on http://localhost:%d", 8001)
	log.Fatalf("%s", l.ListenAndServe())
}
