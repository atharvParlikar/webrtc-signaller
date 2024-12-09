package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Note to self:
//
//	Appearently browsers only initiate a connection with http/https
//	so if a browser wants to connect to a websocket server it needs
//	to 'Upgrade' the initial connection to websocket thus this upgrader
//	is needed here we are just changing an attribute of the struct upgrader
//	i.e the checkOrigin function.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var connections = make(map[string]*websocket.Conn)
var connectionsMutex = &sync.Mutex{}

type MessageJsonType struct {
	SignalType string `json:"signalType"`
	UserId     string `json:"userId"`
	Sdp        string `json:sdp`
}

func handleConnClose(id string) {
	fmt.Printf("[%s] Connection closed 🔥\n", id)

	connectionsMutex.Lock()
	delete(connections, id)
	connectionsMutex.Unlock()
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		fmt.Println("❌ Failed to upgrade to WebSocket: ", err)
		return
	}
	defer conn.Close()

	id := uuid.New().String()

	fmt.Printf("[%s] Client connected 🙌\n", id)

	connectionsMutex.Lock()
	connections[id] = conn
	connectionsMutex.Unlock()

	err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"userId": "%s"}`, id)))
	if err != nil {
		fmt.Println("❌ Failed to send user their id, closing connection...\nError: ", err)
		handleConnClose(id)
		return
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if err.Error() == "websocket: close 1001 (going away)" {
				break
			}
			fmt.Printf("❌ Error reading message from %s: %s\n", id, err)
		}

		var messageJson MessageJsonType

		fmt.Printf("Recieved from %s: %s\n", id, message)

		err = json.Unmarshal(message, &messageJson)

		if messageJson.SignalType == "offer" {
			fmt.Println("Got offer")
			connectionsMutex.Lock()
			connections[messageJson.UserId].WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"userId": "%s", "sdp": "%s", "signalType": "offer"}`, id, messageJson.Sdp)))
			connectionsMutex.Unlock()
		} else if messageJson.SignalType == "answer" {
			fmt.Println("Got answer")
			connectionsMutex.Lock()
			connections[messageJson.UserId].WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"userId": "%s", "sdp": "%s", "signalType": "answer"}`, id, messageJson.Sdp)))
			connectionsMutex.Unlock()
		}

		if err != nil {
			fmt.Println("Error parsing JSON message: ", err)
		}

		if messageJson.SignalType == "offer" {

		}

	}
	handleConnClose(id)
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)

	fmt.Println("WebSocket server started on ws://localhost:8080/ws")
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		fmt.Println("Failed to start server: ", err)
	}
}
