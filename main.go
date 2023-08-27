package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/COSI-Lab/geoip"
	"github.com/COSI-Lab/logging"
	"github.com/joho/godotenv"
)

var geoipHandler *geoip.GeoIPHandler

// .env variables
var (
	// ADM_GROUP
	admGroup int = 0
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
	// SCHEDULER_PAUSED
	schedulerPaused bool
	// RSYNC_DRY_RUN
	syncDryRun bool
	// RSYNC_LOGS
	syncLogs string
	// WEB_SERVER_CACHE
	webServerCache bool
	// HOOK_URL
	hookURL string
	// PING_ID
	pingID string
	// PULL_TOKEN
	pullToken string
	// TORRENT_DIR
	torrentDir string
	// DOWNLOAD_DIR
	downloadDir string
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
	schedulerPaused = os.Getenv("SCHEDULER_PAUSED") == "true"
	syncDryRun = os.Getenv("RSYNC_DRY_RUN") == "true" || os.Getenv("SYNC_DRY_RUN") == "true"
	syncLogs = os.Getenv("RSYNC_LOGS")
	webServerCache = os.Getenv("WEB_SERVER_CACHE") == "true"
	hookURL = os.Getenv("HOOK_URL")
	pingID = os.Getenv("PING_ID")
	pullToken = os.Getenv("PULL_TOKEN")
	admGroupStr := os.Getenv("ADM_GROUP")
	torrentDir = os.Getenv("TORRENT_DIR")
	downloadDir = os.Getenv("DOWNLOAD_DIR")

	if admGroupStr != "" {
		admGroup, err = strconv.Atoi(admGroupStr)

		if err != nil {
			logging.Warn("environment variable ADM_GROUP", err)
		} else {
			// Verify adm is in our list of groups
			groups, err := os.Getgroups()
			if err != nil {
				logging.Warn("Could not get groups")
			}
			var foundAdmGroup bool
			for _, group := range groups {
				if group == admGroup {
					foundAdmGroup = true
				}
			}
			if !foundAdmGroup {
				logging.Warn("ADM_GROUP is not in the list of usable groups")
			}
		}
	}

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

	if schedulerPaused {
		logging.Warn("SCHEDULER_PAUSED is set, the scheduler will not run and projects will never be synced")
	}

	if syncDryRun {
		logging.Warn("RSYNC_DRY_RUN is set, all rsyncs will be run in dry-run mode")
	}

	if syncLogs == "" {
		logging.Warn("No RSYNC_LOGS environment variable found. Persisent logs are not being saved")
	}

	if !webServerCache {
		logging.Warn("WEB_SERVER_CACHE is disabled. Expensive websever requests will not be cached")
	}

	if hookURL == "" || pingID == "" {
		logging.Warn("HOOK_URL and PING_ID are required. Discord webhooks will not be used")
	}

	if pullToken == "" {
		logging.Warn("PULL_TOKEN is not set so there is no master pull token")
	}

	// check that the system is linux
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		logging.Warn("Torrent syncing is only support on *nix systems including the `find` command")
	} else {
		if torrentDir == "" {
			logging.Warn("TORRENT_DIR is not set torrents will not be synced")
		}

		if downloadDir == "" {
			logging.Warn("DOWNLOAD_DIR is not set torrents will not be synced")
		}
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

	// Enforce we are running linux or macos
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		fmt.Println("This program is only meant to be run on *nix systems")
		os.Exit(1)
	}

	// Do not run as root
	if os.Geteuid() == 0 {
		fmt.Println("This program should no longer be run as root")
	}

	// Setup logging
	logging.Setup(hookURL, pingID)

	// Parse the config file
	config := loadConfig()

	// Update the rsyncd.conf file based on the config file
	createRsyncdConfig(config)
	// createNginxRedirects(config)

	// We will always run the mirror map
	map_entries := make(chan *NginxLogEntry, 100)

	// GeoIP lookup
	var err error
	if maxmindLicenseKey != "" {
		geoipHandler, err = geoip.NewGeoIPHandler(maxmindLicenseKey)
		if err != nil {
			logging.Error("Failed to initialize GeoIP handler:", err)
		}
	}

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

	var manual chan string

	if schedulerPaused {
		go func() {
			for {
				<-sighup
				logging.Info("Received SIGHUP")

				config = loadConfig()
				logging.Info("Reloaded config")

				WebserverLoadConfig(config)
				logging.Info("Reloaded projects page")
			}
		}()
	} else {
		// rsync scheduler
		stop := make(chan struct{})
		manual = make(chan string)
		rsyncStatus := make(RSYNCStatus)
		go handleSyncs(config, rsyncStatus, manual, stop)

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
				go handleSyncs(config, rsyncStatus, manual, stop)
			}
		}()
	}

	// torrent scheduler
	// TODO: handle reload
	if torrentDir != "" && downloadDir != "" {
		go HandleTorrents(config, torrentDir, downloadDir)
	}

	// Webserver
	WebserverLoadConfig(config)
	go HandleWebserver(manual, map_entries)

	go HandleCheckIn()

	go checkOldLogs()

	for {
		logging.Info(runtime.NumGoroutine(), "goroutines")
		time.Sleep(time.Hour)

		// Reset the restart count
		restartCount = 0
	}
}
