package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/thanhpk/randstr"
)

var clients map[string]chan []byte
var clients_lock sync.RWMutex
var upgrader = websocket.Upgrader{} // use default options

func serve(clients map[string]chan []byte, entries chan *LogEntry) {
	// Create a map of dists and give them an id, hashing a map is quicker than an array
	distList := []string{"almalinux", "alpine", "archlinux", "archlinux32", "artix-linux", "blender", "centos", "clonezilla", "cpan", "cran", "ctan", "cygwin", "debian", "debian-cd", "eclipse", "freebsd", "gentoo", "gentoo-portage", "gparted", "ipfire", "isabelle", "linux", "linuxmint", "manjaro", "msys2", "odroid", "openbsd", "opensuse", "parrot", "raspbian", "RebornOS", "ros", "sabayon", "serenity", "slackware", "slitaz", "tdf", "templeos", "ubuntu", "ubuntu-cdimage", "ubuntu-ports", "ubuntu-releases", "videolan", "voidlinux", "zorinos"}
	distMap := make(map[string]int)
	for i, dist := range distList {
		distMap[dist] = i
	}

	// Track the previous IP to avoid sending duplicate data
	prevIP := net.IPv4(0, 0, 0, 0)

	// Track if we skipped sending data last entry when we change state we want to print to stdout
	var prevSkip bool

	for {
		// Read from the channel
		entry := <-entries

		if entry.City == nil {
			continue
		}

		clients_lock.RLock()
		skip := len(clients) == 0
		clients_lock.RUnlock()

		if prevIP.Equal(entry.IP) {
			continue
		}

		if skip != prevSkip {
			prevSkip = skip

			if skip {
				log.Printf("[INFO] MirrorMap no clients connected, skipping")
				continue
			} else {
				log.Printf("[INFO] MirrorMap new clients connected, sending data")
			}
		}

		long := entry.City.Location.Latitude
		lat := entry.City.Location.Longitude

		distByte := byte(distMap[entry.Distro])

		var latbyte, longbyte [8]byte
		binary.LittleEndian.PutUint64(latbyte[:], math.Float64bits(lat))
		binary.LittleEndian.PutUint64(longbyte[:], math.Float64bits(long))
		msg := []byte{distByte}
		msg = append(msg, latbyte[:]...)
		msg = append(msg, longbyte[:]...)

		clients_lock.Lock()

		for _, ch := range clients {
			select {
			case ch <- msg:
			default:
			}
		}
		clients_lock.Unlock()
	}

}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	// Handles the websocket
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		w.WriteHeader(404)
		return
	}

	// get the channel
	ch := clients[id]

	log.Printf("[INFO] Websocket new client connected %s : %s ", id, r.RemoteAddr)

	// Upgrade our raw HTTP connection to a websocket based one
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("\x1B[33m[WARN]\x1B[0m Error during connection upgradate", err)
		return
	}

	for {
		// Reciever byte array
		val := <-ch
		// Send message across websocket
		err = conn.WriteMessage(2, val)
		if err != nil {
			break
		}
	}

	// Close connection gracefully
	conn.Close()
	clients_lock.Lock()
	log.Printf("\x1B[33m[WARN]\x1B[0m Error sending message %s : %s", id, err)
	delete(clients, id)
	clients_lock.Unlock()
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	// Create UUID but badly
	// Should work as we arent serving enough clients were psuedo random will mess us up
	id := randstr.Hex(16)

	clients_lock.Lock()
	clients[id] = make(chan []byte, 10)
	clients_lock.Unlock()
	log.Printf("[INFO] Map new connection registered: %s\n", id)

	// Send id to client
	w.WriteHeader(200)
	w.Write([]byte(id))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Send diagnostic information
	clients_lock.RLock()
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprint(len(clients))))
	clients_lock.RUnlock()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {

}

func MapRouter(r *mux.Router, entries chan *LogEntry) {
	clients = make(map[string]chan []byte)

	// Read entries and pass cordinates to each client
	go serve(clients, entries)

	r.HandleFunc("/health", healthHandler)
	r.HandleFunc("/register", registerHandler)
	r.HandleFunc("/socket/{id}", socketHandler)
	r.HandleFunc("/", indexHandler)
}
