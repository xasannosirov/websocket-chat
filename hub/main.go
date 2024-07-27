// WEBSOCKET AND HTTP
// 1. send request with http protocol by front-end
// 2. receive request then handle and process by back-end
// 3. send response to web-socket with channel by back-end
// 4. send response to font-end by web-socket
// 5. continue process

package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
)

type Connection struct {
	WS   *websocket.Conn
	Send chan []byte
}

type Hub struct {
	Connections map[*Connection]bool
	Broadcast   chan []byte
}

func NewHub() *Hub {
	return &Hub{
		Connections: make(map[*Connection]bool),
		Broadcast:   make(chan []byte),
	}
}

func (h *Hub) Run() {
	for {
		message := <-h.Broadcast
		for conn := range h.Connections {
			select {
			case conn.Send <- message:
			default:
				close(conn.Send)
				delete(h.Connections, conn)
			}
		}
	}
}

func handleConnection(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		log.Println(err)
		return
	}

	connection := &Connection{
		WS:   conn,
		Send: make(chan []byte),
	}

	h.Connections[connection] = true

	go func() {
		defer func() {
			delete(h.Connections, connection)
			connection.WS.Close()
		}()

		for {
			_, message, err := connection.WS.ReadMessage()
			if err != nil {
				break
			}
			h.Broadcast <- message
		}
	}()

	go func() {
		for msg := range connection.Send {
			if err := connection.WS.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()
}

func handleMessageHTTPRequest(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		h.Broadcast <- body
		fmt.Fprintf(w, "Message sent to channel")
	}
}

func main() {
	hub := NewHub()
	go hub.Run()

	router := mux.NewRouter()
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnection(hub, w, r)
	})

	router.HandleFunc("/send", handleMessageHTTPRequest(hub)).Methods("POST")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
