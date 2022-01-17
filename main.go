package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	if os.Getenv("INFLUX_TOKEN") == "" {
		log.Fatal("Missing .env envirnment variable INFLUX_TOKEN")
	}

	// Load config file and check schema
	// config := ParseConfig("configs/mirrors.json", "configs/mirrors.schema.json")

	// TODO replace hardcoded shorts with values generated from config
	shorts := []string{"almalinux", "alpine", "archlinux", "archlinux32", "artix-linux", "blender", "centos", "clonezilla", "cpan", "cran", "ctan", "cygwin", "debian", "debian-cd", "debian-security", "eclipse", "fedora", "fedora-epel", "freebsd", "gentoo", "gentoo-portage", "gnu", "gparted", "ipfire", "isabelle", "linux", "linuxmint", "manjaro", "msys2", "odroid", "openbsd", "opensuse", "parrot", "raspbian", "RebornOS", "ros", "sabayon", "serenity", "slackware", "slitaz", "tdf", "templeos", "ubuntu", "ubuntu-cdimage", "ubuntu-ports", "ubuntu-releases", "videolan", "voidlinux", "zorinos"}
	writer, reader := InfluxClients(os.Getenv("INFLUX_TOKEN"))

	nginx_entries := make(chan *LogEntry, 100)
	map_entries := make(chan *LogEntry, 100)

	go ReadLogFile("access.log", nginx_entries, map_entries)
	// ReadLogs("/var/log/nginx/access.log", channels)

	InitNGINXStats(shorts, reader)
	HandleNGINXStats(nginx_entries, writer)

	// HandleMap(map_entries)
}
