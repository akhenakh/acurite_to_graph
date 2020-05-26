package main

import (
	"strconv"
	"time"
)

var (
	labels = []string{"model", "channel", "id", "name"}
)

type DeviceMessage struct {
	Model        string
	ID           int
	Channel      string
	TempCelsius  float64 `json:"temperature_C"`
	Humidity     float64
	LowBattery   int
	Name         string
	ReceivedTime time.Time
}

func (msg *DeviceMessage) ToLabels() map[string]string {
	m := make(map[string]string)
	m["model"] = msg.Model
	m["channel"] = msg.Channel
	m["id"] = strconv.Itoa(msg.ID)
	m["name"] = msg.Name
	return m
}
