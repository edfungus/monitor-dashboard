package main

import (
	"errors"
	"os"
	"time"
)

type Probe interface {
	Start()
	Stop()
}

// Probe defines and contains information to create a new probe like a NewRelicProbe
type ProbeDef struct {
	ID   string            `json:"id"`
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

// ProbeRef references which probe to use along wiht any data
type ProbeRef struct {
	RefID string            `json:"probeRefId"`
	Data  map[string]string `json:"data"`
}

// CreateProbes create the probes defined in the config
func CreateProbes(probeDefs []ProbeDef, statuses []*Status, updateChan chan StatusUpdate) ([]Probe, error) {
	probes := []Probe{}
	for _, probe := range probeDefs {
		switch probe.Type {
		case NewRelicType:
			logger.Debug("Creating new probe", "refID", probe.ID, "type", probe.Type)
			err := checkForMapKeys(probe.Data, []string{NewRelicIntervalKey, NewRelicAPIEnvKey, NewRelicAccountNumberKey})
			if err != nil {
				return nil, errors.New("Error making probe " + probe.ID + " with error: " + err.Error())
			}
			probeInterval, err := time.ParseDuration(probe.Data[NewRelicIntervalKey])
			if err != nil {
				return nil, err
			}
			nrpc := &NewRelicProbeConfig{
				id:       probe.ID,
				update:   updateChan,
				interval: probeInterval,
				key:      os.Getenv(probe.Data[NewRelicAPIEnvKey]),
				account:  probe.Data[NewRelicAccountNumberKey],
			}
			nrp := NewNewRelicProbe(nrpc)
			err = nrp.Initialize(statuses)
			if err != nil {
				logger.Critical("Could not initialize New Relic probe")
				return nil, err
			}
			probes = append(probes, nrp)
		default:
			logger.Critical("Unknown probe type", "probe type", probe.Type)
		}
	}
	return probes, nil
}

func StartProbes(probes []Probe) {
	for _, probe := range probes {
		go probe.Start()
	}
}

func StopProbes(probes []Probe) {
	for _, probe := range probes {
		probe.Stop()
	}
}
