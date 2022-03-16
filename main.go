package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/joho/godotenv"
)

var geoip *GeoIPHandler

var (
	// HOOK_URL and PING_URL and handled in the logging packages

	// MAXMIND_LICENSE_KEY
	maxmindLicenseKey string
	// INFLUX_TOKEN
	influxToken string
	// INFLUX_READ_ONLY
	influxReadOnly bool
	// NGINX_TAIL
	nginxTail string
	// RSYNC_DRY_RUN
	rsyncDryRun bool
	// RSYNC_LOGS
	rsyncLogs string
	// WEB_SERVER_CACHE
	webServerCache bool
)

func init() {
	// Print it's process ID
	logging.Info("PID:", os.Getpid())

	// Load the environment variables
	err := godotenv.Load()
	if err != nil {
		logging.Warn("No .env file found")
	}

	// Parse the necessary environment variables
	maxmindLicenseKey = os.Getenv("MAXMIND_LICENSE_KEY")
	influxToken = os.Getenv("INFLUX_TOKEN")
	influxReadOnly = os.Getenv("INFLUX_READ_ONLY") == "true"
	nginxTail = os.Getenv("NGINX_TAIL")
	rsyncDryRun = os.Getenv("RSYNC_DRY_RUN") == "true"
	rsyncLogs = os.Getenv("RSYNC_LOGS")
	webServerCache = os.Getenv("WEB_SERVER_CACHE") == "true"

	// Check if the environment variables are set
	if maxmindLicenseKey == "" {
		logging.Warn("No MAXMIND_LICENSE_KEY environment variable found. GeoIP database will not be updated")
	}

	if influxToken == "" {
		logging.Warn("No INFLUX_TOKEN environment variable found. InfluxDB will not be used")
	}

	if influxReadOnly {
		logging.Warn("INFLUX_READ_ONLY is set, InfluxDB will only be used for reading")
	}

	if nginxTail == "" {
		logging.Warn("No NGINX_TAIL environment variable found. Live tail will not be used and will instead attempt to read ./access.log")
	}

	if rsyncDryRun {
		logging.Warn("RSYNC_DRY_RUN is set, all rsyncs will be run in dry-run mode")
	}

	if rsyncLogs == "" {
		logging.Warn("No RSYNC_LOGS environment variable found. Persisent logs are not being saved")
	}

	if !webServerCache {
		logging.Warn("WEB_SERVER_CACHE is disabled. Expensive websever requests will not be cached")
	}
}

func loadConfig() *ConfigFile {
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")
	return &config
}

func main() {
	// Setup logging
	err := logging.Setup()
	if err != nil {
		logging.Warn(err)
	}

	// Load environment variables and parse the config file
	config := loadConfig()

	// We will always run the mirror map
	map_entries := make(chan *LogEntry, 100)

	// GeoIP lookup
	geoip = NewGeoIPHandler(maxmindLicenseKey)

	// Connect to the database
	if influxToken == "" {
		// File to tail NGINX access logs, if empty then we read the static ./access.log file
		if nginxTail != "" {
			go ReadLogs(nginxTail, map_entries)
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

		if nginxTail != "" {
			go ReadLogs(nginxTail, nginx_entries, map_entries)
		} else {
			go ReadLogFile("access.log", nginx_entries, map_entries)
		}
	}

	// Listen for sighup
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	// rsync scheduler
	stop := make(chan struct{})
	rsyncStatus := make(RSYNCStatus)
	go handleRSYNC(config, rsyncStatus, stop)

	go func() {
		for {
			<-sighup
			logging.Info("Received SIGHUP")

			config = loadConfig()
			logging.Info("Reloaded config")

			WebserverLoadConfig(config)
			logging.Info("Reloaded projects page")

			// stop the rsync scheduler
			stop <- struct{}{}
			<-stop

			// restart the rsync scheduler
			rsyncStatus := make(RSYNCStatus)
			go handleRSYNC(config, rsyncStatus, stop)
		}
	}()

	// Webserver
	WebserverLoadConfig(config)
	go HandleWebserver(map_entries)

	for {
		logging.Info(runtime.NumGoroutine(), "goroutines")
		time.Sleep(time.Hour)
	}
}
