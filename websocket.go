package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	websocketPort = 8081
)

// websocketServer is in charge of handling websocket communication in order to reload web clients
type websocketServer struct {
	wsHandler     *websocket.Upgrader
	wsConnHandler *wsConnHandler
}

func newWebsocketServer() *websocketServer {
	return &websocketServer{
		wsHandler: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsConnHandler: newWSConnHandler(),
	}
}

// start starts the dev web server
func (s *websocketServer) start() error {
	return http.ListenAndServe(":"+strconv.Itoa(websocketPort), s)
}

func (s *websocketServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	conn, err := s.wsHandler.Upgrade(res, req, http.Header{})
	if err != nil {
		fmt.Println("error upgrading websocket request:", err)
		return
	}

	s.wsConnHandler.registerConn(conn)
}

// wsConnHandler can handle mutiple websocket connections synchronously
type wsConnHandler struct {
	connections map[string]*websocket.Conn
	mutex       sync.RWMutex
}

func newWSConnHandler() *wsConnHandler {
	return &wsConnHandler{
		connections: map[string]*websocket.Conn{},
		mutex:       sync.RWMutex{},
	}
}

func (w *wsConnHandler) registerConn(conn *websocket.Conn) {
	w.mutex.Lock()
	key := fmt.Sprintf("%p", conn)
	fmt.Println("our key is:", key)
	w.connections[key] = conn
	w.mutex.Unlock()
	go func() {
		for {
			_, msgBytes, err := conn.ReadMessage()
			msg := string(msgBytes)
			if err != nil {
				fmt.Println("error reading websocket message:", err)
				continue
			}
			fmt.Println("received message from websocket client:", msg)
			if msg == "close" {
				w.closeConnection(key)
			}
		}
	}()
}

func (w *wsConnHandler) closeConnection(key string) {
	w.mutex.Lock()
	fmt.Println("closing websocket connection with key:", key)
	w.connections[key] = nil
	w.mutex.Unlock()
}

func (w *wsConnHandler) sendUpdateMsg(key string) {
	// TODO: send update message to all connections
}

func genRandomString() {
	strBld := strings.Builder{}
	strBld.Grow(32)
	for i := 0; i < 32; i++ {
	}
}
