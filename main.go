package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/COSI-Lab/Mirror/aggregator"
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

	// Check if the environment variables are set
	if maxmindLicenseKey == "" {
		logging.Warn("No MAXMIND_LICENSE_KEY environment variable found. GeoIP database will not be updated")
	}

	if influxToken == "" {
		logging.Warn("No INFLUX_TOKEN environment variable found. InfluxDB will not be used")
	}

	// check that the system is linux
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		logging.Warn("Torrent syncing is only support on *nix systems including the `find` command")
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

func main() {
	// Mirror only runs on linux or macos
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

	// Initialize the tokens file for manual syncing
	tokens, err := loadTokens()
	if err != nil {
		logging.Error("Failed to load tokens file:", err)
		os.Exit(1)
	}

	// GeoIP lookup for the map
	if maxmindLicenseKey != "" {
		geoipHandler, err = geoip.NewGeoIPHandler(maxmindLicenseKey)
		if err != nil {
			logging.Error("Failed to initialize GeoIP handler:", err)
		}
	}

	// Update rsyncd.conf file based on the config file
	rsyncdConf, err := os.OpenFile("/etc/rsyncd.conf", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Error(err.Error())
	} else {
		err = cfg.CreateRSCYNDConfig(rsyncdConf)
		if err != nil {
			logging.Error("Failed to create rsyncd.conf: ", err.Error())
		}
	}

	// TODO: Update nginx.conf file based on the config file

	nginxChannels := make([]chan<- aggregator.NGINXLogEntry, 0)
	nginxLastUpdated := time.Now()
	rsyncChannels := make([]chan<- aggregator.RSCYNDLogEntry, 0)
	rsyncLastUpdated := time.Now()

	if influxToken != "" {
		// Setup reader and writer for influxdb
		reader, writer := SetupInfluxClients(influxToken)

		// Start the nginx aggregator
		nginxMetrics, lastupdated, err := StartNGINXAggregator(reader, writer, cfg)
		if err != nil {
			logging.Error("Failed to start nginx aggregator:", err)
			nginxLastUpdated = time.Now()
		} else {
			nginxChannels = append(nginxChannels, nginxMetrics)
			nginxLastUpdated = lastupdated
		}

		// Start the rsync aggregator
		rsyncMetrics, lastupdated, err := StartRSYNCAggregator(reader, writer)
		if err != nil {
			logging.Error("Failed to start rsync aggregator:", err)
			rsyncLastUpdated = time.Now()
		} else {
			rsyncChannels = append(rsyncChannels, rsyncMetrics)
			rsyncLastUpdated = lastupdated
		}
	}

	manual := make(chan string)
	scheduler, err := NewScheduler(context.Background(), cfg)
	if err != nil {
		logging.Error("Failed to create scheduler:", err)
		os.Exit(1)
	}

	go scheduler.Start(manual)

	// WebServer
	mapEntries := make(chan aggregator.NGINXLogEntry)
	nginxChannels = append(nginxChannels, mapEntries)

	WebServerLoadConfig(cfg, tokens)
	go HandleWebServer(manual, mapEntries)

	go aggregator.TailNGINXLogFile("/var/log/nginx/access.log", nginxLastUpdated, nginxChannels, geoipHandler)
	go aggregator.TailRSYNCLogFile("/var/log/nginx/rsyncd.log", rsyncLastUpdated, rsyncChannels)

	// Wait forever
	select {}
}
