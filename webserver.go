package main

import (
	"html/template"
	"log"
	"net/http"
	"sync"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/gorilla/mux"
)

var tmpls *template.Template
var distributions []Project
var software []Project
var dataLock = &sync.RWMutex{}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "index.gohtml", "")
}

func handleMap(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "map.gohtml", "")
}

func handleDistributions(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	tmpls.ExecuteTemplate(w, "distributions.gohtml", distributions)
	dataLock.RUnlock()
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "history.gohtml", "")
}

func handleSoftware(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	tmpls.ExecuteTemplate(w, "software.gohtml", software)
	dataLock.RUnlock()
}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	tmpls.ExecuteTemplate(w, "statistics.gohtml", "")
}

func InitWebserver() error {
	var err error
	tmpls, err = template.ParseGlob("templates/*")

	if err == nil {
		logging.Log(logging.Info, tmpls.DefinedTemplates())
		return err
	} else {
		logging.Log(logging.Error, "InitWebserver;", err)
		tmpls = nil
	}

	return nil
}

// Logs request Method and request URI
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Log(logging.Info, r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// Setup distributions and software arrays
func webserverLoadConfig(config ConfigFile) {
	dataLock.Lock()
	distributions = make([]Project, 0, len(config.Mirrors))
	software = make([]Project, 0, len(config.Mirrors))

	for _, project := range config.Mirrors {
		if project.IsDistro {
			distributions = append(distributions, project)
		} else {
			software = append(software, project)
		}
	}
	dataLock.Unlock()
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

	logging.Log(logging.Success, "Serving on http://localhost:8001")
	log.Fatalf("%s", l.ListenAndServe())
}
