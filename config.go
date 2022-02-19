package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/xeipuuv/gojsonschema"
)

type ConfigFile struct {
	Schema  string              `json:"$schema"`
	Mirrors map[string]*Project `json:"mirrors"`
}

type Project struct {
	Name   string `json:"name"`
	Short  string // Copied from key
	Id     byte   // Id is given out in alphabetical order of short (yes only 255 are supported)
	Script struct {
		Command     string `json:"command"`
		SyncsPerDay int    `json:"syncs_per_day"`
	}
	Rsync struct {
		Options      string `json:"options"`
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
	IsDistro    bool   `json:"isDistro"`
	HomePage    string `json:"homepage"`
	PublicRsync bool   `json:"publicRsync"`
}

func ParseConfig(configFile, schemaFile string) (config ConfigFile) {
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
		panic(err.Error())
	}

	// Report errors
	if !result.Valid() {
		fmt.Printf("The document is not valid. see errors :\n")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
	}

	// Finally parse the config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("Could not parse the config file even though it fits the schema file: ", err.Error())
	}

	// Parse passwords & copy key as short
	var i uint8 = 0
	for short, project := range config.Mirrors {
		if project.Rsync.PasswordFile != "" {
			project.Rsync.Password = getPassword("configs/" + project.Rsync.PasswordFile)
		}
		project.Short = short
		project.Id = i

		// add 1 and check for overflow
		if i == 255 {
			logging.Panic("Too many projects, 255 is the maximum because of the live map")
		}
		i++
	}

	return config
}

func getPassword(filename string) string {
	bytes, err := ioutil.ReadFile(filename)

	if err != nil {
		logging.Warn("Could not read password file: ", filename, err.Error())
	}

	return string(bytes)
}
