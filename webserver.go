package main

import (
	"encoding/binary"
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
var projects []Project
var distributions []Project
var software []Project
var dataLock = &sync.RWMutex{}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "index.gohtml", "")

	if err != nil {
		logging.Log(logging.Warn, "handleIndex;", err)
	}
}

func handleMap(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	err := tmpls.ExecuteTemplate(w, "map.gohtml", projects)

	if err != nil {
		logging.Log(logging.Warn, "handleMap;", err)
	}
}

func handleDistributions(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	err := tmpls.ExecuteTemplate(w, "distributions.gohtml", distributions)

	if err != nil {
		logging.Log(logging.Warn, "handleDistributions;", err)
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "history.gohtml", "")

	if err != nil {
		logging.Log(logging.Warn, "handleHistory;", err)
	}
}

func handleSoftware(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	defer dataLock.RUnlock()

	err := tmpls.ExecuteTemplate(w, "software.gohtml", software)

	if err != nil {
		logging.Log(logging.Warn, "handleSoftware;", err)
	}
}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "statistics.gohtml", "")

	if err != nil {
		logging.Log(logging.Warn, "handleStatistics;", err)
	}
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
	defer dataLock.Unlock()

	distributions = make([]Project, 0, len(config.Mirrors))
	software = make([]Project, 0, len(config.Mirrors))

	for _, project := range config.Mirrors {
		if project.IsDistro {
			distributions = append(distributions, project)
		} else {
			software = append(software, project)
		}
	}

	projects = config.Mirrors
}

func entriesToMessages(shorts []string, entries chan *LogEntry, messages chan []byte) {
	// Create a map of dists and give them an id, hashing a map is quicker than an array
	distMap := make(map[string]byte)
	for i, dist := range shorts {
		distMap[dist] = byte(i)
	}

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
		distroByte, ok := distMap[entry.Distro]
		if !ok {
			continue
		}

		// Create a new message
		msg := make([]byte, 17)
		msg[0] = distroByte
		binary.LittleEndian.PutUint64(msg[1:9], math.Float64bits(long))
		binary.LittleEndian.PutUint64(msg[9:17], math.Float64bits(lat))

		messages <- msg
	}
}

func HandleWebserver(shorts []string, entries chan *LogEntry) {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Setup the map
	r.HandleFunc("/map", handleMap)
	mapMessages := make(chan []byte)
	go entriesToMessages(shorts, entries, mapMessages)
	mirrormap.MapRouter(r.PathPrefix("/map").Subrouter(), mapMessages)

	// Handlers for the other pages
	r.HandleFunc("/", handleIndex)
	r.HandleFunc("/distributions", handleDistributions)
	r.HandleFunc("/software", handleSoftware)
	r.HandleFunc("/history", handleHistory)
	r.HandleFunc("/stats", handleStatistics)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8010",
		Handler: r,
	}

	logging.Log(logging.Success, "Serving on http://localhost:8010")
	log.Fatalf("%s", l.ListenAndServe())
}
