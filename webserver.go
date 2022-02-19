package main

import (
	"encoding/binary"
	"encoding/json"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"sync"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/COSI_Lab/Mirror/mirrormap"
	"github.com/gorilla/mux"
)

var tmpls *template.Template
var projects map[string]*Project
var distributions []Project
var software []Project
var dataLock = &sync.RWMutex{}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "index.gohtml", "")

	if err != nil {
		logging.Warn("handleIndex;", err)
	}
}

func handleMap(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	err := tmpls.ExecuteTemplate(w, "map.gohtml", projects)
	dataLock.RUnlock()

	if err != nil {
		logging.Warn("handleMap;", err)
	}
}

func handleDistributions(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	err := tmpls.ExecuteTemplate(w, "distributions.gohtml", distributions)
	dataLock.RUnlock()

	if err != nil {
		logging.Warn("handleDistributions;", err)
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "history.gohtml", "")

	if err != nil {
		logging.Warn("handleHistory;", err)
	}
}

func handleSoftware(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	err := tmpls.ExecuteTemplate(w, "software.gohtml", software)
	dataLock.RUnlock()
	if err != nil {
		logging.Warn("handleSoftware;", err)
	}
}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "statistics.gohtml", "")

	if err != nil {
		logging.Warn("handleStatistics;", err)
	}
}

func InitWebserver() error {
	var err error
	tmpls, err = template.ParseGlob("templates/*")

	if err == nil {
		logging.Info(tmpls.DefinedTemplates())
		return err
	} else {
		logging.Error("InitWebserver;", err)
		tmpls = nil
	}

	return nil
}

// Logs request Method and request URI
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Info(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// Setup distributions and software arrays
func webserverLoadConfig(config ConfigFile) {
	dataLock.Lock()
	distributions = config.GetDistributions()
	software = config.GetSoftware()
	projects = config.Mirrors
	dataLock.Unlock()
}

func entriesToMessages(entries chan *LogEntry, messages chan []byte) {
	// Track the previous IP to avoid sending duplicate data
	prevIP := net.IPv4(0, 0, 0, 0)

	for {
		// Read from the channel
		entry := <-entries

		// If the lookup failed, skip this entry
		if entry == nil || entry.City == nil {
			continue
		}

		// Skip if the IP is the same as the previous one
		if prevIP.Equal(entry.IP) {
			continue
		}

		// Update the previous IP
		prevIP = entry.IP

		// Create a new message
		long := entry.City.Location.Latitude
		lat := entry.City.Location.Longitude

		if long == 0 || lat == 0 {
			continue
		}

		// Get the distro
		project, ok := projects[entry.Distro]
		if !ok {
			continue
		}

		// Create a new message
		msg := make([]byte, 17)
		msg[0] = project.Id
		binary.LittleEndian.PutUint64(msg[1:9], math.Float64bits(long))
		binary.LittleEndian.PutUint64(msg[9:17], math.Float64bits(lat))

		messages <- msg
	}
}

func HandleWebserver(entries chan *LogEntry, status RSYNCStatus) {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Setup the map
	r.HandleFunc("/map", handleMap)
	mapMessages := make(chan []byte)
	go entriesToMessages(entries, mapMessages)
	mirrormap.MapRouter(r.PathPrefix("/map").Subrouter(), mapMessages)

	// Handlers for the other pages
	r.HandleFunc("/", handleIndex)
	r.HandleFunc("/distributions", handleDistributions)
	r.HandleFunc("/software", handleSoftware)
	r.HandleFunc("/history", handleHistory)
	r.HandleFunc("/stats", handleStatistics)

	// API subrouter
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/status/{short}", func(w http.ResponseWriter, r *http.Request) {
		dataLock.RLock()
		defer dataLock.RUnlock()

		vars := mux.Vars(r)
		short := vars["short"]

		s, ok := status[short]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Send the status as json
		json.NewEncoder(w).Encode(s.All())
	})

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8011",
		Handler: r,
	}

	logging.Success("Serving on http://localhost:8011")
	log.Fatalf("%s", l.ListenAndServe())
}
