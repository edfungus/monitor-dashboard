package main

import (
	"fmt"
	"time"

	"github.com/gorilla/mux"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceMonitor", func() {
	child4 := &Status{
		ID: "child4",
		Probe: ProbeRef{
			Type: "NewRelic",
			Data: map[string]string{
				MonitorNameKey: "monitor4",
			},
		},
	}
	child3 := &Status{
		ID:     "child3",
		Status: "good",
		Probe: ProbeRef{
			Type: "NewRelic",
			Data: map[string]string{
				MonitorNameKey: "monitor3",
			},
		},
	}
	child2 := &Status{
		ID:       "child2",
		Children: []*Status{child3, child4},
	}
	child1 := &Status{
		ID: "child1",
		Probe: ProbeRef{
			Type: "NewRelic",
			Data: map[string]string{},
		},
	}
	parent := &Status{
		ID:       "parent",
		Children: []*Status{child1, child2},
	}
	statuses := []*Status{parent}

	Describe("Monitor", func() {
		Describe("Given a tree of statuses", func() {
			Context("When trying to find a nested status", func() {
				It("Then it should be found", func() {
					status, err := FindStatus("parent#child2#child3", statuses)
					Expect(err).To(BeNil())
					Expect(status).To(Equal(child3))

					status, err = FindStatus("parent#child1", statuses)
					Expect(err).To(BeNil())
					Expect(status).To(Equal(child1))
				})
			})
			Context("When trying to find a parent status with children", func() {
				It("Then it should be found along with the children", func() {
					status, err := FindStatus("parent", statuses)
					Expect(err).To(BeNil())
					Expect(status).To(Equal(parent))

					status, err = FindStatus("parent#child2", statuses)
					Expect(err).To(BeNil())
					Expect(status).To(Equal(child2))
				})
			})
			Context("When trying to find a status that doesn't exist", func() {
				It("Then it should not be found", func() {
					_, err := FindStatus("unknown", statuses)
					Expect(err).ToNot(BeNil())

					_, err = FindStatus("parent#child3", statuses)
					Expect(err).ToNot(BeNil())
				})
			})
		})

		Describe("Given a Monitor", func() {
			mc := &MonitorConfig{
				Router:   mux.NewRouter(),
				Statuses: statuses,
			}
			m := NewMonitor(mc)
			Context("When a Status exist", func() {
				It("Then it can be updated", func() {
					child3.Status = "bad"
					err := m.UpdateStatusByID(StatusUpdate{
						ID:     "parent#child2#child3",
						Status: "good",
					})
					Expect(err).To(BeNil())
					Expect(child3.Status).To(Equal("good"))
				})
			})
			Context("When a Status does not exist", func() {
				It("Then it will error when updated", func() {
					err := m.UpdateStatusByID(StatusUpdate{
						ID:     "unknown",
						Status: "good",
					})
					Expect(err).ToNot(BeNil())
				})
			})
		})
	})

	Describe("Probe", func() {
		Describe("Given creating status cache for New Relic Probe", func() {
			config := &NewRelicProbeConfig{}
			nr := NewNewRelicProbe(config)
			Context("When the probe type and monitor name exist", func() {
				It("Then it should be added to the cache correctly", func() {
					err := nr.Initialize([]*Status{child3})
					Expect(err).To(BeNil())

					Expect(nr.statuses).To(HaveKey("monitor3"))
					Expect(*nr.statuses["monitor3"]).To(Equal(StatusUpdate{
						ID:               child3.ID,
						Status:           child3.Status,
						lastUpdateMillis: 0,
					},
					))
				})
			})
			Context("When the probe type and monitor name exist in children", func() {
				It("Then they should be added to the cache correctly", func() {
					err := nr.Initialize([]*Status{child2})
					Expect(err).To(BeNil())

					Expect(nr.statuses).To(HaveKey("monitor3"))
					Expect(*nr.statuses["monitor3"]).To(Equal(StatusUpdate{
						ID:               child2.ID + IdDelimiter + child3.ID,
						Status:           child3.Status,
						lastUpdateMillis: 0,
					},
					))
					Expect(nr.statuses).To(HaveKey("monitor4"))
					Expect(*nr.statuses["monitor4"]).To(Equal(StatusUpdate{
						ID:               child2.ID + IdDelimiter + child4.ID,
						Status:           child4.Status,
						lastUpdateMillis: 0,
					},
					))
				})
			})
			Context("When the monitor name does not exist", func() {
				It("Then we should error", func() {
					err := nr.Initialize([]*Status{child1})
					Expect(err).ToNot(BeNil())
				})
			})
		})

		Describe("Given creating New Relic query", func() {
			monitorNames := []string{"name1", "name2"}
			Context("When cache is empty", func() {
				It("Then query should not have monitor names", func() {
					monitorNames := []string{}
					interval, _ := time.ParseDuration("5m")
					query := createNRQLQuery(monitorNames, interval)
					expected := fmt.Sprintf(NewRelicNRQLBase, "", 8)
					Expect(query).To(Equal(expected))
				})
			})
			Context("When cache has monitor names", func() {
				It("Then query should have monitor names correctly delimited and time in the query should have that time plus buffer time", func() {
					interval, _ := time.ParseDuration("5m")
					query := createNRQLQuery(monitorNames, interval)
					expected := fmt.Sprintf(NewRelicNRQLBase, "'name1','name2'", 8)
					Expect(query).To(Equal(expected))
				})
			})
			Context("When time interval is given not in minutes", func() {
				It("Then time in the query should be properly converted (seconds)", func() {
					interval, _ := time.ParseDuration("30s")
					query := createNRQLQuery(monitorNames, interval)
					expected := fmt.Sprintf(NewRelicNRQLBase, "'name1','name2'", 4)
					Expect(query).To(Equal(expected))
				})
				It("Then time in the query should be properly converted (hours)", func() {
					interval, _ := time.ParseDuration("2h")
					query := createNRQLQuery(monitorNames, interval)
					expected := fmt.Sprintf(NewRelicNRQLBase, "'name1','name2'", 123)
					Expect(query).To(Equal(expected))
				})
			})
		})
		Describe("Given a built StatusUpdate cache", func() {
			Context("When the cache has content", func() {
				It("Then the monitor names should be returned", func() {
					monitorNames := []string{"name1", "name2"}
					cache := make(map[string]*StatusUpdate)
					for _, name := range monitorNames {
						cache[name] = &StatusUpdate{}
					}
					names := monitorNamesFromCache(cache)
					Expect(names).To(ConsistOf(monitorNames))
				})
			})
			Context("When the cache has no content", func() {
				It("Then an empty array should be returned", func() {
					cache := make(map[string]*StatusUpdate)
					names := monitorNamesFromCache(cache)
					Expect(names).To(BeEmpty())
				})
			})
		})
		Describe("Given creating the New Relic requets URL", func() {
			Context("When the query is given", func() {
				It("Then the query should be url encoded and append to base URL", func() {
					monitorNames := []string{"name1", "name2"}
					cache := make(map[string]*StatusUpdate)
					for _, name := range monitorNames {
						cache[name] = &StatusUpdate{}
					}
					interval, _ := time.ParseDuration("5m")
					query := createRequestURL(cache, interval)
					expected := "https://insights-api.newrelic.com/v1/accounts/918250/query?nrql=SELECT%20monitorName%2C%20result%20FROM%20SyntheticCheck%20WHERE%20monitorName%20IN%20%28%27name1%27%2C%27name2%27%29%20SINCE%208%20minutes%20ago%20LIMIT%20100"
					Expect(query).To(Equal(expected))
				})
			})
		})
	})
})
