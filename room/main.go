// GROUP CHAT
// 1. connect users with self username and room name
// 2. send message to room by connected user
// 3. send message all connected user by web-socket
// 4. receive message all connected user
// 5. continue process

package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn     *websocket.Conn
	username string
}

type Room struct {
	clients map[*websocket.Conn]Client
	mu      sync.Mutex
}

var rooms = make(map[string]*Room)
var globalMu sync.Mutex

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	username := r.URL.Query().Get("username")
	roomName := r.URL.Query().Get("room")
	if username == "" || roomName == "" {
		log.Println("username and room query params are required")
		conn.Close()
		return
	}

	globalMu.Lock()
	room, ok := rooms[roomName]
	if !ok {
		room = &Room{
			clients: make(map[*websocket.Conn]Client),
		}
		rooms[roomName] = room
	}
	globalMu.Unlock()

	client := Client{conn: conn, username: username}
	room.mu.Lock()
	room.clients[conn] = client
	room.mu.Unlock()

	log.Println("client", username, "connected to room", roomName)

	if err := conn.WriteMessage(websocket.TextMessage, []byte("Hi "+username+"! Welcome to room "+roomName)); err != nil {
		log.Println(err)
		room.mu.Lock()
		delete(room.clients, conn)
		room.mu.Unlock()
		conn.Close()
		return
	}

	go reader(client, room)
}

func reader(client Client, room *Room) {
	conn := client.conn
	username := client.username

	defer func() {
		room.mu.Lock()
		conn.Close()
		delete(room.clients, conn)
		room.mu.Unlock()
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		log.Println(username+":", string(message))

		room.mu.Lock()
		for _, c := range room.clients {
			if c.conn != conn {
				if err := c.conn.WriteMessage(messageType, []byte(username+": "+string(message))); err != nil {
					log.Println(err)
				}
			}
		}
		room.mu.Unlock()
	}
}

func setUpRoutes() {
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	setUpRoutes()
	log.Fatal(http.ListenAndServe(":9999", nil))
}
