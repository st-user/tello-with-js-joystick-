package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

var drone *tello.Driver
var videoFrameChannel chan []byte

func loadFileByName(filename string) string {
	path := filepath.Join("client", "static", filename)
	content, _ := ioutil.ReadFile(path)
	return string(content[:])
}

func loadFile(r *http.Request) string {
	filename := r.URL.Path[len("/"):]
	if filename == "" {
		filename = "index.html"
	}
	return loadFileByName(filename)
}

func checkThenAct(w http.ResponseWriter, actFunc func()) {
	if drone == nil {
		log.Println("Your drone has not been initialized yet.")
		w.WriteHeader(500)
		return
	}
	actFunc()
}

func connect(w http.ResponseWriter, r *http.Request) {

	go func() {

		if drone != nil {
			log.Println("Your drone has already been initialized.")
			return
		}

		drone = tello.NewDriverWithIP("192.168.10.1", "8888")
		drone.On(tello.ConnectedEvent, func(data interface{}) {
			fmt.Println("Starts receiving video frames from your drone.")
			drone.StartVideo()
			drone.SetVideoEncoderRate(tello.VideoBitRateAuto)
			gobot.Every(3*time.Second, func() {
				drone.StartVideo()
			})
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
				if videoFrameChannel != nil {
					videoFrameChannel <- buf
				}
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
}

func takeoff(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		drone.TakeOff()
	})
}

func land(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		drone.Land()
	})
}

func disconnect(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {

		drone.Land()
		time.Sleep(1 * time.Second)

		log.Printf("Halts the entire application.")
		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			log.Println(err)
			return
		}
		proc.Kill()
	})
}

func offer(w http.ResponseWriter, r *http.Request) {

	videoFrameChannel = make(chan []byte)

	offerSdp := webrtc.SessionDescription{}
	err := json.NewDecoder(r.Body).Decode(&offerSdp)
	writeErr := func(err error) {
		log.Println(err)
		w.WriteHeader(500)
	}
	if err != nil {
		writeErr(err)
		return
	}
	rtcPeerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		writeErr(err)
		return
	}
	cap := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}
	videoTrack, err := webrtc.NewTrackLocalStaticSample(cap, "video", "pion")
	if err != nil {
		writeErr(err)
		return
	}
	_, err = rtcPeerConnection.AddTrack(videoTrack)
	if err != nil {
		writeErr(err)
		return
	}

	rtcPeerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
	})

	log.Println(offerSdp)
	err = rtcPeerConnection.SetRemoteDescription(offerSdp)
	if err != nil {
		writeErr(err)
		return
	}
	answer, err := rtcPeerConnection.CreateAnswer(nil)
	if err != nil {
		writeErr(err)
		return
	}
	gatherComplete := webrtc.GatheringCompletePromise(rtcPeerConnection)
	rtcPeerConnection.SetLocalDescription(answer)

	<-gatherComplete

	go func() {

		latest := time.Now()

		for {

			frame, ok := <-videoFrameChannel
			if !ok {
				log.Println("The channel for video frames is closed.")
				return
			}
			videoTrack.WriteSample(media.Sample{
				Data: frame, Duration: time.Since(latest),
			})
			latest = time.Now()

		}
	}()

	localDescription := rtcPeerConnection.LocalDescription()
	json.NewEncoder(w).Encode(map[string]string{
		"sdp":  localDescription.SDP,
		"type": localDescription.Type.String(),
	})
}

func requestH264Params(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		drone.StartVideo()
	})
}

func videoOff(w http.ResponseWriter, r *http.Request) {
	close(videoFrameChannel)
}

func moveXy(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		xy := make(map[string]float32)
		json.NewDecoder(r.Body).Decode(&xy)

		droneX := xy["y"]
		droneY := xy["x"]

		// log.Printf("%v, %v", droneX, droneY)

		_, _, z, psi := drone.Vector()
		drone.SetVector(droneX, droneY, z, psi)
	})
}

func moveZr(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		zr := make(map[string]float32)
		json.NewDecoder(r.Body).Decode(&zr)

		droneZ := zr["z"]
		droneR := zr["r"]

		// log.Printf("%v, %v", droneZ, droneR)

		x, y, _, _ := drone.Vector()
		drone.SetVector(x, y, droneZ, droneR)
	})
}

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, loadFile(r))
	})
	http.HandleFunc("/connect", connect)
	http.HandleFunc("/takeoff", takeoff)
	http.HandleFunc("/land", land)
	http.HandleFunc("/disconnect", disconnect)
	http.HandleFunc("/videoOff", videoOff)
	http.HandleFunc("/offer", offer)
	http.HandleFunc("/requestH264Params", requestH264Params)
	http.HandleFunc("/moveXy", moveXy)
	http.HandleFunc("/moveZr", moveZr)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
