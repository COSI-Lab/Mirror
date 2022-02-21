package main

import (
	"os"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/joho/godotenv"
)

var geoip *GeoIPHandler

func main() {
	godotenv.Load()

	// Setup error logger
	err := logging.Setup()
	if err != nil {
		logging.Error(err)
	}

	// Load config file and check schema
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")

	// We always do the map parsing
	map_entries := make(chan *LogEntry, 100)

	// GeoIP lookup
	geoip, err = NewGeoIPHandler(os.Getenv("MAXMIND_LICENSE_KEY"))
	if err != nil {
		logging.Error("Failed to use MaxMind GeoIP data", err)
	} else {
		logging.Success("Using MaxMind GeoIP data")
	}

	// Connect to the database
	influxToken := os.Getenv("INFLUX_TOKEN")
	if influxToken == "" {
		logging.Error("missing .env variable INFLUX_TOKEN, not using database")

		// File to tail NGINX access logs, if empty then we read the static ./access.log file
		if os.Getenv("NGINX_TAIL") != "" {
			go ReadLogs(os.Getenv("NGINX_TAIL"), map_entries)
		} else {
			go ReadLogFile("access.log", map_entries)
		}
	} else {
		SetupInfluxClients(influxToken)
		logging.Success("Connected to InfluxDB")

		// Stats handling
		nginx_entries := make(chan *LogEntry, 100)

		InitNGINXStats(config.Mirrors)
		go HandleNGINXStats(nginx_entries)

		if os.Getenv("NGINX_TAIL") != "" {
			go ReadLogs(os.Getenv("NGINX_TAIL"), nginx_entries, map_entries)
		} else {
			go ReadLogFile("access.log", nginx_entries, map_entries)
		}
	}

	// RSYNC
	rsyncStatus := make(RSYNCStatus)
	if os.Getenv("RSYNC_DISABLE") != "" {
		logging.Error(".env variable RSYNC_DISABLE is set, rsync will not run")
	} else {
		if os.Getenv("RSYNC_LOGS") == "" {
			logging.Error("missing .env variable RSYNC_LOGS, not saving rsync logs")
		}

		initRSYNC(config)
		go handleRSYNC(config, rsyncStatus)
	}

	// Webserver
	InitWebserver()
	WebserverLoadConfig(config)
	go HandleWebserver(map_entries, rsyncStatus)

	// Wait for all goroutines to finish
	for {
		time.Sleep(time.Hour)
	}
}
