package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const (
	IdDelimiter = "#"
)

var (
	StatusNotFound = errors.New("Status not found")
)

// Monitor is in charge of managing all the status
type Monitor struct {
	statuses []*Status
	display  *Display
	username string
	password string
	update   chan StatusUpdate
}

// MonitorConfig is the config to create a Monitor
type MonitorConfig struct {
	Router   *mux.Router
	Statuses []*Status
	Username string
	Password string
	Name     string
}

// Statuses is the json being pass in and out of the service (to frontend).
// It is also the structure used to define the structure of statuses to initialize the application
type Statuses struct {
	Statuses []*Status `json:"statuses"`
}

// Status is the core unit to anything that has a good/degraded/bad/unknown status.
type Status struct {
	ID         string    `json:"id"`
	FullName   string    `json:"fullName"`
	AbbrevName string    `json:"abbrevName"`
	SubText    string    `json:"subText"`
	Status     string    `json:"status"`
	Children   []*Status `json:"children"`
	URL        string    `json:"url"`
	Probe      ProbeRef  `json:"probe"`
}

// NewMonitor returns a new Monitor
func NewMonitor(config *MonitorConfig) *Monitor {
	monitor := &Monitor{
		statuses: config.Statuses,
		username: config.Username,
		password: config.Password,
		update:   make(chan StatusUpdate),
	}

	config.Router.Handle("/status", monitor.basicAuth(monitor.getStatusHandler(), true)).Methods(http.MethodGet)
	config.Router.Handle("/login", monitor.basicAuth(login(), false)).Methods(http.MethodGet)
	// This is not very REST friendly but will make updating less complicated
	config.Router.Handle("/update/{id}/{status}", monitor.basicAuth(monitor.updateStatusHandler(), true)).Methods(http.MethodGet)

	monitor.display = NewDisplay(config.Name)
	config.Router.Handle("/live", monitor.basicAuth(monitor.display.LiveStatus(), true))
	monitor.display.RouteStatic(config.Router)

	go monitor.updateListener()

	return monitor
}

func (m *Monitor) updateListener() {
	for {
		status := <-m.update
		err := m.UpdateStatusByID(status)
		if err != nil {
			logger.Error(err.Error())
			// throw to error chan to go to show up in frontend
		}
	}
}

func (m *Monitor) basicAuth(h http.Handler, prompt bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, isOk := r.BasicAuth()
		if !isOk || username != m.username || password != m.password {
			if prompt {
				w.Header().Set("WWW-Authenticate", `Basic realm=""`)
			}
			w.WriteHeader(401)
			w.Write([]byte("Unauthorized\n"))
			return
		}
		h.ServeHTTP(w, r)
	})
}

func login() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (m *Monitor) getStatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ss := Statuses{
			Statuses: m.statuses,
		}
		json, _ := json.Marshal(ss)
		w.Write(json)
		w.WriteHeader(http.StatusOK)
	})
}

func (m *Monitor) updateStatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		su := StatusUpdate{
			ID:     vars["id"],
			Status: vars["status"],
		}
		err := m.UpdateStatusByID(su)
		switch {
		case err == StatusNotFound:
			w.WriteHeader(http.StatusNotFound)
			return
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			return
		default:
			w.WriteHeader(http.StatusOK)
			return
		}
	})
}

// UpdateStatusByID updates the status where the id is concatenation of ids from parent to target status
func (m *Monitor) UpdateStatusByID(su StatusUpdate) error {
	s, err := FindStatus(su.ID, m.statuses)
	if err != nil {
		logger.Debug("Could not find status", "id", su.ID)
		return err
	}
	if s.Status != su.Status {
		s.Status = su.Status
		logger.Debug("New status update!", "update", su)
		return m.display.Send(su)
	}
	return nil
}

func FindStatus(id string, statuses []*Status) (*Status, error) {
	ids := strings.SplitN(id, IdDelimiter, 2)
	currentID := ids[0]
	for _, s := range statuses {
		if s.ID == currentID {
			if len(ids) >= 2 {
				nextIDs := ids[1]
				return FindStatus(nextIDs, s.Children)
			}
			return s, nil
		}
	}
	logger.Debug("Could not find id during traversal", "id", currentID)
	return nil, StatusNotFound
}

func (m *Monitor) GetUpdateChan() chan StatusUpdate {
	return m.update
}
