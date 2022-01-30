package main

import (
	"os"
	"time"

	"github.com/COSI_Lab/Mirror/mirrorErrors"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Setup error logger
	err := mirrorErrors.Setup()
	if err != nil {
		mirrorErrors.Error(err.Error(), "error")
	}

	mirrorErrors.Error("Starting Mirror", "startup")

	// Load config file and check schema
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")

	shorts := make([]string, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		shorts = append(shorts, mirror.Name)
	}

	writer, reader := InfluxClients(os.Getenv("INFLUX_TOKEN"))
	mirrorErrors.Error("Connected to InfluxDB", "startup")

	nginx_entries := make(chan *LogEntry, 100)
	map_entries := make(chan *LogEntry, 100)

	go ReadLogFile("access.log", nginx_entries, map_entries)
	// ReadLogs("/var/log/nginx/access.log", channels)

	if os.Getenv("INFLUX_TOKEN") == "" {
		mirrorErrors.Error("Missing .env envirnment variable INFLUX_TOKEN, not using database", "error")
	} else {
		InitNGINXStats(shorts, reader)
		go HandleNGINXStats(nginx_entries, writer)
	}

	if InitWebserver() == nil {
		webserverLoadConfig(config)
		go HandleWebserver(map_entries)
	}

	// Wait for all goroutines to finish
	for {
		time.Sleep(time.Second)
	}
}
