// PRIVATE CHAT
// 1. connected user with self username
// 2. send message as 'receiver: content'
// 3. send message content to receiver if receiver connected to web-socket
// 4. receive message from receiver
// 5. continue process

package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strings"
	"sync"
)

var socketUpgrade = websocket.Upgrader{
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

var (
	clients   = make(map[string]*Client)
	clientsMu sync.Mutex
)

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	conn, err := socketUpgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error while upgrading connection:", err)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		log.Println("Username query param is required")
		err := conn.Close()
		if err != nil {
			return
		}
		return
	}

	client := &Client{conn: conn, username: username}
	clientsMu.Lock()
	clients[username] = client
	clientsMu.Unlock()

	log.Println("Client", username, "connected")

	if err := conn.WriteMessage(websocket.TextMessage, []byte("Hi "+username+"! You are connected.")); err != nil {
		log.Println("Error sending connection confirmation:", err)
		clientsMu.Lock()
		delete(clients, username)
		clientsMu.Unlock()
		err := conn.Close()
		if err != nil {
			return
		}
		return
	}

	go reader(client)
}

func reader(client *Client) {
	conn := client.conn
	username := client.username

	defer func() {
		clientsMu.Lock()
		err := conn.Close()
		if err != nil {
			return
		}
		delete(clients, username)
		clientsMu.Unlock()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return
		}

		log.Println(username+" sent message:", string(message))

		// Format message as "toUsername: message"
		parts := strings.SplitN(string(message), ":", 2)
		if len(parts) < 2 {
			err := conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format. Use 'toUsername: message'."))
			if err != nil {
				return
			}
			continue
		}

		toUsername := strings.TrimSpace(parts[0])
		textMessage := strings.TrimSpace(parts[1])

		clientsMu.Lock()
		toClient, ok := clients[toUsername]
		clientsMu.Unlock()

		if ok {
			if err := toClient.conn.WriteMessage(websocket.TextMessage, []byte(username+": "+textMessage)); err != nil {
				log.Println("Error sending message to", toUsername, ":", err)
			}
		} else {
			if err := conn.WriteMessage(websocket.TextMessage, []byte("User "+toUsername+" not found")); err != nil {
				log.Println("Error sending user not found message:", err)
			}
		}
	}
}

func setUpRoutes() {
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	setUpRoutes()
	log.Fatal(http.ListenAndServe(":9999", nil))
}
