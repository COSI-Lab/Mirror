package mirrormap

import (
	"log"

	"github.com/gorilla/websocket"
)

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
			log.Printf("error: %v", err)
			break
		}

		w.Write(message)
		w.Close()
	}
}
