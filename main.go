package load

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"

	internal "github.com/evanofslack/nats-load-traffic/internal"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func Run(configPath, payloadPath string) {
	// load config
	config, err := internal.NewConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	internal.InitLogger(config.Log.Level, config.Log.Env)

	// check if remote write is enabled
	var rwUrl string
	rw := config.RemoteWrite
	if rw.Enabled && rw.Url != nil {
		slog.Info("remote write enabled", slog.String("url", *rw.Url))
		rwUrl = *rw.Url
	}

	// start metrics
	ctx := context.Background()
	metrics := internal.NewMetrics(rwUrl)
	defer metrics.Shutdown(ctx)
	if rw.Enabled {
		interval := internal.DefaultWriteInterval
		if rw.Interval != nil {
			slog.Debug("setting remote write interval", slog.String("interval", rw.Interval.String()))
			interval = *rw.Interval
		}
		go metrics.RemoteWriteThread(ctx, interval)
	}

	// load json payload
	json, err := os.Open(payloadPath)
	if err != nil {
		log.Fatal(err)
	}
	defer json.Close()
	payload, err := io.ReadAll(json)
	if err != nil {
		log.Fatal(err)
	}

	// connect to nats
	url := config.Nats.Url
	opts := []nats.Option{nats.Name("Wasabi NATS Load Test")}

	slog.Debug("connecting to nats", slog.String("url", url))
	nc, err := nats.Connect(url, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)

	// shared result chan for all jobs
	resultChan := make(chan internal.Task)

	var wg = sync.WaitGroup{}
	slog.Info(fmt.Sprintf("starting %d profiles", len(config.Profiles)))
	for _, job := range config.Profiles {
		load, err := internal.LoadFromString(job.Load)
		if err != nil {
			fmt.Println(err)
			load = internal.DefaultLoad
		}
		producer := internal.NewProducer(js, job.Name, job.Subject, payload, load, job.Duration, job.Rate)

		// each job gets own task chan
		taskChan := make(chan internal.Task)

		go producer.Run(ctx, taskChan)
		for i := 0; i < 100; i++ {
			worker := internal.NewWorker(js, job.Subject)

			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.Run(ctx, taskChan, resultChan)
			}()
		}
	}

	// wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for task := range resultChan {
		// count total tasks and task errors
		success := task.Err == nil
		metrics.IncSubmit(success, task.Subject)
		if task.Err != nil {
		}
	}

	nc.Flush()
	if err := nc.LastError(); err != nil {
		log.Fatal(err)
	}

	var totalOk uint64
	var totalErr uint64
	for _, sub := range metrics.Report() {
		slog.Info("submission count for subject", slog.String("subject", sub.Subject), slog.Uint64("success", sub.Success), slog.Uint64("failure", sub.Failure))
		totalOk += sub.Success
		totalErr += sub.Failure
	}
	slog.Info("total submission count for all subjects", slog.Uint64("success", totalOk), slog.Uint64("failure", totalErr))
}
