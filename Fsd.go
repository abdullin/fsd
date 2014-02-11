package fsd

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

var (
	Instance *Fsd
)

type Fsd struct {
	outgoing chan string
	address  string
	conn     net.Conn
}

func init() {
	Start("127.0.0.1:8125")
}

func Start(address string) {
	Instance = &Fsd{address: address, outgoing: make(chan string, 100000)}
	Instance.connect()

	go Instance.processOutgoing()
}

func (fsd *Fsd) connect() error {
	conn, err := net.Dial("udp", fsd.address)
	if err != nil {
		return err
	}

	fsd.conn = conn
	return nil
}

func (fsd *Fsd) processOutgoing() {
	for outgoing := range fsd.outgoing {
		if _, err := fsd.conn.Write([]byte(outgoing)); err != nil {
			fsd.connect()
		}
	}
}

// To read about the different semantics check out
// https://github.com/b/statsd_spec
// http://docs.datadoghq.com/guides/dogstatsd/

// Increment the page.views counter.
// page.views:1|c
func Count(name string, value float64) {
	CountL(name, value, 1.0)
}

func CountL(name string, value float64, rate float64) {
	payload, err := rateCheck(rate, createPayload(name, value, "c"))
	if err != nil {
		return
	}

	send(payload)
}

// Record the fuel tank is half-empty
// fuel.level:0.5|g
func Gauge(name string, value float64) {
	payload := createPayload(name, value, "g")
	send(payload)
}

// A request latency
// request.latency:320|ms
// Or a payload of a image
// image.size:2.3|ms
func Timer(name string, duration time.Duration) {
	TimerL(name, duration, 1.0)
}

func TimerL(name string, duration time.Duration, rate float64) {
	HistogramL(name, float64(duration.Nanoseconds()/1000000), rate)
}

func Histogram(name string, value float64) {
	HistogramL(name, value, 1.0)
}

func HistogramL(name string, value float64, rate float64) {
	payload, err := rateCheck(rate, createPayload(name, value, "ms"))
	if err != nil {
		return
	}

	send(payload)
}

// TimeSince records a named timer with the duration since start
func TimeSince(name string, start time.Time) {
	TimeSinceL(name, start, 1.0)
}

// TimeSince records a rated and named timer with the duration since start
func TimeSinceL(name string, start time.Time, rate float64) {
	TimerL(name, time.Now().Sub(start), rate)
}

func Time(name string, lambda func()) {
	TimeL(name, 1.0, lambda)
}

func TimeL(name string, rate float64, lambda func()) {
	start := time.Now()
	lambda()
	TimeSinceL(name, start, rate)
}

// Track a unique visitor id to the site.
// users.uniques:1234|s
func Set(name string, value float64) {
	payload := createPayload(name, value, "s")
	send(payload)
}

func createPayload(name string, value float64, suffix string) string {
	return fmt.Sprintf("%s:%f|%s", name, value, suffix)
}

func rateCheck(rate float64, payload string) (string, error) {
	if rate < 1 {
		if rand.Float64() < rate {
			return payload + fmt.Sprintf("|@%f", rate), nil
		}
	} else { // rate is 1.0 == all samples should be sent
		return payload, nil
	}

	return "", errors.New("Out of rate limit")
}

func send(payload string) {
	length := float64(len(Instance.outgoing))
	capacity := float64(cap(Instance.outgoing))

	if length < capacity*0.9 {
		Instance.outgoing <- payload
	}
}
