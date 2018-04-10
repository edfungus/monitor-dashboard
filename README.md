# Monitor Dashboard 📺

## Introduction 

![Monitor Dashboard](https://github.com/edfungus/monitor-dashboard/raw/master/demo.png "Example Monitor Dashboard Picture")

Monitor Dashboard is a live dashboard that shows an aggregated view of your services' health from sources like New Relic. Configured with a simple JSON file, deployable to CF and has features like basic authentication security, Monitor Dashboard is easy and ready to use!

_Suggested use_: Put in on a big TV screen!

## How to get running 🏃

### Prerequisites

* Golang `https://golang.org/dl/`
* NPM `https://www.npmjs.com/get-npm`
* Glide `brew install glide`
* Bower `npm install -g bower`

### Running locally 

1. Clone the repository
2. `glide install -v`
3. `(cd public && bower install)`
4. ` go build`
5. Run Service Monitor! 
```
CONFIG_FILE=config_example.json \
NEW_RELIC_12345_KEY=somekey \
PASSWORD=thisiscool \
./monitor-dashboard
```
6. Visit `localhost:3000` and login with `admin` and `thisiscool`

What you should see after the login page is the words _Production_ and _Dev_ along with two gray boxes below them. The boxes are gray because the monitor does not know the statuses of the services from New Relic. Also because the New Relic account and key are just demo, the gray will not change. So how do you get your own New Relic monitors hooked up? Next section!

## How to customize 🎉

Customizing Service Monitor is fairly easy and is done through a JSON file. An example JSON file is `config_example.json` Steps are as follows when creating a `config.json`:

1. Configure a probe which defines where status will come from (only New Relic at this point)
2. Create a heirarchy of statuses to be displayed on the dashboard
3. Hook up the probe to the status by referencing your defined probe in the status 

### Config.json

The basic structure of `config.json` is the following:
```json
{
  "dashboardName": "Example Dashboard",
  "probes": [{
    "id": "probe-id", 
    "type": "NewRelic",
    "data": { 
      "accountNumber": "12345",
      "interval": "1m",
      "apiKeyEnvVar": "NEW_RELIC_12345_KEY"
    }
  }], 
  "statuses": [{
    "id": "env",
    "fullName": "Production",
    "children": [{
      "id": "service",
      "fullName": "Service Name",
      "abbrevName": "SN",
      "children": [],
      "url": "https://serivce.com",
      "probe": {
        "probeRefId": "probe-id",
        "data": {
          "monitorName": "new-relic-service-monitor-name"
        }
      }
    }]
  }]
}
```

### Dashboard Name
`dashboardName` defines the title that is shown at the top of the monitor

### Probes 
`probes` define sources to get updates for status.

```yaml
# Probe fields
id: ID referenced in statuses
type: Type of probe created
data: Additional information map required by probe to function
```
Currently the only probe type support is `NewRelic`.

| Probe | Description | Type | Probe definition `data` fields | Probe reference `data` fields |
|---|---|---|---|---|
| New Relic | Polls New Relic for new synthetic statuses | `NewRelic` | <ul><li>`accountNumber`: New Relic user ID</li><li>`interval`: time interval to poll New Relic. Suggested `1m`</li><li>`apiKeyEnvVar`: Environment variable where New Relic API key will be stored</li></ul>| <ul><li>`monitorName`: The synthetic monitor's name associated with status</li></ul>|


### Statuses
`statuses` define the structure of which the status are shown on the dashboard. A `status` can have children of `statuses` which gives the hierarchy. If a `status` is a parent, it will represent the main categories shown on the dashboard. Usually this is based on deployment environments. The children of a `status` are the colored boxes representing the status of a service which is update by a `probe`. At this time, none of the children beyond depth 2 are shown and they don't do anything. 

```yaml
# Status fields
id: ID referenced when updating status
fullName: Name used in title (if parent)
children: Array of children which will be shown as boxes (if parent)
abbrevName: The main letters shown to identify the box (if child)
subText: The sub-text shown to differentiate visually if multiple box have same `abbrevName` (if child)
url: URL to open if box clicked on (if child)
probe: The probe which will update the status (if child)
  probeRefId: Identifies which probe to use
  data: Additional information map required by probe to function
```

## How to deploy to CF 🚀

Let's make your monitor avialible for others to see! 

1. Run steps 1-3 from the `Running Locally` steps above
2. Add `PASSWORD` env to `manifest_example.yml`
3. `cf push -f manifest_example.yml`
4. Visit the route given at the end of the push and you should see the same dashboard you run locally!

Maybe in the future I'll have an example Jenkinsfile too ... 

## Extra APIs

Generally the service monitor is only used for display, but there are a couple simple APIs that could be useful. _Note_: The API is basic auth protected with the same credentials used to login

| Method | Route | Description |
|---|---|---|
| `GET` | `/status` | Gives the current status of all the monitors in a format similar to the `config.json` |
| `GET` | `/update/{id}/{status}` | This is the only push method of updating a status. `{id}` is the concatenation of the id's with `-` as the delimiter from parent to target child. `{status}` can be `good`, `bad`, `degraded`, or `unknown` |

## Contributing

If you would like to contribute, simply make a PR! Always looking for new contributers and ideas!
