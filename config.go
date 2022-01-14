package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/xeipuuv/gojsonschema"
)

type ConfigFile struct {
	Schema  string `json:"$schema"`
	Mirrors []struct {
		Name   string `json:"name"`
		Short  string `json:"short"`
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
		} `json:"rsync"`
		Static struct {
			Location string `json:"location"`
			Source   string `json:"source"`
		} `json:"static"`
		Official bool   `json:"official"`
		IsDistro bool   `json:"isDistro"`
		HomePage string `json:"homepage"`
	} `json:"mirrors"`
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

	return config
}
