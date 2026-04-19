package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spq/pkappa2/internal/index/manager"
)

type (
	dirs struct {
		base, pcap, index, snapshot, state, converter, watch string
	}
	websocketWrapper struct {
		ws *websocket.Conn
	}
)

func NewWebsocketWrapper(url string) (*websocketWrapper, error) {
	wsURL := "ws" + strings.TrimPrefix(url, "http") + "/ws"
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &websocketWrapper{ws: ws}, nil
}

func (w *websocketWrapper) Shutdown(t *testing.T) {
	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Bye")
	if err := w.ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(writeWait)); err != nil {
		t.Fatalf("could not send close message to WebSocket server: %v", err)
	}
	timeStart := time.Now()
	if err := w.ws.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("could not set read deadline on WebSocket connection: %v", err)
	}
	for _, _, err := w.ws.ReadMessage(); reflect.TypeOf(err) != reflect.TypeOf(&websocket.CloseError{}); {
		if time.Until(timeStart) > time.Minute {
			break
		}
	}
	w.ws.Close()
}

func (w *websocketWrapper) ReadEvent(eventName string) (*manager.Event, error) {
	messageType, message, err := w.ws.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("could not read message from WebSocket: %w", err)
	}
	if messageType != websocket.TextMessage {
		return nil, fmt.Errorf("expected message type %d, got %d", websocket.TextMessage, messageType)
	}
	var event manager.Event
	if err := json.Unmarshal(message, &event); err != nil {
		return nil, fmt.Errorf("could not unmarshal WebSocket message: %w", err)
	}
	if event.Type != eventName {
		return nil, fmt.Errorf("expected event type '%s', got '%s'", eventName, event.Type)
	}
	return &event, nil
}

func makeTempdirs(t *testing.T) dirs {
	dirs := dirs{
		base: t.TempDir(),
	}
	dirs.pcap = path.Join(dirs.base, "pcap") + "/"
	dirs.index = path.Join(dirs.base, "index") + "/"
	dirs.state = path.Join(dirs.base, "state") + "/"
	dirs.snapshot = path.Join(dirs.base, "snapshot") + "/"
	dirs.converter = path.Join(dirs.base, "converter") + "/"
	dirs.watch = path.Join(dirs.base, "watch") + "/"
	for _, p := range []string{dirs.pcap, dirs.index, dirs.snapshot, dirs.state, dirs.converter, dirs.watch} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatalf("Mkdir(%q) failed with error: %v", p, err)
		}
	}
	return dirs
}

func makeManager(t *testing.T, dirs dirs) *manager.Manager {
	mgr, err := manager.New(dirs.pcap, dirs.index, dirs.snapshot, dirs.state, dirs.converter, dirs.watch)
	if err != nil {
		t.Fatalf("manager.New failed with error: %v", err)
	}
	return mgr
}

func TestConfig(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	r := setupRouter(mgr, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/config returned status code %d, want 200", rr.Code)
	}
	var config manager.Config
	if err := json.NewDecoder(rr.Body).Decode(&config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if config.AutoInsertLimitToQuery {
		t.Fatalf("Config.AutoInsertLimitToQuery = true, want false")
	}

	config.AutoInsertLimitToQuery = true

	jsonBody, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBuffer(jsonBody))
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/config returned status code %d, want 200", rr.Code)
	}
}

func TestStatus(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	r := setupRouter(mgr, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/status.json", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/status returned status code %d, want 200", rr.Code)
	}
	var status manager.Statistics
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to unmarshal status: %v", err)
	}

	// TODO: Add pcaps and check the status
}

func TestWebsocket(t *testing.T) {
	dirs := makeTempdirs(t)
	mgr := makeManager(t, dirs)
	defer mgr.Close()
	r := setupRouter(mgr, nil, nil)
	server := httptest.NewServer(r)
	defer server.Close()

	ws, err := NewWebsocketWrapper(server.URL)
	if err != nil {
		t.Fatalf("could not connect to WebSocket server: %v", err)
	}
	defer ws.Shutdown(t)

	req, err := http.NewRequest(http.MethodPut, server.URL+"/api/tags", bytes.NewReader([]byte("port:1234")))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	q := req.URL.Query()
	q.Add("name", "service/service1")
	q.Add("color", "#ff0000")
	req.URL.RawQuery = q.Encode()
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("could not send request to server: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status code 200, got %d", res.StatusCode)
	}

	event, err := ws.ReadEvent("tagAdded")
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}
	if event.Tag.Name != "service/service1" {
		t.Fatalf("expected tag name 'service/service1', got '%s'", event.Tag.Name)
	}
	if event.Tag.Definition != "port:1234" {
		t.Fatalf("expected tag definition 'port:1234', got '%s'", event.Tag.Definition)
	}
	if event.Tag.Color != "#ff0000" {
		t.Fatalf("expected tag color '#ff0000', got '%s'", event.Tag.Color)
	}
}
