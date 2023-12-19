package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/castai/promwrite"
)

const (
	namespace = "nats_load_traffic"
)

type counter struct {
	value uint64
}

func newCounter() *counter {
	return &counter{
		value: 0,
	}
}

func (c *counter) get() uint64 {
	return atomic.LoadUint64(&c.value)
}

func (c *counter) inc(count int) {
	// no-op
	if count <= 0 {
		return
	}
	atomic.AddUint64(&c.value, uint64(count))
}

type subjectMetrics struct {
	subject            string
	submissionsSuccess *counter
	submissionsFailure *counter
}

func newSubjectMetrics(subject string) *subjectMetrics {
	sm := &subjectMetrics{
		subject:            subject,
		submissionsSuccess: newCounter(),
		submissionsFailure: newCounter(),
	}
	return sm
}

type remoteWriter struct {
	enabled bool
	client  *promwrite.Client
}

type metrics struct {
	lock     sync.RWMutex
	subjects map[string]*subjectMetrics
	writer   remoteWriter
}

func newMetrics(remoteWriteUrl string) *metrics {
	var writer remoteWriter
	if remoteWriteUrl == "" {
		writer.enabled = false
	} else {
		writer.enabled = true
		writer.client = promwrite.NewClient(remoteWriteUrl)
	}

	m := &metrics{
		lock:     sync.RWMutex{},
		subjects: make(map[string]*subjectMetrics),
		writer:   writer,
	}
	return m
}

func (m *metrics) getSubjectMetrics(subject string) *subjectMetrics {
	m.lock.RLock()
	sm, exists := m.subjects[subject]
	m.lock.RUnlock()
	if exists {
		return sm
	}

	sm = newSubjectMetrics(subject)
	m.lock.Lock()
	m.subjects[subject] = sm
	m.lock.Unlock()

	return sm
}

func (m *metrics) incSubmit(success bool, subject string) {
	sm := m.getSubjectMetrics(subject)
	if success {
		sm.submissionsSuccess.inc(1)
	} else {
		sm.submissionsFailure.inc(1)
	}
}

func (m *metrics) toTimeSeries() []promwrite.TimeSeries {
	m.lock.RLock()
	defer m.lock.RUnlock()

	series := []promwrite.TimeSeries{}
	for _, sm := range m.subjects {
		success := sm.submissionsSuccess.get()
		series = append(series, promwrite.TimeSeries{
			Labels: []promwrite.Label{
				{
					Name:  "__name__",
					Value: fmt.Sprintf("%s_submissions_total", namespace),
				},
				{
					Name:  "subject",
					Value: sm.subject,
				},
				{
					Name:  "result",
					Value: "success",
				},
			},
			Sample: promwrite.Sample{
				Time:  time.Now(),
				Value: float64(success),
			},
		})

		failure := sm.submissionsFailure.get()
		series = append(series, promwrite.TimeSeries{
			Labels: []promwrite.Label{
				{
					Name:  "__name__",
					Value: fmt.Sprintf("%s_submissions_total", namespace),
				},
				{
					Name:  "subject",
					Value: sm.subject,
				},
				{
					Name:  "result",
					Value: "failure",
				},
			},
			Sample: promwrite.Sample{
				Time:  time.Now(),
				Value: float64(failure),
			},
		})
	}
	return series
}

func (m *metrics) remoteWrite(ctx context.Context) error {
	fmt.Println("sending remote write request")
	times := m.toTimeSeries()
	req := &promwrite.WriteRequest{
		TimeSeries: times,
	}
	_, err := m.writer.client.Write(ctx, req)
	return err
}

func (m *metrics) remoteWriteThread(ctx context.Context, interval time.Duration) {
	fmt.Println("starting remote write thread")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// try to do a final write when exiting
	defer m.remoteWrite(context.Background())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.remoteWrite(ctx)
		}
	}
}

func (m *metrics) shutdown(ctx context.Context) {
	if m.writer.enabled {
		m.remoteWrite(ctx)
	}
}
