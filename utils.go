package main

import (
	"errors"
)

const (
	nrGood = "SUCCESS"
	nrBad  = "FAILED"

	monitorGood     = "good"
	monitorBad      = "bad"
	monitorDegraded = "degraded"
	monitorUnknown  = "unknown"
)

func convertNRtoMonitor(nrString string) string {
	switch nrString {
	case nrGood:
		return monitorGood
	case nrBad:
		return monitorBad
	default:
		logger.Debug("Recieved unknown status from new relic", "status", nrString)
		return monitorUnknown
	}
}

func checkForMapKeys(m map[string]string, requiredKeys []string) error {
	for _, key := range requiredKeys {
		if m[key] == "" {
			return errors.New("Missing required key: " + key)
		}
	}
	return nil
}
