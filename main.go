package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func usage() {
	log.Printf("Usage: loadtest [-s server] [-c config] [-p payload]\n")
	flag.PrintDefaults()
}

func showUsageAndExit(exitcode int) {
	usage()
	os.Exit(exitcode)
}

func main() {
	var urls = flag.String("u", nats.DefaultURL, "The nats server URLs (separated by comma)")
	var configPath = flag.String("c", "config.yaml", "Path to config file")
	var payloadPath = flag.String("m", "msg.json", "Path to message payload file")

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 0 {
		showUsageAndExit(1)
	}

	// load config
	config, err := newConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	// check if remote write is enabled
	var rwUrl string
	rw := config.RemoteWrite
	fmt.Println(rw.enabled)
	fmt.Println(rw.url)
	if rw.enabled && rw.url != nil {
		rwUrl = *rw.url
	}

	// start metrics
	ctx := context.Background()
	metrics := newMetrics(rwUrl)
	defer metrics.shutdown(ctx)
	if rw.enabled {
		go metrics.remoteWriteThread(ctx, time.Second*10)
	}

	// load json payload
	json, err := os.Open(*payloadPath)
	if err != nil {
		log.Fatal(err)
	}
	defer json.Close()
	payload, err := io.ReadAll(json)
	if err != nil {
		log.Fatal(err)
	}

	// connect to nats
	fmt.Println(config.Nats.url)
	opts := []nats.Option{nats.Name("Wasabi NATS Load Test")}

	nc, err := nats.Connect(*urls, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)

	// shared result chan for all jobs
	resultChan := make(chan task)

	var wg = sync.WaitGroup{}
	for _, job := range config.Profiles {
		load, err := loadFromString(job.Load)
		if err != nil {
			fmt.Println(err)
			load = defaultLoad
		}
		producer := newProducer(js, job.Name, job.Subject, payload, load, job.Duration, job.Rate)

		// each job gets own task chan
		taskChan := make(chan task)

		go producer.run(ctx, taskChan)
		for i := 0; i < 100; i++ {
			worker := newWorker(js, job.Subject)

			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.run(ctx, taskChan, resultChan)
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
		success := task.err == nil
		metrics.incSubmit(success, task.subject)
		if task.err != nil {
		}
	}

	nc.Flush()
	if err := nc.LastError(); err != nil {
		log.Fatal(err)
	}

	var totalOk uint64
	var totalErr uint64
	for _, sub := range metrics.subjects {
		ok, e := sub.submissionsSuccess.get(), sub.submissionsFailure.get()
		fmt.Println("-- report --")
		fmt.Printf("%s | ok %d | err %d |\n", sub.subject, ok, e)
		totalOk += ok
		totalErr += e
	}
	fmt.Printf("TOTAL | ok %d | err %d |\n", totalOk, totalErr)
}
