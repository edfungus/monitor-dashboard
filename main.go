package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"os"

	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	logging "github.com/op/go-logging"
)

var logger = logging.MustGetLogger("eevee")

// EnvVar has all the environment variables needed to run the service monitor
type EnvVar struct {
	Port               string `default:"3000"`
	ConfigFileLocation string `envconfig:"CONFIG_FILE" default:"config.json"`
	Username           string `envconfig:"USERNAME" default:"admin"`
	Password           string `envconfig:"PASSWORD"`
}

type Configuration struct {
	Name      string     `json:"dashboardName"`
	Statuses  []*Status  `json:"statuses"`
	ProbeDefs []ProbeDef `json:"probes"`
} 

func main() {
	ev := getEnv()
	r := mux.NewRouter()

	c := loadConfigurationFromFile(ev.ConfigFileLocation)
	mc := &MonitorConfig{
		Router:   r,
		Statuses: c.Statuses,
		Username: ev.Username,
		Password: ev.Password,
		Name:     c.Name,
	}

	m := NewMonitor(mc)

	p, err := CreateProbes(c.ProbeDefs, c.Statuses, m.GetUpdateChan())
	if err != nil {
		log.Fatal("Could not create Probe. " + err.Error())
	}
	StartProbes(p)
	defer StopProbes(p)

	log.Println("Starting servic monitor on port: " + ev.Port)
	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + ev.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func getEnv() EnvVar {
	var ev EnvVar
	err := envconfig.Process("", &ev)
	if err != nil {
		log.Fatal(err.Error())
	}
	return ev
}

func loadConfigurationFromFile(fileLocation string) Configuration {
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		log.Fatal("Error reading status structure file. Error: " + err.Error())
	}

	var config Configuration
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal("Error parsing json from config file. Error: " + err.Error())
	}
	return config
}

func StartLogger() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter(`%{color}%{shortfunc} ▶ %{level:.4s} %{color:reset} %{message}`)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
}