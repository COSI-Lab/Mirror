package main

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/COSI-Lab/logging"
	"github.com/gorilla/mux"
	"github.com/wcharczuk/go-chart/v2"
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

// The /stats page
func handleStats(w http.ResponseWriter, r *http.Request) {
	// get bar chart data
	line, err := QueryWeeklyNetStats()
	if err != nil {
		logging.Warn("handleStats;", err)
		return
	}

	err = tmpls.ExecuteTemplate(w, "statistics.gohtml", line)
	if err != nil {
		logging.Warn("handleStats;", err)
	}
}

// The /stats/{project}/{statistic} endpoint
// Supported statistics:
//   - daily_sent
func handleStatistics(w http.ResponseWriter, r *http.Request) {
	// Get the statistic name
	vars := mux.Vars(r)
	project := vars["project"]
	statistic := vars["statistic"]

	if project == "" || statistic == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch statistic {
	case "daily_sent":
		// Get the bar chart data
		stats, err := PrepareDailySendStats()
		if err != nil {
			logging.Warn("handleStatistics /daily_sent", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Create the bar chart for the to
		if data, ok := stats[project]; ok {
			graph := CreateBarChart(data, project)
			// render the chart as PNG
			err = graph.Render(chart.PNG, w)
			if err != nil {
				logging.Warn("handleStatistics /daily_sent", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "image/png")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// handleManualSyncs is a endpoint that allows a privileged user to manually cause a project to sync
// Access token is included in the query string. The http method is not considered.
// /sync/{project}?token={token}
func handleManualSyncs(manual chan<- string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if manual == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Get the project name
		vars := mux.Vars(r)
		projectName := vars["project"]

		// Get the access token
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "No token provided", http.StatusBadRequest)
			return
		}

		if projectName != "all" {
			// Get the project
			project, ok := projects[projectName]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if token == pullToken || token == project.AccessToken {
				// Return a success message
				fmt.Fprintf(w, "Sync requested for project: %s", projectName)

				// Sync the project
				logging.InfoToDiscord("**INFO** Manual sync requested for project: _", projectName, "_")
				manual <- projectName
			} else {
				http.Error(w, "Invalid access token", http.StatusForbidden)
			}
		} else {
			if token == pullToken {
				// Return a success message
				fmt.Fprintf(w, "Sync requested for all projects")

				// Sync the project
				logging.InfoToDiscord("**INFO** Manual sync requested for all projects")
				manual <- projectName
			} else {
				http.Error(w, "Invalid access token", http.StatusForbidden)
			}
		}
	}
}

// Always returns status OK with no other content
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Reload distributions and software arrays
func WebserverLoadConfig(config *ConfigFile) {
	dataLock.Lock()
	projectsById = config.GetProjects()
	projectsGrouped = config.GetProjectsByPage()
	projects = config.Mirrors
	dataLock.Unlock()
}

// HandleWebserver starts the webserver and listens for incoming connections
// manual is a channel that project short names are sent down to manually trigger a projects rsync
// entries is a channel that contains log entries that are disabled by the mirror map
func HandleWebserver(manual chan<- string, entries chan *NginxLogEntry) {
	r := mux.NewRouter()

	cache = make(map[string]*CacheEntry)

	// Setup the map
	r.Handle("/map", cachingMiddleware(handleMap))
	mapMessages := make(chan []byte)
	go entriesToMessages(entries, mapMessages)
	MapRouter(r.PathPrefix("/map").Subrouter(), mapMessages)

	// Handlers for the other pages
	// redirect / to /home
	r.Handle("/", http.RedirectHandler("/home", http.StatusTemporaryRedirect))
	r.Handle("/home", cachingMiddleware(handleHome))
	r.Handle("/projects", cachingMiddleware(handleProjects))
	r.Handle("/history", cachingMiddleware(handleHistory))
	r.Handle("/stats/{project}/{statistic}", cachingMiddleware(handleStatistics))
	r.Handle("/stats", cachingMiddleware(handleStats))
	r.Handle("/sync/{project}", handleManualSyncs(manual))
	r.HandleFunc("/health", handleHealth)
	r.HandleFunc("/ws", HandleWebsocket)

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
