package main

import (
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
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
	// RSYNCD_TAIL
	rsyncdTail string
	// RSYNC_DRY_RUN
	rsyncDryRun bool
	// RSYNC_LOGS
	rsyncLogs string
	// WEB_SERVER_CACHE
	webServerCache bool
	// HOOK_URL
	hookURL string
	// PING_ID
	pingID string
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
	rsyncdTail = os.Getenv("RSYNCD_TAIL")
	rsyncDryRun = os.Getenv("RSYNC_DRY_RUN") == "true"
	rsyncLogs = os.Getenv("RSYNC_LOGS")
	webServerCache = os.Getenv("WEB_SERVER_CACHE") == "true"
	hookURL = os.Getenv("HOOK_URL")
	pingID = os.Getenv("PING_ID")

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

	if rsyncdTail == "" {
		logging.Warn("No RSYNCD_TAIL environment variable found. Live tail will not be used and will instead attempt to read ./rsyncd.log")
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

	if hookURL == "" || pingID == "" {
		logging.Warn("HOOK_URL and PING_ID are required. Discord webhooks will not be used")
	}
}

func loadConfig() *ConfigFile {
	config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json", "configs/tokens.txt")
	return &config
}

var restartCount int

func main() {
	defer func() {
		if r := recover(); r != nil {
			restartCount++
			if restartCount > 3 {
				logging.PanicWithAttachment(debug.Stack(), "Program panicked more than 3 times in an hour! Exiting.")
				os.Exit(1)
			}

			logging.PanicWithAttachment(debug.Stack(), "Program panicked and attempted to restart itself. Someone should ssh in and check it out.")
			main()
		}
	}()

	// Setup logging
	logging.Setup(hookURL, pingID)

	// Parse the config file
	config := loadConfig()

	// Update the rsyncd.conf file based on the config file
	createRsyncdConfig(config)

	// We will always run the mirror map
	map_entries := make(chan *NginxLogEntry, 100)

	// GeoIP lookup
	geoip = NewGeoIPHandler(maxmindLicenseKey)

	// Connect to the database
	if influxToken == "" {
		if nginxTail != "" {
			// zero date
			var zero time.Time
			go TailNginxLogFile(nginxTail, zero, map_entries)
		} else {
			// if nginxTail is empty we attempt to read a local access log for testing
			go ReadNginxLogFile("access.log", map_entries)
		}
	} else {
		SetupInfluxClients(influxToken)
		logging.Success("Connected to InfluxDB")

		// Stats handling
		nginxEntries := make(chan *NginxLogEntry, 100)
		rsyncdEntries := make(chan *RsyncdLogEntry, 100)

		lastUpdated, err := InitStatistics(config.Mirrors)

		if err != nil {
			logging.Error("Failed to initialize statistics. Not tracking statistics", err)
		} else {
			logging.Success("Initialized statistics")
			go HandleStatistics(nginxEntries, rsyncdEntries)

			if nginxTail != "" {
				go TailNginxLogFile(nginxTail, lastUpdated, nginxEntries, map_entries)
			} else {
				// if nginxTail is empty we attempt to read a local file for testing
				go ReadNginxLogFile("access.log", nginxEntries, map_entries)
			}

			if rsyncdTail != "" {
				go TailRSyncdLogFile(rsyncdTail, lastUpdated, rsyncdEntries)
			} else {
				// if rsyncdTail is empty we attempt to read a local file for testing
				go ReadRsyncdLogFile("rsyncd.log", rsyncdEntries)
			}
		}
	}

	// Listen for sighup
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	// rsync scheduler
	stop := make(chan struct{})
	manual := make(chan string)
	rsyncStatus := make(RSYNCStatus)
	go handleRSYNC(config, rsyncStatus, manual, stop)

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
			go handleRSYNC(config, rsyncStatus, manual, stop)
		}
	}()

	// Webserver
	WebserverLoadConfig(config)
	go HandleWebserver(manual, map_entries)

	for {
		logging.Info(runtime.NumGoroutine(), "goroutines")
		time.Sleep(time.Hour)

		// Reset the restart count
		restartCount = 0
	}
}
