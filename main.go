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
	nginxLastUpdated, err := StartAggregator[NGINXLogEntry](nginxAg, nginxMetrics)

	return nginxMetrics, nginxLastUpdated, err
}

func startRSYNC() (chan<- RSCYNDLogEntry, time.Time, error) {
	rsyncAg := NewRSYNCProjectAggregator()

	rsyncMetrics := make(chan RSCYNDLogEntry)
	rsyncLastUpdated, err := StartAggregator[RSCYNDLogEntry](rsyncAg, rsyncMetrics)

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
	rsyncdConf, err := os.OpenFile("/etc/rsyncd.conf", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Error("Could not open rsyncd.conf: ", err.Error())
	}
	err = cfg.CreateRSCYNDConfig(rsyncdConf)
	if err != nil {
		logging.Error("Failed to create rsyncd.conf: ", err.Error())
	}

	nginxChannels := make([]chan<- NGINXLogEntry, 0)
	nginxLastUpdated := time.Now()
	rsyncChannels := make([]chan<- RSCYNDLogEntry, 0)
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
	scheduler := NewScheduler(context.Background(), cfg)
	go scheduler.Start(manual)

	// WebServer
	mapEntries := make(chan NGINXLogEntry)
	nginxChannels = append(nginxChannels, mapEntries)

	WebServerLoadConfig(cfg, tokens)
	go HandleWebServer(manual, mapEntries)

	go TailNGINXLogFile("/var/log/nginx/access.log", nginxLastUpdated, nginxChannels)
	go TailRSYNCLogFile("/var/log/nginx/rsyncd.log", rsyncLastUpdated, rsyncChannels)

	// Wait forever
	select {}
}
