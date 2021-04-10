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
	key := fmt.Sprintf("%p", conn) // just use the pointer location as unique key
	w.connections[key] = conn
	w.mutex.Unlock()
	go func() {
		for {
			_, msgBytes, err := conn.ReadMessage()
			msg := string(msgBytes)
			if err != nil {
				w.closeConnection(key)
				return
			}
			if msg == "close" {
				w.closeConnection(key)
				return
			}
		}
	}()
}

func (w *wsConnHandler) closeConnection(key string) {
	w.mutex.Lock()
	if w.connections[key] != nil {
		w.connections[key].Close()
		w.connections[key] = nil
	}
	w.mutex.Unlock()
}

func (w *wsConnHandler) sendUpdateMsg() {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	for key, conn := range w.connections {
		if conn == nil {
			continue
		}
		err := conn.WriteMessage(1, []byte("update"))
		if err != nil {
			w.closeConnection(key)
		}
	}
}

func genRandomString() {
	strBld := strings.Builder{}
	strBld.Grow(32)
	for i := 0; i < 32; i++ {
	}
}
