package main

import (
	"os"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Setup error logger
	err := logging.Setup()
	if err != nil {
		logging.Log(logging.Error, "Setting up logging", err)
	}

	// Load config file and check schema
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")

	shorts := make([]string, 0, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		shorts = append(shorts, mirror.Short)
	}

	InfluxClients(os.Getenv("INFLUX_TOKEN"))
	logging.Log(logging.Success, "Connected to InfluxDB")

	nginx_entries := make(chan *LogEntry, 100)
	map_entries := make(chan *LogEntry, 100)

	go ReadLogFile("access.log", nginx_entries, map_entries)
	// ReadLogs("/var/log/nginx/access.log", channels)

	if os.Getenv("INFLUX_TOKEN") == "" {
		logging.Log(logging.Error, "Missing .env envirnment variable INFLUX_TOKEN, not using database")
	} else {
		InitNGINXStats(shorts)
		go HandleNGINXStats(nginx_entries)
	}

	if InitWebserver() == nil {
		webserverLoadConfig(config)
		go HandleWebserver(shorts, map_entries)
	}

	// Wait for all goroutines to finish
	for {
		time.Sleep(time.Second)
	}
}
