package eventmanager

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/chriskaliX/SDK"
	"github.com/chriskaliX/SDK/transport/protocol"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type EventManager struct {
	s    SDK.ISandbox
	m    map[int]*Event
	cron *cron.Cron
}

func New(s *SDK.Sandbox) *EventManager {
	return &EventManager{
		s: s,
		m: make(map[int]*Event),
		cron: cron.New(
			cron.WithChain(
				cron.SkipIfStillRunning(&logWrapper{
					logger: s.Logger,
				}),
			),
		),
	}
}

func (e *EventManager) AddEvent(event IEvent, t time.Duration) {
	zap.S().Info(fmt.Sprintf("%s is added, duration: %dm, flag: %d", event.Name(), int(t.Minutes()), event.Flag()))
	e.m[event.DataType()] = &Event{
		event:    event,
		interval: t,
		done:     make(chan struct{}, 1),
		sig:      make(chan struct{}, 1),
	}
	e.m[event.DataType()].done <- struct{}{}
}

func (e *EventManager) Run(s SDK.ISandbox) error {
	zap.S().Info("eventmanager running")
	// Run the immediately firstly
	for _, event := range e.m {
		if event.event.Immediately() {
			zap.S().Infof("%s first run", event.event.Name())
			event.Start(s)
		}
	}
	// Start the goroutines
	for _, event := range e.m {
		switch event.event.Flag() {
		case Realtime:
			go event.Start(s)
		case Periodic:
			go func(ev *Event) {
				if !ev.event.Immediately() {
					var rint = 600
					if s.Debug() {
						rint = 10
					}
					r := rand.Intn(rint)
					time.Sleep(time.Duration(r) * time.Second)
					zap.S().Infof("%s first run", ev.event.Name())
					ev.Start(s)
				}
				id, _ := e.cron.AddFunc(
					fmt.Sprintf("@every %dm", int(ev.interval.Minutes())),
					func() { ev.Start(s) },
				)
				ev.id = id
			}(event)
		case Trigger:
			// just ignore this
		}
	}
	go e.taskResolve()
	e.cron.Run()
	return nil
}

// collector task resolve
// only data_type and (an int interval)
func (e *EventManager) taskResolve() {
	for {
		task := e.s.RecvTask()
		// exit if task is nil, which should not happen
		if task == nil {
			return
		}
		// look up events by data type
		event, ok := e.m[int(task.DataType)]
		if !ok {
			zap.S().Error(fmt.Sprintf("%d is invalid", task.DataType))
			continue
		}

		var data = &protocol.Record{
			DataType:  5100,
			Timestamp: time.Now().Unix(),
			Data: &protocol.Payload{
				Fields: map[string]string{
					"status": "successed",
					"msg":    "",
					"token":  task.Token,
				},
			},
		}

		switch event.event.Flag() {
		case Trigger:
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-event.done:
					go event.Start(e.s)
					goto Send
				case <-timer.C:
					serr := fmt.Sprintf("%s job is running", event.event.Name())
					zap.S().Error(serr)
					data.Data.Fields = map[string]string{
						"status": "failed",
						"msg":    serr,
					}
					goto Send
				}
			}
		case Periodic:
			// All trigger by interval
			interval, err := strconv.Atoi(task.Data)
			if err != nil {
				zap.S().Error(err)
				continue
			}
			if interval > 0 {
				event.interval = time.Duration(interval) * time.Minute
				e.cron.Remove(event.id)
				id, _ := e.cron.AddFunc(
					fmt.Sprintf("@every %dm", int(event.interval.Minutes())),
					func() { event.Start(e.s) },
				)
				event.id = id
				goto Send
			}
			if err := event.Stop(e.cron); err != nil {
				serr := fmt.Sprintf("%s stop fail", event.event.Name())
				zap.S().Error(serr)
				data.Data.Fields = map[string]string{
					"status": "failed",
					"msg":    serr,
				}
			}
		case Realtime:
			// All trigger by interval
			interval, err := strconv.Atoi(task.Data)
			if err != nil {
				zap.S().Error(err)
				continue
			}
			if interval > 0 {
				go event.Start(e.s)
				goto Send
			}
			if err := event.Stop(e.cron); err != nil {
				serr := fmt.Sprintf("%s stop fail", event.event.Name())
				zap.S().Error(serr)
				data.Data.Fields = map[string]string{
					"status": "failed",
					"msg":    serr,
				}
			}
		}
	Send:
		e.s.SendRecord(data)
	}
}
