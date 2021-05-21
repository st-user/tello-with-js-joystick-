package main

import (
	"fmt"
	"log"
	"time"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

type MyDrone struct {
	Driver *tello.Driver
}

func NewMyDrone() MyDrone {
	return MyDrone{}
}

func (d *MyDrone) isInitialized() bool {
	return d.Driver != nil
}

func (d *MyDrone) Start(channels *Channels) {

	waitUntilConnected := make(chan struct{})
	var drone *tello.Driver
	go func() {

		if drone != nil {
			log.Println("Your drone has already been initialized.")
			return
		}

		drone = tello.NewDriverWithIP("192.168.10.1", "8888")
		d.Driver = drone

		drone.On(tello.ConnectedEvent, func(data interface{}) {
			fmt.Println("Starts receiving video frames from your drone.")
			drone.StartVideo()
			drone.SetVideoEncoderRate(tello.VideoBitRate4M)
			gobot.Every(10*time.Second, func() {
				drone.StartVideo()
			})
			close(waitUntilConnected)
		})

		lastLoggedTime := time.Now()
		drone.On(tello.FlightDataEvent, func(data interface{}) {
			if 3 < time.Since(lastLoggedTime).Seconds() {
				fd := data.(*tello.FlightData)
				log.Printf("Battery level %v%%", fd.BatteryPercentage)
				lastLoggedTime = time.Now()
			}
		})

		var buf []byte
		isNalUnitStart := func(b []byte) bool {
			return len(b) > 3 && b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 1
		}

		sendPreviousBytes := func(b []byte) bool {
			return len(b) > 4 && (b[4]&0b11111 == 7 || b[4]&0b11111 == 1)
		}

		loggedRecoverCount := 0
		handleData := func(_data interface{}) {

			defer func() {
				if r := recover(); r != nil {
					if loggedRecoverCount%100 == 0 {
						log.Printf("Ignores panic. %v", r)
						loggedRecoverCount = 0
					}
					loggedRecoverCount++
				}
			}()

			data := _data.([]byte)

			if !isNalUnitStart(data) || !sendPreviousBytes(data) {
				buf = append(buf, data...)
				return
			} else {
				channels.WriteToVideoFrameChannel(buf)
				var zero []byte
				buf = append(zero, data...)
			}

		}
		drone.On(tello.VideoFrameEvent, handleData)
		robot := gobot.NewRobot(
			[]gobot.Connection{},
			[]gobot.Device{drone},
		)
		robot.Start()
	}()

	<-waitUntilConnected
}

type SafetySignal struct {
	isStarted             bool
	endChannel            chan struct{}
	lastAccessedTimestamp time.Time
}

func NewSafetySignal() SafetySignal {
	return SafetySignal{}
}

// TODO use mutex
func (s *SafetySignal) StartChecking(drone *MyDrone) {
	if s.isStarted {
		return
	}
	s.endChannel = make(chan struct{})
	s.lastAccessedTimestamp = time.Now()
	s.isStarted = true
	go func() {

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.endChannel:
				return
			case <-ticker.C:
				if 500 < time.Since(s.lastAccessedTimestamp).Milliseconds() {
					log.Println("Set a zero translation vector because of losting stop signals.")
					drone.Driver.SetVector(0, 0, 0, 0)
					s.endChecking()
					return
				}
			}
		}
	}()
}

func (s *SafetySignal) ConsumeSignal(x float32, y float32) {
	if x == 0 && y == 0 {
		s.endChecking()
		return
	}
	s.lastAccessedTimestamp = time.Now()
}

func (s *SafetySignal) endChecking() {
	s.isStarted = false
	s.lastAccessedTimestamp = time.Now()
	close(s.endChannel)
}
