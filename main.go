package main

import (
	"log"
	"os"
	"time"

	queue "github.com/COSI_Lab/Mirror/datarithms"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Load config file and check schema
	config := ParseConfig("configs/mirrors.json.test", "configs/mirrors.schema.json")

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
		log.Println("\x1B[31m[Error]\x1B[0m Missing .env envirnment variable INFLUX_TOKEN, not using database")
	} else {
		InitNGINXStats(shorts, reader)
		go HandleNGINXStats(nginx_entries, writer)
	}

	if InitWebserver() == nil {
		webserverLoadConfig(config)
		go HandleWebserver(map_entries)
	}

	// TODO should be moved into it's own area
	//
	rsyncStatus := make(map[string]*queue.CircularQueue, len(config.Mirrors))

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			rsyncStatus[mirror.Short] = queue.Init(7 * mirror.Rsync.SyncsPerDay)
		}
	}

	for _, mirror := range config.Mirrors {
		b, _ := rsync(mirror)
		// TODO check if the state is ok
		rsyncStatus[mirror.Short].Push(b)
	}

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			log.Println(mirror.Short, rsyncStatus[mirror.Short].Len())
		}
	}

	// Wait for all goroutines to finish
	for {
		time.Sleep(time.Second)
	}
}
