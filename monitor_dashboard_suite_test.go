package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServiceMonitor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MonitorDashboard Suite")
}
