package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/xeipuuv/gojsonschema"
)

type ConfigFile struct {
	Schema  string              `json:"$schema"`
	Mirrors map[string]*Project `json:"mirrors"`
}

// Returns a slice of all projects sorted by id
func (config *ConfigFile) GetProjects() []Project {
	var projects []Project

	for _, project := range config.Mirrors {
		projects = append(projects, *project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Id < projects[j].Id
	})

	return projects
}

type ProjectsGrouped struct {
	Distributions []Project
	Software      []Project
	Miscellaneous []Project
}

// Returns 3 slices of projects grouped by Page and sorted by Human name
func (config *ConfigFile) GetProjectsByPage() ProjectsGrouped {
	// "Distributions", "Software", "Miscellaneous"
	var distributions, software, misc []Project

	for _, project := range config.GetProjects() {
		switch project.Page {
		case "Distributions":
			distributions = append(distributions, project)
		case "Software":
			software = append(software, project)
		case "Miscellaneous":
			misc = append(misc, project)
		}
	}

	sort.Slice(distributions, func(i, j int) bool {
		return strings.ToLower(distributions[i].Name) < strings.ToLower(distributions[j].Name)
	})

	sort.Slice(software, func(i, j int) bool {
		return strings.ToLower(software[i].Name) < strings.ToLower(software[j].Name)
	})

	sort.Slice(misc, func(i, j int) bool {
		return strings.ToLower(misc[i].Name) < strings.ToLower(misc[j].Name)
	})

	return ProjectsGrouped{
		Distributions: distributions,
		Software:      software,
		Miscellaneous: misc,
	}
}

type Project struct {
	Name      string `json:"name"`
	Short     string // Copied from key
	Id        byte   // Id is given out in alphabetical order of short (only 255 are supported)
	SyncStyle string // "script" "rsync" or "static"
	Script    struct {
		Command     string `json:"command"`
		SyncsPerDay int    `json:"syncs_per_day"`
	}
	Rsync struct {
		Options      string `json:"options"` // cmdline options for first stage
		Second       string `json:"second"`  // cmdline options for second stage
		Third        string `json:"third"`   // cmdline options for third stage
		User         string `json:"user"`
		Host         string `json:"host"`
		Src          string `json:"src"`
		Dest         string `json:"dest"`
		SyncFile     string `json:"sync_file"`
		SyncsPerDay  int    `json:"syncs_per_day"`
		PasswordFile string `json:"password_file"`
		Password     string // Loaded from password file
	} `json:"rsync"`
	Static struct {
		Location string `json:"location"`
		Source   string `json:"source"`
	} `json:"static"`
	Color       string `json:"color"`
	Official    bool   `json:"official"`
	Page        string `json:"page"`
	HomePage    string `json:"homepage"`
	PublicRsync bool   `json:"publicRsync"`
	AccessToken string // Loaded from the access tokens file
}

func ParseConfig(configFile, schemaFile, tokensFile string) (config ConfigFile) {
	// Parse the schema file
	schemaBytes, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		log.Fatal("Could not read schema file: ", configFile, err.Error())
	}
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)

	// Parse the config file
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("Could not read config file: ", configFile, err.Error())
	}
	documentLoader := gojsonschema.NewBytesLoader(configBytes)

	// Validate the config against the schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Report errors
	if !result.Valid() {
		fmt.Printf("The document is not valid. see errors :\n")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
		os.Exit(1)
	}

	// Finally parse the config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("Could not parse the config file even though it fits the schema file: ", err.Error())
	}

	// Parse passwords & copy key as short & determine style
	var i uint8 = 0
	for short, project := range config.Mirrors {
		if project.Rsync.Dest != "" {
			project.SyncStyle = "rsync"
		} else if project.Static.Location != "" {
			project.SyncStyle = "static"
		} else {
			project.SyncStyle = "script"
		}

		if project.Rsync.PasswordFile != "" {
			project.Rsync.Password = getPassword("configs/" + project.Rsync.PasswordFile)
		}
		project.Short = short
		project.Id = i

		// add 1 and check for overflow
		if i == 255 {
			log.Fatal("Too many projects, 255 is the maximum because of the live map")
		}
		i++
	}

	// Parse access tokens
	if tokensFile != "" {
		// Read line by line
		file, err := os.Open(tokensFile)
		if err != nil {
			log.Fatal("Could not open access tokens file: ", tokensFile, err.Error())
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			// The format is "project:token" and we ignore lines that don't match
			reg := regexp.MustCompile(`^([^:]+):([^:]+)$`)
			if reg.MatchString(line) {
				// Get the project name and the token
				projectName := reg.FindStringSubmatch(line)[1]
				token := reg.FindStringSubmatch(line)[2]

				// Add the token to the project
				config.Mirrors[projectName].AccessToken = token
			}
		}
	}

	return config
}

func getPassword(filename string) string {
	bytes, err := os.ReadFile(filename)

	if err != nil {
		log.Fatal("Could not read password file: ", filename, " ", err.Error())
	}

	// trim whitespace from the end
	password := strings.TrimSpace(string(bytes))

	return string(password)
}

func createRsyncdConfig(config *ConfigFile) {
	tmpl := `# This is a generated file. Do not edit manually.
uid = nobody
gid = nogroup
use chroot = yes
max connections = 40
pid file = /var/run/rsyncd.pid
motd file = /etc/rsyncd.motd
log file = /var/log/rsyncd.log
log format = %t %o %a %m %f %b
dont compress = *.gz *.tgz *.zip *.z *.Z *.rpm *.deb *.bz2 *.tbz2 *.xz *.txz *.rar
refuse options = checksum delete
{{ range .Mirrors }}
[{{ .Short }}]
	comment = {{ .Name }}
	path = /storage/{{ .Short }}
	exclude = lost+found/
	read only = true
	ignore nonreadable = yes{{ end }}
`

	// Apply the template
	t := template.Must(template.New("rsyncd.conf").Parse(tmpl))
	var buf bytes.Buffer
	err := t.Execute(&buf, config)

	if err != nil {
		log.Fatal("Could not create rsyncd.conf: ", err.Error())
	}

	// save to rsyncd.conf
	err = os.WriteFile("/etc/rsyncd.conf", buf.Bytes(), 0644)
	if err != nil {
		logging.Error("Could not write rsyncd.conf: ", err.Error())
	}
}
