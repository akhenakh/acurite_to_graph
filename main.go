package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpPort = flag.Int("httpPort", 44010, "http port to listen on")
	protocol = flag.String("protocol", "39", "Protocol to enable")
	cmdPath  = flag.String("cmdPath", "rtl_433", "full path for rtl_433")
	debug    = flag.Bool("debug", false, "set debug")

	influxUsername = flag.String("influxUsername", "", "influxDB Username")
	influxPassword = flag.String("influxPassword", "", "influxDB Password")
	influxURL      = flag.String("influxURL", "", "influxDB URL, disabled if empty")
	influxDatabase = flag.String("influxDatabase", "", "influx Database name")

	temperature = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "temperature_celsius",
		Help: "Current temperature in Celsius",
	},
		[]string{"model", "channel", "id"},
	)

	humidity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "humidity",
		Help: "Current Humidity",
	},
		[]string{"model", "channel", "id"},
	)
)

func init() {
	prometheus.MustRegister(temperature)
	prometheus.MustRegister(humidity)
}

type DeviceMessage struct {
	Model       string
	Id          int
	Channel     string
	TempCelsius float64 `json:"temperature_C"`
	Humidity    float64
	Battery     string
}

func (msg *DeviceMessage) ToLabels() map[string]string {
	m := make(map[string]string)
	m["model"] = msg.Model
	m["channel"] = msg.Channel
	m["id"] = strconv.Itoa(msg.Id)
	return m
}

func (msg *DeviceMessage) ToInfluxPoint() *client.Point {
	fields := map[string]interface{}{
		"temperature": msg.TempCelsius,
		"humidity":    msg.Humidity,
	}
	pt, err := client.NewPoint("sensor", msg.ToLabels(), fields, time.Now())
	if err != nil {
		log.Println(err)
		return nil
	}

	return pt
}

func main() {
	flag.Parse()

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

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		err = http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil)
		log.Println(err)
	}()

	for in.Scan() {
		if err := json.Unmarshal([]byte(in.Text()), &msg); err != nil {
			log.Println(err)
			continue
		}
		// Set values on prometheus gauges
		temperature.With(prometheus.Labels(msg.ToLabels())).Set(msg.TempCelsius)
		humidity.With(prometheus.Labels(msg.ToLabels())).Set(msg.Humidity)

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

	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil)
	log.Println(err)
}
