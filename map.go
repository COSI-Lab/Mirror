package main

import (
	"net"
	"net/http"

	"github.com/COSI-Lab/logging"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var h hub

// Upgrade the connection to a websocket and start the client
func HandleWebsocket(w http.ResponseWriter, r *http.Request) {
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
			// broadcasts the message to all clients
			for client := range hub.clients {
				select {
				case client.send <- message:
				default:
					// If the client blocks we skip it
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

func MapRouter(r *mux.Router, broadcast chan []byte) {
	r.HandleFunc("/ws", HandleWebsocket)
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

func entriesToMessages(entries chan *NginxLogEntry, messages chan []byte) {
	// Send groups of 8 messages
	ch := make(chan []byte)
	go func() {
		for {
			group := make([]byte, 0, 40)
			for i := 0; i < 8; i++ {
				group = append(group, <-ch...)
			}
			messages <- group
		}
	}()

	// Track the previous IP to avoid sending duplicate data
	prevIP := net.IPv4(0, 0, 0, 0)
	for {
		// Read from the channel
		entry := <-entries

		// If the lookup failed, skip this entry
		if entry == nil || entry.City == nil {
			continue
		}

		// Skip if the IP is the same as the previous one
		if prevIP.Equal(entry.IP) {
			continue
		}

		// Update the previous IP
		prevIP = entry.IP

		// Get the distro
		project, ok := projects[entry.Distro]
		if !ok {
			continue
		}

		// Get the location
		lat_ := entry.City.Location.Latitude
		long_ := entry.City.Location.Longitude

		if lat_ == 0 && long_ == 0 {
			continue
		}

		// convert [-90, 90] latitude to [0, 4096] pixels
		lat := int16((lat_ + 90) * 4096 / 180)
		// convert [-180, 180] longitude to [0, 4096] pixels
		long := int16((long_ + 180) * 4096 / 360)

		// Create a new message
		msg := make([]byte, 5)
		// First byte is the project ID
		msg[0] = project.Id
		// Second and Third byte are the latitude
		msg[1] = byte(lat >> 8)
		msg[2] = byte(lat & 0xFF)
		// Fourth and Fifth byte are the longitude
		msg[3] = byte(long >> 8)
		msg[4] = byte(long & 0xFF)

		ch <- msg
	}
}
