package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/thanhpk/randstr"
)

var clients map[string]chan []byte
var clients_lock sync.RWMutex
var upgrader = websocket.Upgrader{} // use default options

func InitMap() {

}

func ip2string(ip net.IP) string {
	return string(ip)
}

func serve(clients map[string]chan []byte, entries chan *LogEntry) {

	// Create a map of dists and give them an id, hashing a map is quicker than an array
	distList := []string{"almalinux", "alpine", "archlinux", "archlinux32", "artix-linux", "blender", "centos", "clonezilla", "cpan", "cran", "ctan", "cygwin", "debian", "debian-cd", "eclipse", "freebsd", "gentoo", "gentoo-portage", "gparted", "ipfire", "isabelle", "linux", "linuxmint", "manjaro", "msys2", "odroid", "openbsd", "opensuse", "parrot", "raspbian", "RebornOS", "ros", "sabayon", "serenity", "slackware", "slitaz", "tdf", "templeos", "ubuntu", "ubuntu-cdimage", "ubuntu-ports", "ubuntu-releases", "videolan", "voidlinux", "zorinos"}
	distMap := make(map[string]int)
	for i, dist := range distList {
		distMap[dist] = i
	}

	// Track the previous IP to avoid sending duplicate data
	prevSkip := false
	prevIP := ""

	for {
		// Read from the channel
		entry := <-entries

		clients_lock.RLock()
		skip := len(clients) == 0 || entry.City == nil
		clients_lock.RUnlock()

		if prevSkip != skip {
			prevSkip = skip
			if skip {
				log.Printf("No clients connected, skipping")
			} else {
				log.Printf("Clients connected, sending")
			}
		}

		if skip {
			continue
		}

		if prevIP == "" {
			continue
		}

		ip := ip2string(entry.IP)
		if prevIP == ip {
			continue
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

	log.Printf("%s connected!\n", id)

	// Upgrade our raw HTTP connection to a websocket based one
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Error during connection upgradation:", err)
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
	log.Printf("Error sending message %s : %s", id, err)
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
	log.Printf("new connection registered: %s\n", id)

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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func HandleMap(entries chan *LogEntry) {
	clients = make(map[string]chan []byte)

	interrupt := make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupt
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		os.Exit(1)
	}()

	// Read from standard in and pass cordinates to each client
	go serve(clients, entries)

	// gorilla/mux router
	r := mux.NewRouter()

	r.HandleFunc("/map/health", healthHandler)
	r.HandleFunc("/map/register", registerHandler)
	r.HandleFunc("/map/socket/{id}", socketHandler)
	r.PathPrefix("/map").Handler(http.StripPrefix("/map", http.FileServer(http.Dir("static"))))

	r.Use(loggingMiddleware)

	// Serve on 8080
	l := &http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	log.Printf("Serving on http://localhost:%d/map", 8000)
	log.Fatalf("%s", l.ListenAndServe())
}
