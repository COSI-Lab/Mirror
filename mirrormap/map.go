package mirrormap

import (
	"fmt"
	"net/http"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var h hub

// Upgrade the connection to a websocket and start the client
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the connection to a websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Log(logging.Warn, err)
		return
	}

	// Create a new client
	client := &client{
		conn: conn,
		send: make(chan []byte),
	}

	// Register the client
	h.register <- client

	// Start the client
	go client.write()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Returns count of connected clients
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprint(h.count())))
}

func MapRouter(r *mux.Router, broadcast chan []byte) {
	r.HandleFunc("/ws", handleWebsocket)
	r.HandleFunc("/health", handleHealth)

	// Create a new hub
	h = hub{
		broadcast:  broadcast,
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[*client]bool),
	}

	// Start the hub
	go h.run()
}
