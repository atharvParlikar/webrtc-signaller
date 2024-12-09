package main

import (
	"encoding/json"
	"log"
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

// ConnectionManager handles WebSocket connections
type ConnectionManager struct {
	connections map[string]*websocket.Conn
	mutex       sync.RWMutex
}

// NewConnectionManager creates a new ConnectionManager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*websocket.Conn),
	}
}

// Add a new connection
func (cm *ConnectionManager) Add(id string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[id] = conn
}

// Remove a connection
func (cm *ConnectionManager) Remove(id string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.connections, id)
}

// Get a connection
func (cm *ConnectionManager) Get(id string) (*websocket.Conn, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	conn, exists := cm.connections[id]
	return conn, exists
}

// SignalMessageSdp represents the structure of WebRTC signaling messages
type SignalMessageSdp struct {
	SignalType string `json:"signalType"`
	UserID     string `json:"userId"`
	SDP        string `json:"sdp_base64"`
}

type SignalMessageCandidate struct {
	SignalType string `json:"signalType"`
	UserID     string `json:"userId"`
	Candidate  string `json:"candidate"`
}

// WebSocketServer manages WebSocket connections and signaling
type WebSocketServer struct {
	connectionManager *ConnectionManager
}

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		connectionManager: NewConnectionManager(),
	}
}

// handleConnection manages a single WebSocket connection
func (ws *WebSocketServer) handleConnection(conn *websocket.Conn) {
	// Generate unique connection ID
	id := uuid.New().String()
	log.Printf("[%s] Client connected 🙌\n", id)

	// Add connection to manager
	ws.connectionManager.Add(id, conn)
	defer ws.closeConnection(id, conn)

	// Send connection ID to client
	if err := conn.WriteJSON(map[string]string{"userId": id}); err != nil {
		log.Printf("❌ Failed to send user ID: %v\n", err)
		return
	}

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()

		var genericMessage struct {
			SignalType string `json:signalType`
		}

		err = json.Unmarshal(message, &genericMessage)

		if err != nil {
			log.Printf("❌ Error Parsing Signal Message: %v\n", err)
		}

		switch genericMessage.SignalType {
		case "sdp":
			var messageJson SignalMessageSdp
			json.Unmarshal(message, &messageJson)
			ws.forwardSignal(id, messageJson)

		case "candidate":
			var messageJson SignalMessageCandidate
			json.Unmarshal(message, &messageJson)
			ws.forwardSignal(id, messageJson)

		}

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("❌ Unexpected close error: %v\n", err)
			}
			break
		}

		log.Printf("Received from %s: %+v\n", id, message)
	}
}

// forwardSignal routes signaling messages between clients
func (ws *WebSocketServer) forwardSignal(senderID string, message SignalMessageSdp) {
	// Preserve targetId in a variable
	targetID := message.UserID

	// Modify message to include sender's ID
	message.UserID = senderID

	// Get target connection
	targetConn, exists := ws.connectionManager.Get(targetID)
	if !exists {
		log.Printf("❌ Target connection not found: %s\n", message.UserID)
		return
	}

	// Forward message
	if err := targetConn.WriteJSON(message); err != nil {
		log.Printf("❌ Failed to forward message: %v\n", err)
	}
}

// closeConnection handles connection cleanup
func (ws *WebSocketServer) closeConnection(id string, conn *websocket.Conn) {
	log.Printf("[%s] Connection closed 🔥\n", id)
	conn.Close()
	ws.connectionManager.Remove(id)
}

// handleWebSocket is the HTTP handler for WebSocket connections
func (ws *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ Failed to upgrade to WebSocket: %v\n", err)
		return
	}
	ws.handleConnection(conn)
}

func main() {
	server := NewWebSocketServer()

	http.HandleFunc("/ws", server.handleWebSocket)

	log.Println("WebSocket server started on ws://localhost:8080/ws")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Context:
// You are implementing candidates exchange feature, the frontend is done now is the time to
// also implement it in the signaling server, but for that forwardSignal needs to accept 2 types
// SignalMessageSdp and SignalMessageCandidate, because go is a fucking retarded language
// we cannot just use | to do or, we need to do some fucking generic type bullshit.
// just copy that shit over from ChatGPT and get done with this language and back to typescript.
