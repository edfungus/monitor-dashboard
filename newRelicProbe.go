package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	NewRelicType             = "NewRelic"
	NewRelicMonitorNameKey   = "monitorName"
	NewRelicAccountNumberKey = "accountNumber"
	NewRelicIntervalKey      = "interval"
	NewRelicAPIEnvKey        = "apiKeyEnvVar"

	NewRelicBaseURL  = "https://insights-api.newrelic.com/v1/accounts/%s/query?nrql=%s"
	NewRelicNRQLBase = "SELECT monitorName, result FROM SyntheticCheck WHERE monitorName IN (%s) SINCE %d minutes ago LIMIT 100"
	bufferResultTime = 3 // (minutes) Always read this much more time in addition to interval providing a time overlap just in case

	MissingMonitorNameKey   = "%s is missing 'MonitorName' field in 'data' for New Relic probe"
	MissingProbeConfigValue = "Missing probe configuration value. Requires: accountNumber, interval, apiKeyEnvVar in data map"
)

// NewRelicProbeConfig is the configuration for a new relic probe
type NewRelicProbeConfig struct {
	id       string
	update   chan StatusUpdate
	interval time.Duration
	key      string
	account  string
}

// NewRelicProbe is a probe that checks new relic for statuses
type NewRelicProbe struct {
	refID      string
	account    string
	update     chan StatusUpdate
	interval   time.Duration
	ticker     *time.Ticker
	stopChan   chan bool
	key        string
	requestURI string
	statuses   map[string]*StatusUpdate // key: new relic monitor name
}

// NewRelicResponse
type NewRelicResponse struct {
	Results []struct {
		Events []struct {
			Timestamp   int    `json:"timestamp"`
			MonitorName string `json:"monitorName"`
			Result      string `json:"result"`
		} `json:"events"`
	} `json:"results"`
}

// NewNewRelicProbe returns a new relic probe
func NewNewRelicProbe(config *NewRelicProbeConfig) *NewRelicProbe {
	return &NewRelicProbe{
		refID:    config.id,
		account:  config.account,
		update:   config.update,
		interval: config.interval,
		stopChan: make(chan bool),
		key:      config.key,
		statuses: make(map[string]*StatusUpdate),
	}
}

// Initialize parses through all status to find which ones needs to be update via new relic
func (nr *NewRelicProbe) Initialize(statuses []*Status) error {
	err := nr.createCache(nr.refID, statuses, "")
	if err != nil {
		return err
	}
	nr.requestURI = createRequestURL(nr.statuses, nr.account, nr.interval)
	return nil
}

// Start begins sending requests at given interval to new relic
func (nr *NewRelicProbe) Start() {
	if nr.ticker == nil {
		logger.Debug("Starting New Relic Probe", "refID", nr.refID)
		nr.ticker = time.NewTicker(nr.interval)
		nr.probe()
		for {
			select {
			case <-nr.ticker.C:
				nr.probe()
			case <-nr.stopChan:
				logger.Debug("Exiting New Relic Probe", "refID", nr.refID)
				return
			}
		}
	}
}

// Stops sending requests to new relic for updates
func (nr *NewRelicProbe) Stop() {
	nr.stopChan <- true
	nr.ticker.Stop()
	nr.ticker = nil
	return
}

func (nr *NewRelicProbe) probe() {
	nrn, err := nr.requestNewRelic()
	if err != nil {
		logger.Error(err.Error())
		// should return to error chan so it shows up on the frontend
		// go nr.Stop() ... need to do that ^^^^^
		return
	}
	updatedMonitors := nr.updateCache(nrn)
	nr.sendUpdatesForMonitors(updatedMonitors)
}

