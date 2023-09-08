package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/COSI-Lab/Mirror/config"
	"github.com/COSI-Lab/Mirror/logging"
	"github.com/COSI-Lab/geoip"
	"github.com/gofrs/flock"
	"github.com/joho/godotenv"
)

var geoipHandler *geoip.GeoIPHandler

// .env variables
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
	torrentDir = os.Getenv("TORRENT_DIR")
	downloadDir = os.Getenv("DOWNLOAD_DIR")

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

func loadConfig() (*config.File, error) {
	configFile, err := os.Open("configs/mirrors.json")
	if err != nil {
		return nil, errors.New("Could not open mirrors.json: " + err.Error())
	}
	defer configFile.Close()

	schemaFile, err := os.Open("configs/mirrors.schema.json")
	if err != nil {
		return nil, errors.New("Could not open mirrors.schema.json: " + err.Error())
	}
	defer schemaFile.Close()

	config, err := config.ReadProjectConfig(configFile, schemaFile)
	if err != nil {
		return nil, err
	}

	return config, config.Validate()
}

func loadTokens() (*config.Tokens, error) {
	tokensFile, err := os.Open("configs/tokens.toml")
	if err != nil {
		return nil, errors.New("Could not open tokens.toml: " + err.Error())
	}

	tokens, err := config.ReadTokens(tokensFile)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func startNGINX(config *config.File) (chan<- NGINXLogEntry, time.Time, error) {
	nginxAg := NewNGINXProjectAggregator()
	nginxAg.AddMeasurement("nginx", func(re NGINXLogEntry) bool {
		return true
	})

	// Add subnet aggregators
	for name, subnetStrings := range config.Subnets {
		subnets := make([]*net.IPNet, 0)
		for _, subnetString := range subnetStrings {
			_, subnet, err := net.ParseCIDR(subnetString)
			if err != nil {
				logging.Warn("Failed to parse subnet", subnetString, "for", name)
				continue
			}
			subnets = append(subnets, subnet)
		}

		if len(subnets) == 0 {
			logging.Warn("No valid subnets for", name)
			continue
		}

		nginxAg.AddMeasurement(name, func(re NGINXLogEntry) bool {
			for _, subnet := range subnets {
				if subnet.Contains(re.IP) {
					return true
				}
			}
			return false
		})

		logging.Info("Added subnet aggregator for", name)
	}

	nginxMetrics := make(chan NGINXLogEntry)
	nginxLastUpdated, err := StartAggregator[NGINXLogEntry](nginxAg, nginxMetrics, reader, writer)

	return nginxMetrics, nginxLastUpdated, err
}

func startRSYNC() (chan<- RsyncdLogEntry, time.Time, error) {
	rsyncAg := NewRSYNCProjectAggregator()

	rsyncMetrics := make(chan RsyncdLogEntry)
	rsyncLastUpdated, err := StartAggregator[RsyncdLogEntry](rsyncAg, rsyncMetrics, reader, writer)

	return rsyncMetrics, rsyncLastUpdated, err
}

func main() {
	// Enforce we are running linux or macos
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		fmt.Println("This program is only meant to be run on *nix systems")
		os.Exit(1)
	}

	// Do not run as root
	if os.Geteuid() == 0 {
		fmt.Println("This program should no longer be run as root")
	}

	// Manage lock file to prevent multiple instances from running simultaneously
	f := flock.New(os.TempDir() + "/mirror.lock")
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		if f.Locked() {
			f.Unlock()
		}
		os.Exit(0)
	}()

	locked, err := f.TryLock()
	if err != nil {
		logging.Error(f.Path(), " could not be locked: ", err)
		os.Exit(1)
	}
	if !locked {
		logging.Error(f.Path(), " is already locked")
		os.Exit(1)
	}

	// Parse the config file
	cfg, err := loadConfig()
	if err != nil {
		logging.Error("Failed to load config file:", err)
		os.Exit(1)
	}

	tokens, err := loadTokens()
	if err != nil {
		logging.Error("Failed to load tokens file:", err)
		os.Exit(1)
	}

	// GeoIP lookup
	if maxmindLicenseKey != "" {
		geoipHandler, err = geoip.NewGeoIPHandler(maxmindLicenseKey)
		if err != nil {
			logging.Error("Failed to initialize GeoIP handler:", err)
		}
	}

	// Update rsyncd.conf file based on the config file
	rsyncd_conf, err := os.OpenFile("/etc/rsyncd.conf", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Error("Could not open rsyncd.conf: ", err.Error())
	}
	err = cfg.CreateRsyncdConfig(rsyncd_conf)
	if err != nil {
		logging.Error("Failed to create rsyncd.conf: ", err.Error())
	}

	nginxChannels := make([]chan<- NGINXLogEntry, 0)
	nginxLastUpdated := time.Now()
	rsyncChannels := make([]chan<- RsyncdLogEntry, 0)
	rsyncLastUpdated := time.Now()

	if influxToken != "" {
		// Setup reader and writer for influxdb
		SetupInfluxClients(influxToken)

		// Start the nginx aggregator
		nginxMetrics, lastupdated, err := startNGINX(cfg)
		if err != nil {
			logging.Error("Failed to start nginx aggregator:", err)
			nginxLastUpdated = time.Now()
		} else {
			nginxChannels = append(nginxChannels, nginxMetrics)
			nginxLastUpdated = lastupdated
		}

		// Start the rsync aggregator
		rsyncMetrics, lastupdated, err := startRSYNC()
		if err != nil {
			logging.Error("Failed to start rsync aggregator:", err)
			rsyncLastUpdated = time.Now()
		} else {
			rsyncChannels = append(rsyncChannels, rsyncMetrics)
			rsyncLastUpdated = lastupdated
		}
	}

	manual := make(chan string)
	scheduler := NewScheduler(cfg, context.Background())
	go scheduler.Start(manual)

	// torrent scheduler
	if torrentDir != "" && downloadDir != "" {
		go HandleTorrents(cfg, torrentDir, downloadDir)
	}

	// WebServer
	mapEntries := make(chan NGINXLogEntry)
	nginxChannels = append(nginxChannels, mapEntries)

	WebServerLoadConfig(cfg, tokens)
	go HandleWebServer(manual, mapEntries)

	go TailNGINXLogFile(nginxTail, nginxLastUpdated, nginxChannels)
	go TailRSYNCLogFile(rsyncdTail, rsyncLastUpdated, rsyncChannels)

	// Wait forever
	select {}
}
