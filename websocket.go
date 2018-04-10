package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// StatusUpdate is the payload sent the to frontend for status update
type StatusUpdate struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	lastUpdateMillis int    `json:"lastUpdateMillis"`
}

type tmpl struct {
	routePath string
	template  *template.Template
	data      interface{}
}

// Display keeps track of the client to send updates
type Display struct {
	clients map[*client]bool
	tmpls   []tmpl
}

// NewDisplay returns a new display
func NewDisplay(name string) *Display {
	d := &Display{
		clients: map[*client]bool{},
		tmpls:   []tmpl{},
	}
	d.tmpls = append(d.tmpls, tmpl{
		routePath: "/components/container-header/container-header.html",
		template:  template.Must(template.ParseFiles("./public/components/container-header/container-header.html")),
		data:      name,
	})
	return d
}

// RouteStatic routes the static webpage ... must be routed last
func (d *Display) RouteStatic(router *mux.Router) {
	d.addTemplatePages(router)
	routeStaticWebApp(router, "./public")
}

// AddClient adds a client to be sent updates
func (d *Display) AddClient(c *client) {
	d.clients[c] = true
}

// RemoveClient removes a client
func (d *Display) RemoveClient(c *client) {
	for client := range d.clients {
		if client == c {
			delete(d.clients, client)
			c.close()
		}
	}
}

// Send sends status update to all connected clients
func (d *Display) Send(su StatusUpdate) error {
	payload, err := json.Marshal(su)
	if err != nil {
		return err
	}
	for c := range d.clients {
		go func(c *client) {
			c.send <- payload
		}(c)
	}
	return nil
}

//LiveStatus is the handler for the websocket endpoint
func (d *Display) LiveStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("ws client connection opened")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Could not upgrade to ws", "error", err)
			return
		}
		client := &client{
			conn:      conn,
			send:      make(chan []byte),
			heartbeat: 1 * time.Second,
		}
		d.AddClient(client)
		go client.process(d)
	})
}

func (d *Display) addTemplatePages(r *mux.Router) {
	for _, tmpl := range d.tmpls {
		r.HandleFunc(tmpl.routePath, func(w http.ResponseWriter, r *http.Request) {
			tmpl.template.Execute(w, tmpl.data)
		})
	}
}

func routeStaticWebApp(r *mux.Router, dir string) {
	public := http.FileServer(http.Dir(dir))
	r.PathPrefix("/").Handler(public)
}

// client handles the actual connection to the client/webpage
type client struct {
	conn      *websocket.Conn
	send      chan []byte
	heartbeat time.Duration
}

// close closes connection and channel
func (c *client) close() {
	close(c.send)
	c.conn.Close()
}

// process sends messages from channel to the websocket connection
func (c *client) process(d *Display) {
	defer func() {
		logger.Debug("ws client connection closed")
		d.RemoveClient(c)
	}()
	ticker := time.NewTicker(c.heartbeat)
	for {
		select {
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case payload := <-c.send:
			err := writeToConnection(c.conn, payload)
			if err != nil {
				logger.Error("Could not make ws writer", "error", err)
				continue
			}
		}
	}
}

func writeToConnection(conn *websocket.Conn, payload []byte) error {
	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	w.Write(payload)
	w.Close()
	return nil
}
