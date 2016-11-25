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
	"strconv"
	"strings"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpPort = flag.Int("httpPort", 44010, "http port to listen on")
	protocol = flag.String("protocol", "39", "Protocol to enable")
	cmdPath  = flag.String("cmdPath", "rtl_433", "full path for rtl_433")
	debug    = flag.Bool("debug", false, "set debug")

	namedOnly = flag.Bool("namedOnly", false, "Only insert named sensors. See named nameFields")

	influxUsername = flag.String("influxUsername", "", "influxDB Username")
	influxPassword = flag.String("influxPassword", "", "influxDB Password")
	influxURL      = flag.String("influxURL", "", "influxDB URL, disabled if empty")
	influxDatabase = flag.String("influxDatabase", "", "influx Database name")

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
	<title>Bootstrap 101 Template</title>

	<!-- Bootstrap -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u"
		crossorigin="anonymous">
	<!-- Optional theme -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap-theme.min.css" integrity="sha384-rHyoN1iRsVXV4nD0JutlnGaslCJuC7uwjduW9SVrLvRYooPp2bWYgmgJQIXwl/Sp"
		crossorigin="anonymous">

</head>

<body>

<div class="list-group">

  <a href="#" class="list-group-item">
    <h1 class="list-group-item-heading">18°C 56%</h1>
    <p class="list-group-item-text"> <h3>Livingroom</h3></p>
  </a>

  <a href="#" class="list-group-item">
    <h1 class="list-group-item-heading">19°C 66%</h1>
    <p class="list-group-item-text"> <h3>Kitchen</h3></p>
  </a>

  <a href="#" class="list-group-item">
    <h1 class="list-group-item-heading">4°C 63%</h1>
    <p class="list-group-item-text"> <h3>Outside</h3></p>
  </a>

</div>


</body>

</html>`
)

func init() {
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(humidity)
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	t := template.New("index.html")
	t, err := t.Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	var sensors []map[string]interface{}

	m := temperature.MetricVec.WithLabelValues(labels...)
	fmt.Println(m.Desc())
	t.Execute(w, sensors)
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

	var influxClient client.Client
	if *influxURL != "" {
		var err error
		influxClient, err = client.NewHTTPClient(client.HTTPConfig{
			Addr:     *influxURL,
			Username: *influxUsername,
			Password: *influxPassword,
		})

		if err != nil {
			log.Fatal(err)
		}
	}

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

	var msg DeviceMessage

	bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  *influxDatabase,
		Precision: "s",
	})

	for in.Scan() {
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

		if influxClient != nil {
			bp.AddPoint(msg.ToInfluxPoint())
			if err := influxClient.Write(bp); err != nil {
				log.Println(err)
			}
		}
		if *debug {
			log.Println(msg)
		}
	}
	if err := in.Err(); err != nil {
		log.Printf("error: %s", err)
	}

}
