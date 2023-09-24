package main

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/COSI-Lab/Mirror/config"
	"github.com/COSI-Lab/Mirror/logging"
	"github.com/gorilla/mux"
)

var tmpls *template.Template
var projects map[string]*config.Project
var projectsByID []config.Project
var projectsGrouped config.ProjectsGrouped
var tokens *config.Tokens
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
	err := tmpls.ExecuteTemplate(w, "map.gohtml", projectsByID)
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

// handleManualSyncs is a endpoint that allows a privileged user to manually cause a project to sync
// Access token is included in the query string. The http method is not considered.
//
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

		// Check if the token has permission for projectName
		t := tokens.GetToken(token)
		if t == nil {
			http.Error(w, "Invalid access token", http.StatusForbidden)
			return
		}

		// Check if the token has permission for projectName
		if !t.HasProject(projectName) {
			http.Error(w, "Invalid access token", http.StatusForbidden)
			return
		}

		// Return a success message
		fmt.Fprintf(w, "Sync requested for project: %s", projectName)

		// Sync the project
		logging.Info("Manual sync requested for project: _", projectName, "_")
		manual <- projectName
	}
}

// Always returns status OK with no other content
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// WebServerLoadConfig loads a new config file to define the projects
func WebServerLoadConfig(cfg *config.File, t *config.Tokens) {
	dataLock.Lock()
	projectsByID = cfg.GetProjects()
	projectsGrouped = cfg.GetProjectsByPage()
	projects = cfg.Projects
	tokens = t
	dataLock.Unlock()
}

// HandleWebServer starts the webserver and listens for incoming connections
// manual is a channel that project short names are sent down to manually trigger a projects rsync
// entries is a channel that contains log entries that are disabled by the mirror map
func HandleWebServer(manual chan<- string, entries <-chan NGINXLogEntry) {
	r := mux.NewRouter()

	// Setup the map
	r.HandleFunc("/map", handleMap)
	mapMessages := make(chan []byte)
	go entriesToMessages(entries, mapMessages)
	MapRouter(r.PathPrefix("/map").Subrouter(), mapMessages)

	// Handlers for the other pages
	// redirect / to /home
	r.Handle("/", http.RedirectHandler("/home", http.StatusTemporaryRedirect))
	r.HandleFunc("/home", handleHome)
	r.HandleFunc("/projects", handleProjects)
	r.HandleFunc("/history", handleHistory)
	r.HandleFunc("/sync/{project}", handleManualSyncs(manual))
	r.HandleFunc("/health", handleHealth)

	// Static files
	r.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir("static")).ServeHTTP(w, r)
	}))

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8012",
		Handler: r,
	}

	logging.Success("Serving on http://localhost:8012")
	go l.ListenAndServe()
}
