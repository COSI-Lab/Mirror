package main

import (
	"os"
	"time"

	"github.com/COSI_Lab/Mirror/datarithms"
	"github.com/COSI_Lab/Mirror/logging"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Setup error logger
	err := logging.Setup()
	if err != nil {
		logging.Error("Setting up logging", err)
	}

	// Load config file and check schema
	config := ParseConfig("configs/testmirror.json", "configs/mirrors.schema.json")
	shorts := make([]string, 0, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		shorts = append(shorts, mirror.Short)
	}

	// We always do the map parsing
	map_entries := make(chan *LogEntry, 100)

	// Connect to the database
	influxToken := os.Getenv("INFLUX_TOKEN")
	if influxToken == "" {
		logging.Error("Missing .env envirnment variable INFLUX_TOKEN, not using database")

		if os.Getenv("TAIL") != "" {
			go ReadLogs("/var/log/nginx/access.log", map_entries)
		} else {
			go ReadLogFile("access.log", map_entries)
		}
	} else {
		SetupInfluxClients(influxToken)
		logging.Success("Connected to InfluxDB")

		// Stats handling
		nginx_entries := make(chan *LogEntry, 100)

		InitNGINXStats(shorts)
		go HandleNGINXStats(nginx_entries)

		if os.Getenv("TAIL") != "" {
			go ReadLogs("/var/log/nginx/access.log", nginx_entries, map_entries)
		} else {
			go ReadLogFile("access.log", nginx_entries, map_entries)
		}
	}

	// RSYNC
	rsyncStatus := &RSYNCStatus{
		Status: make(map[string]*datarithms.CircularQueue),
	}
	initRSYNC(config)
	go handleRSYNC(config, rsyncStatus)

	// Webserver
	if InitWebserver() == nil {
		webserverLoadConfig(config)
		go HandleWebserver(shorts, map_entries, rsyncStatus)
	}

	// Wait for all goroutines to finish
	for {
		time.Sleep(time.Hour)
	}
}
