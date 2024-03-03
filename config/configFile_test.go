package config

import (
	"os"
	"testing"
)

// Verify that ../configs/mirrors.json is valid
func TestMirrorJSON(t *testing.T) {
	// Open the config file
	file, err := os.Open("../configs/mirrors.json")
	if err != nil {
		t.Error("Could not open mirrors.json", err.Error())
	}

	// Open the schema file
	schema, err := os.Open("../configs/mirrors.schema.json")
	if err != nil {
		t.Error("Could not open mirrors.schema.json", err.Error())
	}

	// Parse the config
	config, err := ReadProjectConfig(file, schema)
	if err != nil {
		t.Error("Could not parse mirrors.json", err.Error())
	}

	// Verify that something was parsed
	if config == nil {
		t.Error("Config was nil")
		return
	}
	if len(config.Projects) == 0 {
		t.Error("Config had no projects")
	}

	// Verify that the config is valid
	err = config.Validate()
	if err != nil {
		t.Error("Config did not validate", err.Error())
	}

}
