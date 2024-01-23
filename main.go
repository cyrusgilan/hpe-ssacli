package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	m := NewMetrics()
	go m.CollectLoop()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9101", nil))
}

type metrics struct {
	numControllers prometheus.Gauge
	ldStatus       *prometheus.GaugeVec
	ldCapacity     *prometheus.GaugeVec
	ldRaid         *prometheus.GaugeVec
}

func NewMetrics() *metrics {
	m := &metrics{
		numControllers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "num_controllers",
			Help: "Number of smart storage array controllers.",
		}),
		ldStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "logical_device_status",
			Help: "Status of logical device. 0 = OK",
		}, []string{"controller_slot", "logical_device_number"}),
		ldCapacity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "logical_device_capacity",
			Help: "Capacity of logical device in MB",
		}, []string{"controller_slot", "logical_device_number"}),
		ldRaid: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "logical_device_raid",
			Help: "RAID level of logical device",
		}, []string{"controller_slot", "logical_device_number"}),
	}

	prometheus.MustRegister(m.numControllers)
	prometheus.MustRegister(m.ldStatus)
	prometheus.MustRegister(m.ldCapacity)
	prometheus.MustRegister(m.ldRaid)

	return m
}

func (m *metrics) CollectLoop() {
	for {
		m.Collect()
		time.Sleep(5 * time.Second)
	}
}
func (m *metrics) Collect() {
	controller_idxs := m.CollectControllerCount()
	for _, idx := range controller_idxs {
		m.CollectLogicalDrives(idx)
	}
}

var controllerSlotRegex = regexp.MustCompile(`Slot (\d+)`)

func (m *metrics) CollectControllerCount() []int {
	out, err := exec.Command("ssacli", "ctrl", "all", "show").Output()
	if err != nil {
		log.Printf("error listing controllers: %v", err)
		return nil
	}

	slots := []int{}
	matches := controllerSlotRegex.FindAllSubmatch([]byte(out), -1)
	for _, match := range matches {
		slot, err := strconv.Atoi(string(match[1]))
		if err != nil {
			continue
		}

		slots = append(slots, slot)
	}

	m.numControllers.Set(float64(len(matches)))

	return slots
}

var logicalDriveRegex = regexp.MustCompile(`logicaldrive (\d+) \((.*B), RAID (.*), (.*)\)`)

func (m *metrics) CollectLogicalDrives(slot int) {
	out, err := exec.Command("ssacli", "ctrl", fmt.Sprintf("slot=%d", slot), "ld", "all", "show").Output()
	if err != nil {
		log.Printf("error getting controller logical devices: %v", err)
		return
	}

	matches := logicalDriveRegex.FindAllSubmatch([]byte(out), -1)
	if matches == nil {
		return
	}

	for _, match := range matches {
		ld_num, err := strconv.Atoi(string(match[1]))
		if err != nil {
			continue
		}
		ld_cap := ParseCapacity(string(match[2]))
		ld_raid, err := strconv.Atoi(string(match[3]))
		if err != nil {
			ld_raid = -1
		}
		ld_status := ParseStatus(string(match[4]))

		labels := prometheus.Labels{"controller_slot": strconv.Itoa(slot), "logical_device_number": strconv.Itoa(ld_num)}
		m.ldStatus.With(labels).Set(float64(ld_status))
		m.ldCapacity.With(labels).Set(float64(ld_cap))
		m.ldRaid.With(labels).Set(float64(ld_raid))
	}
}

func ParseCapacity(s string) int {
	num_str, suf, found := strings.Cut(s, " ")
	num, err := strconv.ParseFloat(num_str, 64)
	if !found || err != nil {
		return -1
	}

	switch suf[0] {
	case 'T':
		return int(num * 1024.0 * 1024.0)
	case 'G':
		return int(num * 1024.0)
	case 'M':
		return int(num)
	default:
		return int(num)
	}
}

func ParseStatus(s string) int {
	switch s {
	case "OK":
		return 0
	default:
		return 1
	}

}
