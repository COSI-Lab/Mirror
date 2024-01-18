package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// File is the struct that represents the main mirror.json config file
type File struct {
	Schema string `json:"$schema"`
	// Torrents is a list of upstreams to scrape .torrent files from
	Torrents []*ScrapeTarget `json:"torrents"`
	// Subnets defines a map of subnets we track usage from (e.g. "clarkson" -> ['128.153.0.0/16'])
	Subnets map[string][]string `json:"subnets"`
	// Projects is a map short names to project definitions
	Projects map[string]*Project `json:"mirrors"`
}

// ReadProjectConfig reads the main mirrors.json file and checks that it matches the schema
func ReadProjectConfig(cfg, schema io.Reader) (config *File, err error) {
	// read cfg and schema into byte arrays
	cfgBytes, err := io.ReadAll(cfg)
	if err != nil {
		return nil, err
	}

	schemaBytes, err := io.ReadAll(schema)
	if err != nil {
		return nil, err
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	documentLoader := gojsonschema.NewBytesLoader(cfgBytes)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		log.Fatal("Config file did not match the schema: ", err.Error())
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
	err = json.Unmarshal(cfgBytes, &config)
	if err != nil {
		return nil, err
	}

	// Post processing for the config
	var i uint8 = 0
	for short, project := range config.Projects {
		// Self reference the short name
		project.Short = short

		// Determine the sync style
		if project.Rsync != nil {
			project.SyncStyle = "rsync"
		} else if project.Static != nil {
			project.SyncStyle = "static"
		} else {
			project.SyncStyle = "script"
		}

		// Set the project id for the live map
		if i == 255 {
			return nil, fmt.Errorf("too many projects, max is 255")
		}
		project.ID = i
		i++
	}

	return config, nil
}

// GetProjects returns a slice of projects sorted by short name
func (config *File) GetProjects() []Project {
	var projects []Project

	for _, project := range config.Projects {
		projects = append(projects, *project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Short) < strings.ToLower(projects[j].Short)
	})

	return projects
}

// Validate checks the config file for a few properties
//
// - All projects have a unique long name, case insensitive
// - All projects have a unique short name, case insensitive
// - The sync style flag is set correctly
func (config *File) Validate() error {
	// Check that all projects have a unique long name
	longNames := make(map[string]bool)
	for _, project := range config.Projects {
		if _, ok := longNames[strings.ToLower(project.Name)]; ok {
			return fmt.Errorf("duplicate long name: %s", project.Name)
		}
		longNames[project.Name] = true
	}

	// Check that all projects have a unique short name, case insensitive
	shortNames := make(map[string]bool)
	for _, project := range config.Projects {
		if _, ok := shortNames[strings.ToLower(project.Short)]; ok {
			return fmt.Errorf("duplicate short name: %s", project.Short)
		}
		shortNames[strings.ToLower(project.Short)] = true
	}

	// Check that the sync style flag is set correctly
	for _, project := range config.Projects {
		switch project.SyncStyle {
		case "rsync":
			if project.Rsync == nil {
				return fmt.Errorf("sync style is 'rsync' but rsync config is nil for project %s", project.Short)
			}

			if project.Static != nil {
				return fmt.Errorf("sync style is 'rsync' but static config is not nil for project %s", project.Short)
			}

			if project.Script != nil {
				return fmt.Errorf("sync style is 'rsync' but script config is not nil for project %s", project.Short)
			}
		case "static":
			if project.Rsync != nil {
				return fmt.Errorf("sync style is 'static' but rsync config is not nil for project %s", project.Short)
			}

			if project.Static == nil {
				return fmt.Errorf("sync style is 'static' but static config is nil for project %s", project.Short)
			}

			if project.Script != nil {
				return fmt.Errorf("sync style is 'static' but script config is not nil for project %s", project.Short)
			}
		case "script":
			if project.Rsync != nil {
				return fmt.Errorf("sync style is 'script' but rsync config is not nil for project %s", project.Short)
			}

			if project.Static != nil {
				return fmt.Errorf("sync style is 'script' but static config is not nil for project %s", project.Short)
			}

			if project.Script == nil {
				return fmt.Errorf("sync style is 'script' but script config is nil for project %s", project.Short)
			}
		default:
			return fmt.Errorf("unknown sync style '%s' for project %s", project.SyncStyle, project.Short)
		}
	}

	return nil
}

// ProjectsGrouped is a simple 3-tuple of slices of projects
type ProjectsGrouped struct {
	// Distributions are projects with "Distributions" as their page
	Distributions []Project
	// Software are projects with "Software" as their page
	Software []Project
	// Miscellaneous are projects with "Miscellaneous" as their page
	Miscellaneous []Project
}

// GetProjectsByPage returns a ProjectsGrouped struct with the projects grouped by page.
// Within each group the projects are sorted by short
func (config *File) GetProjectsByPage() ProjectsGrouped {
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

	return ProjectsGrouped{
		Distributions: distributions,
		Software:      software,
		Miscellaneous: misc,
	}
}