// createCache creates the map in which future new relic requests will be based off of
// This will recursively go through statuses. id accumlates as we traverse to give the correct id to update on the frontend
func (nr *NewRelicProbe) createCache(id string, statuses []*Status, statusID string) error {
	for _, s := range statuses {
		var fullStatusID string
		if statusID == "" {
			fullStatusID = s.ID
		} else {
			fullStatusID = statusID + IdDelimiter + s.ID
		}

		if s.Probe.RefID == id {
			monitorName, ok := s.Probe.Data[NewRelicMonitorNameKey]
			if !ok {
				return fmt.Errorf(MissingMonitorNameKey, s.FullName)
			}

			nr.statuses[monitorName] = &StatusUpdate{
				ID:               fullStatusID,
				Status:           s.Status,
				lastUpdateMillis: 0,
			}
		}

		if len(s.Children) > 0 {
			nr.createCache(id, s.Children, fullStatusID)
		}
	}
	return nil
}

// updateCacheEntry will update the cache status only if the timestamp is after the timestamp in the cache
// boolean of whether an update occurred will be returned
func (nr *NewRelicProbe) updateCacheEntry(monitorName, status string, lastUpdateMillis int) bool {
	s, ok := nr.statuses[monitorName]
	if !ok {
		logger.Debug("Could not find in cache", "monitorName", monitorName)
		return false
	}
	if lastUpdateMillis < s.lastUpdateMillis {
		return false
	}
	s.lastUpdateMillis = lastUpdateMillis
	s.Status = convertNRtoMonitor(status)
	return true
}

// updateCache will update all of the cache and return the monitor names which were updated
func (nr *NewRelicProbe) updateCache(nrn NewRelicResponse) []string {
	updatedMonitors := []string{}
	for _, events := range nrn.Results {
		for _, event := range events.Events {
			updated := nr.updateCacheEntry(event.MonitorName, event.Result, event.Timestamp)
			if updated {
				updatedMonitors = append(updatedMonitors, event.MonitorName)
			}
		}
	}
	return updatedMonitors
}

// sendUpdatesForMonitors passes StatusUpdate to channel for a list of monitor names
func (nr *NewRelicProbe) sendUpdatesForMonitors(monitorNames []string) {
	logger.Debug("Got New Relic monitor statuses", "refID", nr.refID, "count", len(monitorNames))
	for _, name := range monitorNames {
		go func(monitorName string) {
			update, ok := nr.statuses[monitorName]
			if ok {
				nr.update <- *update
			}
		}(name)
	}
}

func (nr *NewRelicProbe) requestNewRelic() (NewRelicResponse, error) {
	logger.Debug("Making call to New Relic", "refID", nr.refID, "url", nr.requestURI)
	client := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, nr.requestURI, nil)
	if err != nil {
		return NewRelicResponse{}, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Query-Key", nr.key)

	resp, err := client.Do(req)
	if err != nil {
		return NewRelicResponse{}, err
	}
	var b bytes.Buffer
	b.ReadFrom(resp.Body)

	var nrn NewRelicResponse
	err = json.Unmarshal(b.Bytes(), &nrn)
	if err != nil {
		return NewRelicResponse{}, err
	}

	if resp.StatusCode > 200 {
		logger.Error("Bad status code form New Relic", "refID", nr.refID, "status code", resp.StatusCode)
	}

	return nrn, nil
}

// Creates the request URL to new relic from cache
func createRequestURL(cache map[string]*StatusUpdate, account string, interval time.Duration) string {
	monitorNames := monitorNamesFromCache(cache)
	query := createNRQLQuery(monitorNames, interval)
	newRelicRequestURL := fmt.Sprintf(NewRelicBaseURL, account, url.PathEscape(query))
	return newRelicRequestURL
}

func createNRQLQuery(monitorNames []string, interval time.Duration) string {
	names := []string{}
	for _, monitorName := range monitorNames {
		names = append(names, fmt.Sprintf("'%s'", monitorName))
	}
	monitorList := strings.Join(names, ",")
	time := int(math.Ceil(float64(interval.Minutes())) + bufferResultTime) //min is 1 and no decimals
	query := fmt.Sprintf(NewRelicNRQLBase, monitorList, time)
	return query
}

func monitorNamesFromCache(cache map[string]*StatusUpdate) []string {
	monitorNames := []string{}
	for monitorName := range cache {
		monitorNames = append(monitorNames, monitorName)
	}
	return monitorNames
}
