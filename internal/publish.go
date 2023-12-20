package internal

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/ratelimit"
)

type Task struct {
	seq      int
	Subject  string
	payload  []byte
	duration time.Duration
	Err      error
}

func newTask(seq int, subject string, payload []byte) Task {
	t := Task{
		seq:     seq,
		Subject: subject,
		payload: payload,
	}
	return t
}

type worker struct {
	js      jetstream.JetStream
	subject string
}

func NewWorker(js jetstream.JetStream, subject string) worker {
	w := worker{
		js:      js,
		subject: subject,
	}
	return w
}

func (w *worker) Run(ctx context.Context, taskChan <-chan Task, resultChan chan<- Task) {
	for task := range taskChan {
		start := time.Now()

		w.publishSync(ctx, task)

		task.duration = time.Duration(time.Since(start))
		resultChan <- task
	}
}

func (w *worker) publishSync(ctx context.Context, task Task) {
	ack, err := w.js.Publish(ctx, task.Subject, task.payload)
	if err != nil {
		task.Err = err
		fmt.Println(err)
	} else {
		fmt.Printf("published msg | seq [%d|%d] | subject %s\n", task.seq, ack.Sequence, task.Subject)
	}
}

func publishAsync(js jetstream.JetStream, task Task) {
	ackF, err := js.PublishAsync(task.Subject, task.payload)
	if err != nil {
		fmt.Println(err)
	}

	go func() {
		select {
		case ack := <-ackF.Ok():
			task.Err = err
			fmt.Printf("published msg | seq [%d|%d] | subject %s\n", task.seq, ack.Sequence, task.Subject)
		case err := <-ackF.Err():
			fmt.Println(err)
		}
	}()
}

type producer struct {
	rl       ratelimit.Limiter
	js       jetstream.JetStream
	name     string
	subject  string
	payload  []byte
	load     load
	duration time.Duration
}

func NewProducer(js jetstream.JetStream, name, subject string, payload []byte, load load, duration time.Duration, rate int) producer {
	p := producer{
		rl:       ratelimit.New(rate, ratelimit.Per(60*time.Second)),
		js:       js,
		name:     name,
		subject:  subject,
		payload:  payload,
		load:     load,
		duration: duration,
	}
	return p
}

func (p *producer) Run(ctx context.Context, taskChan chan Task) {
	defer close(taskChan)
	timeout := time.After(p.duration)
	seq := 0
	fmt.Printf("profile %s | load %s | duration %s\n", p.name, p.load, p.duration)

	// setup some state for periodic load profile
	periodicSend := true
	if p.load == Periodic {
		go func() {
			interval := p.duration / 60
			if interval < time.Second*1 {
				interval = time.Second * 1
			}
			if interval > time.Minute*1 {
				interval = time.Minute * 1
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			fmt.Printf("profile %s setting periodic interval %s\n", p.name, interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					periodicSend = !periodicSend
				}
			}
		}()
	}

	// setup some state for random load profile
	randomSend := true
	if p.load == Random {
		rand.NewSource(time.Now().UnixNano())
		go func() {
			interval := p.duration / 60
			if interval < time.Second*1 {
				interval = time.Second * 1
			}
			if interval > time.Second*5 {
				interval = time.Second * 5
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			fmt.Printf("profile %s setting random interval %s\n", p.name, interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// 50% change to flip
					if rand.Intn(2) == 1 {
						randomSend = !randomSend
						sleep := rand.Intn(4)
						time.Sleep(interval * time.Duration(sleep))
					}
				}
			}
		}()
	}

	// setup some state for rare load profile
	rareSend := false
	if p.load == Rare {
		rand.NewSource(time.Now().UnixNano())
		go func() {
			interval := p.duration / 60
			if interval < time.Second*1 {
				interval = time.Second * 1
			}
			if interval > time.Minute*5 {
				interval = time.Minute * 5
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			fmt.Printf("profile %s setting rare interval %s\n", p.name, interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					fmt.Printf("checking rare\n")
					if rand.Intn(15) == 1 {
						rareSend = true
					} else {
						rareSend = false
					}
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			fmt.Printf("profile %s finished after %s\n", p.name, p.duration)
			return
		default:
			switch p.load {
			// constant: always send
			case Constant:
				p.send(taskChan, seq)
			// periodic: alternate
			case Periodic:
				if periodicSend {
					p.send(taskChan, seq)
				}
			// random: flip randomly
			case Random:
				if randomSend {
					p.send(taskChan, seq)
				}
			// rare: send very infrequently
			case Rare:
				if rareSend {
					p.send(taskChan, seq)
				}
			}
		}
	}
}

func (p *producer) send(taskChan chan Task, seq int) {
	p.rl.Take()
	task := newTask(seq, p.subject, p.payload)
	taskChan <- task
	seq++
}
