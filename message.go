package main

import (
	"log"
	"strconv"
	"time"

	client "github.com/influxdata/influxdb/client/v2"
)

type DeviceMessage struct {
	Model       string
	ID          int
	Channel     string
	TempCelsius float64 `json:"temperature_C"`
	Humidity    float64
	Battery     string
}

func (msg *DeviceMessage) ToLabels() map[string]string {
	m := make(map[string]string)
	m["model"] = msg.Model
	m["channel"] = msg.Channel
	m["id"] = strconv.Itoa(msg.ID)
	return m
}

func (msg *DeviceMessage) ToInfluxPoint() *client.Point {
	fields := map[string]interface{}{
		"temperature": msg.TempCelsius,
		"humidity":    msg.Humidity,
		"low_battery": false,
	}
	if msg.Battery == "LOW" {
		fields["low"] = true
	}
	pt, err := client.NewPoint("sensor", msg.ToLabels(), fields, time.Now())
	if err != nil {
		log.Println(err)
		return nil
	}

	return pt
}
