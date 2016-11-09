package main

import (
	"log"
	"strconv"
	"time"

	client "github.com/influxdata/influxdb/client/v2"
)

var (
	labels = []string{"model", "channel", "id", "name"}
)

type DeviceMessage struct {
	Model       string
	ID          int
	Channel     string
	TempCelsius float64 `json:"temperature_C"`
	Humidity    float64
	LowBattery  int
	Name        string
}

func (msg *DeviceMessage) ToLabels() map[string]string {
	m := make(map[string]string)
	m["model"] = msg.Model
	m["channel"] = msg.Channel
	m["id"] = strconv.Itoa(msg.ID)
	m["name"] = msg.Name
	return m
}

func (msg *DeviceMessage) ToInfluxPoint() *client.Point {
	fields := map[string]interface{}{
		"temperature": msg.TempCelsius,
		"humidity":    msg.Humidity,
		"low_battery": false,
	}
	if msg.LowBattery == 1 {
		fields["low_battery"] = true
	}
	pt, err := client.NewPoint("sensor", msg.ToLabels(), fields, time.Now())
	if err != nil {
		log.Println(err)
		return nil
	}

	return pt
}
