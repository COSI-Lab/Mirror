package main

import (
	"log"
	"os"

	"github.com/COSI_Lab/Mirror/mirrorErrors"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Load config file and check schema
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")

	shorts := make([]string, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		shorts = append(shorts, mirror.Name)
	}

	writer, reader := InfluxClients(os.Getenv("INFLUX_TOKEN"))

	nginx_entries := make(chan *LogEntry, 100)
	map_entries := make(chan *LogEntry, 100)

	go ReadLogFile("access.log", nginx_entries, map_entries)
	// ReadLogs("/var/log/nginx/access.log", channels)

	if os.Getenv("INFLUX_TOKEN") == "" {
		mirrorErrors.Error("\x1B[31m[Error]\x1B[0m Missing .env envirnment variable INFLUX_TOKEN, not using database")
		log.Println("\x1B[31m[Error]\x1B[0m Missing .env envirnment variable INFLUX_TOKEN, not using database")
	} else {
		InitNGINXStats(shorts, reader)
		go HandleNGINXStats(nginx_entries, writer)
	}

	if InitWebserver() == nil {
		HandleWebserver(map_entries)
	}
}
