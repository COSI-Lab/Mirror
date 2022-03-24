package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/COSI_Lab/Mirror/mirrormap"
	"github.com/gorilla/mux"
)

var tmpls *template.Template
var projects map[string]*Project
var projectsById []Project
var projectsGrouped ProjectsGrouped
var dataLock = &sync.RWMutex{}

func init() {
	// Load the templates with safeJS
	tmpls = template.Must(template.New("").Funcs(template.FuncMap{
		"safeJS": func(s interface{}) template.JS {
			return template.JS(fmt.Sprint(s))
		},
	}).ParseGlob("templates/*.gohtml"))

	logging.Info(tmpls.DefinedTemplates())
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "home.gohtml", "")

	if err != nil {
		logging.Warn("handleHome;", err)
	}
}

func handleMap(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	err := tmpls.ExecuteTemplate(w, "map.gohtml", projectsById)
	dataLock.RUnlock()

	if err != nil {
		logging.Warn("handleMap;", err)
	}
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "history.gohtml", "")

	if err != nil {
		logging.Warn("handleHistory;", err)
	}
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	dataLock.RLock()
	err := tmpls.ExecuteTemplate(w, "projects.gohtml", projectsGrouped)
	dataLock.RUnlock()
	if err != nil {
		logging.Warn("handleProjects,", projects, err)
	}
}

type LineChart struct {
	Sent  []float64
	Recv  []float64
	Times []int64
}

type StatsPage struct {
	Pie    map[string]int64
	Weekly LineChart
}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	// get pie chart data
	pie, err := QueryBytesSentByProject()

	if err != nil {
		logging.Warn("handleStatistics; failed to load pie data", err)
	}

	// get bar chart data
	sent, recv, times, nil := QueryWeeklyNetStats()

	if err != nil {
		logging.Warn("handleStatistics; failed to load bar data", err)
	}

	page := StatsPage{
		Pie: pie,
		Weekly: LineChart{
			sent,
			recv,
			times,
		},
	}

	err = tmpls.ExecuteTemplate(w, "statistics.gohtml", page)

	if err != nil {
		logging.Warn("handleStatistics;", err)
	}
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

func cachingMiddleware(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	if !webServerCache {
		logging.Info("Caching disabled")
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logging.Info(r.Method, r.URL.Path)
			next(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Check if the request is cached
		cacheLock.RLock()
		if entry, ok := cache[r.RequestURI]; ok && r.Method == "GET" {
			// Check that the cache entry is still valid
			if time.Since(entry.time) < time.Hour {
				// Send the cached response
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

		// Call the next handler
		next(proxyWriter, r)

		// Create the response cache entry
		entry := &CacheEntry{
			header: proxyWriter.header,
			body:   proxyWriter.buffer.Bytes(),
			status: proxyWriter.status,
			time:   time.Now(),
		}

		// Cache the response
		go func() {
			cacheLock.Lock()
			cache[r.RequestURI] = entry
			cacheLock.Unlock()
		}()

		// Send the response to client
		entry.WriteTo(w)
		logging.Info(r.Method, r.RequestURI, "in", time.Since(start))
	})
}

func entriesToMessages(entries chan *LogEntry, messages chan []byte) {
	// Send groups of 8 messages
	ch := make(chan []byte)
	go func() {
		for {
			group := make([]byte, 0, 40)
			for i := 0; i < 8; i++ {
				group = append(group, <-ch...)
			}
			messages <- group
		}
	}()

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

		// Get the distro
		project, ok := projects[entry.Distro]
		if !ok {
			continue
		}

		// Get the location
		lat_ := entry.City.Location.Latitude
		long_ := entry.City.Location.Longitude

		if lat_ == 0 && long_ == 0 {
			continue
		}

		// convert [-90, 90] latitude to [0, 4096] pixels
		lat := int16((lat_ + 90) * 4096 / 180)
		// convert [-180, 180] longitude to [0, 4096] pixels
		long := int16((long_ + 180) * 4096 / 360)

		// Create a new message
		msg := make([]byte, 5)
		// First byte is the project ID
		msg[0] = project.Id
		// Second and Third byte are the latitude
		msg[1] = byte(lat >> 8)
		msg[2] = byte(lat & 0xFF)
		// Fourth and Fifth byte are the longitude
		msg[3] = byte(long >> 8)
		msg[4] = byte(long & 0xFF)

		ch <- msg
	}
}

// Reload distributions and software arrays
func WebserverLoadConfig(config *ConfigFile) {
	dataLock.Lock()
	projectsById = config.GetProjects()
	projectsGrouped = config.GetProjectsByPage()
	projects = config.Mirrors
	dataLock.Unlock()
}

func HandleWebserver(entries chan *LogEntry) {
	r := mux.NewRouter()

	cache = make(map[string]*CacheEntry)

	// Setup the map
	r.Handle("/map", cachingMiddleware(handleMap))
	mapMessages := make(chan []byte)
	go entriesToMessages(entries, mapMessages)
	mirrormap.MapRouter(r.PathPrefix("/map").Subrouter(), mapMessages)

	// Handlers for the other pages
	// redirect / to /home
	r.Handle("/", http.RedirectHandler("/home", http.StatusTemporaryRedirect))
	r.Handle("/home", cachingMiddleware(handleHome))
	r.Handle("/projects", cachingMiddleware(handleProjects))
	r.Handle("/history", cachingMiddleware(handleHistory))
	r.Handle("/stats", cachingMiddleware(handleStatistics))

	// Static files
	r.PathPrefix("/").Handler(cachingMiddleware(http.FileServer(http.Dir("static")).ServeHTTP))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8012",
		Handler: r,
	}

	logging.Success("Serving on http://localhost:8012")
	go l.ListenAndServe()
}
