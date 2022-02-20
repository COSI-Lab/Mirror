package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/COSI_Lab/Mirror/mirrormap"
	"github.com/gorilla/mux"
)

var tmpls *template.Template
var projects map[string]*Project
var projects_sorted []Project
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
	err := tmpls.ExecuteTemplate(w, "map.gohtml", projects_sorted)
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
	var buf bytes.Buffer
	err := tmpls.ExecuteTemplate(&buf, "statistics.gohtml", getPieChart())
	if err != nil {
		logging.Warn("handleStatistics;", err)
	}

	pat := regexp.MustCompile(`(__f__")|("__f__)|(__f__)`)
	content := pat.ReplaceAll(buf.Bytes(), []byte(""))

	w.Write(content)
}

type ProxyWriter struct {
	header http.Header
	buffer bytes.Buffer
	status int
}

func (p *ProxyWriter) Header() http.Header {
	return p.header
}

func (p *ProxyWriter) Write(bytes []byte) (int, error) {
	return p.buffer.Write(bytes)
}

func (p *ProxyWriter) WriteHeader(status int) {
	p.status = status
}

type CacheEntry struct {
	header http.Header
	body   []byte
	status int
	time   time.Time
}

func (c *CacheEntry) WriteTo(w http.ResponseWriter) (int, error) {
	header := w.Header()

	for k, v := range c.header {
		header[k] = v
	}

	if c.status != 0 {
		w.WriteHeader(c.status)
	}

	return w.Write(c.body)
}

// Caches the responses from the webserver
var cache = map[string]*CacheEntry{}
var cacheLock = &sync.RWMutex{}

func cachingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Check if the request is cached
		cacheLock.RLock()
		if entry, ok := cache[r.RequestURI]; ok && r.Method == "GET" {
			// Check that the cache entry is still valid
			if time.Since(entry.time) < time.Hour {
				// Write the cached response
				entry.WriteTo(w)
				cacheLock.RUnlock()
				logging.Info(r.Method, r.RequestURI, "in", time.Since(start), "; cached")
				return
			}
		}
		cacheLock.RUnlock()

		// Create a new response writer
		proxyWriter := &ProxyWriter{
			header: make(http.Header),
		}

		// If not, call the next handler
		next.ServeHTTP(proxyWriter, r)

		// Create the response cache entry
		entry := &CacheEntry{
			header: proxyWriter.header,
			body:   proxyWriter.buffer.Bytes(),
			status: proxyWriter.status,
			time:   time.Now(),
		}

		// Send the response to client
		go func() {
			// Cache the response
			cacheLock.Lock()
			cache[r.RequestURI] = entry
			cacheLock.Unlock()
		}()

		entry.WriteTo(w)
		logging.Info(r.Method, r.RequestURI, "in", time.Since(start))
	})
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

func InitWebserver() {
	// Load the templates with safeJS
	tmpls = template.Must(template.New("").Funcs(template.FuncMap{
		"safeJS": func(s interface{}) template.JS {
			return template.JS(fmt.Sprint(s))
		},
	}).ParseGlob("templates/*.gohtml"))

	logging.Info(tmpls.DefinedTemplates())
}

// Reload distributions and software arrays
func WebserverLoadConfig(config ConfigFile) {
	dataLock.Lock()
	distributions = config.GetDistributions()
	software = config.GetSoftware()
	projects_sorted = config.GetProjects()
	projects = config.Mirrors
	dataLock.Unlock()
}

func HandleWebserver(entries chan *LogEntry, status RSYNCStatus) {
	r := mux.NewRouter()

	cache = make(map[string]*CacheEntry)
	r.Use(cachingMiddleware)

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

	// Static files
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8012",
		Handler: r,
	}

	logging.Success("Serving on http://localhost:8012")
	log.Fatalf("%s", l.ListenAndServe())
}
