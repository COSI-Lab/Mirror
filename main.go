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

	shorts := []string{"almalinux", "alpine", "archlinux", "archlinux32", "artix-linux", "blender", "centos", "clonezilla", "cpan", "cran", "ctan", "cygwin", "debian", "debian-cd", "debian-security", "eclipse", "fedora", "fedora-epel", "freebsd", "gentoo", "gentoo-portage", "gnu", "gparted", "ipfire", "isabelle", "linux", "linuxmint", "manjaro", "msys2", "odroid", "openbsd", "opensuse", "parrot", "raspbian", "RebornOS", "ros", "sabayon", "serenity", "slackware", "slitaz", "tdf", "templeos", "ubuntu", "ubuntu-cdimage", "ubuntu-ports", "ubuntu-releases", "videolan", "voidlinux", "zorinos"}
	points := make(chan DataPoint)

	go HandleNGINX(shorts, points)

	writer := SetupWriteClient(os.Getenv("INFLUX_TOKEN"))

	for p := range points {
		writer.WritePoint(p)
	}

	writer.Flush()
}
