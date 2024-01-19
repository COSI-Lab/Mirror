package main

import (
	"net"
	"net/http"
	"time"

	"github.com/COSI-Lab/Mirror/aggregator"
	"github.com/COSI-Lab/Mirror/logging"
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
		logging.Warn(err)
		return
	}

	// Create a new client
	client := &client{
		conn: conn,
		send: make(chan []byte, 16),
	}

	// Register the client
	h.register <- client

	// Start the client
	go client.write()
}

type hub struct {
	// Hashset of clients
	clients map[*client]struct{}

	// Inbound messages from the clients
	broadcast chan []byte

	// registers a client from the hub
	register chan *client

	// unregister a client from the hub
	unregister chan *client
}

func (hub *hub) run() {
	for {
		select {
		case client := <-hub.register:
			// registers a client
			hub.clients[client] = struct{}{}
			logging.Info("Registered client", client.conn.RemoteAddr())
		case client := <-hub.unregister:
			// unregister a client
			delete(hub.clients, client)
			close(client.send)
			logging.Info("Unregistered client", client.conn.RemoteAddr())
		case message := <-hub.broadcast:
			for client := range hub.clients {
				select {
				case client.send <- message:
				default:
					// If the client is not receiving messages, unregister it
					hub.unregister <- client
				}
			}
		}
	}
}

type client struct {
	// The websocket connection
	conn *websocket.Conn

	// Outbound messages
	send chan []byte
}

func (c *client) write() {
	defer func() {
		c.conn.WriteMessage(websocket.CloseMessage, []byte{})
		c.conn.Close()
	}()

	for message := range c.send {
		w, err := c.conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			logging.Info("Closing websocket connection", err)
			break
		}

		w.Write(message)
		w.Close()
	}
}

// MapRouter adds map routes to the provided router
// Any messages sent to the broadcast channel will be forwarded to all connected clients
func MapRouter(r *mux.Router, broadcast chan []byte) {
	r.HandleFunc("/ws", handleWebsocket)
	r.HandleFunc("/health", handleHealth)

	// Create a new hub
	h = hub{
		broadcast:  broadcast,
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    make(map[*client]struct{}),
	}

	// Start the hub
	go h.run()
}

func entriesToMessages(entries <-chan aggregator.NGINXLogEntry, messages chan<- []byte) {
	// Send a group of messages every second
	ch := make(chan []byte)
	go func() {
		ticker := time.NewTicker(time.Second)
		group := make([]byte, 0)
		for {
			select {
			case msg := <-ch:
				group = append(group, msg...)
			case <-ticker.C:
				if len(group) == 0 {
					continue
				}

				messages <- group
				group = make([]byte, 0)
			}
		}
	}()

	// Deduplicate neighboring entries with the same IP
	prevIP := net.IPv4(0, 0, 0, 0)
	for {
		entry := <-entries

		// Skip the entry if it's an immediate duplicate
		if prevIP.Equal(entry.IP) {
			continue
		}
		prevIP = entry.IP

		if entry.City == nil {
			continue
		}

		// Maps project names to project structs
		if projects[entry.Project] != nil {
			continue
		}
		id := projects[entry.Project].ID

		// Get the location
		_lat := entry.City.Location.Latitude
		_long := entry.City.Location.Longitude

		if _lat == 0 && _long == 0 {
			continue
		}

		// convert [-90, 90] latitude to [0, 4096] pixels
		lat := int16((_lat + 90) * 4096 / 180)
		// convert [-180, 180] longitude to [0, 4096] pixels
		long := int16((_long + 180) * 4096 / 360)

		// Create a new message
		msg := make([]byte, 5)
		// First byte is the project ID
		msg[0] = id
		// Second and Third byte are the latitude
		msg[1] = byte(lat >> 8)
		msg[2] = byte(lat & 0xFF)
		// Fourth and Fifth byte are the longitude
		msg[3] = byte(long >> 8)
		msg[4] = byte(long & 0xFF)

		ch <- msg
	}
}
