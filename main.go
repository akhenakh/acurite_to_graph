package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpPort = flag.Int("httpPort", 44010, "http port to listen on")
	protocol = flag.String("protocol", "39", "Protocol to enable")
	cmdPath  = flag.String("cmdPath", "rtl_433", "full path for rtl_433")
	debug    = flag.Bool("debug", false, "set debug")

	namedOnly = flag.Bool("namedOnly", false, "Only insert named sensors. See named nameFields")

	indexTpl = template.New("index")

	temperature = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensoracurite_temperature_celsius",
		Help: "Current temperature in Celsius",
	},
		labels,
	)

	lowBattery = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensoracurite_low_battery",
		Help: "Battery is low",
	},
		labels,
	)

	humidity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensoracurite_humidity",
		Help: "Current Humidity",
	},
		labels,
	)

	indexHTML = `<!DOCTYPE html>
<html lang="en">

<head>
	<meta charset="utf-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<!-- The above 3 meta tags *must* come first in the head; any other head content must come *after* these tags -->
	<title>Temperature sensors</title>

	<!-- Bootstrap -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u"
		crossorigin="anonymous">
	<!-- Optional theme -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap-theme.min.css" integrity="sha384-rHyoN1iRsVXV4nD0JutlnGaslCJuC7uwjduW9SVrLvRYooPp2bWYgmgJQIXwl/Sp"
		crossorigin="anonymous">

</head>

<body>

<div class="list-group">
  {{ range .Metrics }}
  <a href="#" class="list-group-item">
    <h1 class="list-group-item-heading">{{ .Temperature }}°C {{ .Humidity }}%</h1>
    <p class="list-group-item-text"><h3>{{ .Name }} {{ .Channel }} </h3></p>
  </a>
  {{ end }}
</div>


</body>

</html>`
)

func init() {
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(humidity)
}

type TplMetric struct {
	Name        string
	Temperature float64
	Channel     string
	Humidity    float64
}

type TplMetrics struct {
	Metrics []*TplMetric
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	mm := make(map[string]*TplMetric)

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		http.Error(w, "No metrics gathered, last error:\n\n"+err.Error(), http.StatusInternalServerError)
		return
	}
	for _, mf := range mfs {
		if mf.GetName() == "sensoracurite_temperature_celsius" {
			for _, m := range mf.GetMetric() {
				lbls := m.GetLabel()
				var name string
				var id string
				var channel string
				for _, lbl := range lbls {
					if *lbl.Name == "name" {
						name = *lbl.Value
					}
					if *lbl.Name == "id" {
						id = *lbl.Value
					}
					if *lbl.Name == "channel" {
						channel = *lbl.Value
					}
				}
				if name == "" {
					name = id
				}
				if _, ok := mm["name"]; !ok {
					mm[name] = &TplMetric{}
				}
				mm[name].Name = name
				mm[name].Temperature = m.Gauge.GetValue()
				mm[name].Channel = channel
			}
		}
		if mf.GetName() == "sensoracurite_humidity" {
			for _, m := range mf.GetMetric() {
				lbls := m.GetLabel()
				var name string
				var id string
				for _, lbl := range lbls {
					if *lbl.Name == "name" {
						name = *lbl.Value
					}
					if *lbl.Name == "id" {
						id = *lbl.Value
					}
				}
				if name == "" {
					name = id
				}

				if _, ok := mm["name"]; !ok {
					mm[name] = &TplMetric{}
				}
				mm[name].Humidity = m.Gauge.GetValue()
				if *debug {
					log.Println("HUMI set", name, m.Gauge.GetValue())
				}
			}
		}
	}

	t := template.New("index.html")
	t, err = t.Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	sensors := make([]*TplMetric, len(mm))
	i := 0
	for _, v := range mm {
		sensors[i] = v
		i++
	}

	// sort by location name
	sort.SliceStable(sensors, func(i, j int) bool { return sensors[i].Name < sensors[j].Name })

	t.Execute(w, &TplMetrics{Metrics: sensors})
}

func main() {
	// name device
	var nameFieldsFlag fieldFlag
	nameFields := make(map[int64]string)
	flag.Var(&nameFieldsFlag, "nameFields", "List of id=name pairs (comma separated) to  be injected as a name label eg 1251=kitchen")

	flag.Parse()

	// parsing args for naming devices
	for _, field := range nameFieldsFlag.Fields {
		if len(strings.Split(field, "=")) != 2 {
			fmt.Println("Invalid forceField", field)
			flag.PrintDefaults()
			os.Exit(2)
		}
		split := strings.Split(field, "=")
		deviceID, err := strconv.ParseInt(split[0], 10, 64)
		if err != nil {
			fmt.Println("Invalid forceField shoud be an int", field)
			flag.PrintDefaults()
			os.Exit(2)
		}
		nameFields[deviceID] = split[1]
	}

	if *namedOnly && len(nameFields) == 0 {
		fmt.Println("namedOnly is filtering all the sensors")
		flag.PrintDefaults()
		os.Exit(2)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/", pageHandler)
		err := http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil)
		log.Println(err)
	}()

	cmd := exec.Command(*cmdPath, "-R", *protocol, "-F", "json", "-q")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	// read command's stdout line by line
	in := bufio.NewScanner(stdout)

	for in.Scan() {
		var msg DeviceMessage
		if err := json.Unmarshal([]byte(in.Text()), &msg); err != nil {
			log.Println(err)
			continue
		}
		// add names labels
		if name, ok := nameFields[int64(msg.ID)]; ok {
			msg.Name = name
		} else {
			if *namedOnly {
				if *debug {
					log.Println("Skipped sensors because of namedOnly", msg.ID)
				}
				continue
			}
		}
		// Set values on prometheus gauges
		temperature.With(prometheus.Labels(msg.ToLabels())).Set(msg.TempCelsius)
		humidity.With(prometheus.Labels(msg.ToLabels())).Set(msg.Humidity)
		lowBattery.With(prometheus.Labels(msg.ToLabels())).Set(float64(msg.LowBattery))

		if *debug {
			log.Println(msg)
		}
	}
	if err := in.Err(); err != nil {
		log.Printf("error: %s", err)
	}

}
