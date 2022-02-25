package mirrormap

import (
	"github.com/COSI_Lab/Mirror/logging"
)

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

func (hub *hub) count() int {
	return len(hub.clients)
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
