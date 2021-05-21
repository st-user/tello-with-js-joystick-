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

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"gobot.io/x/gobot/platforms/dji/tello"
)

var drone MyDrone
var safetySignal SafetySignal
var channels Channels

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
	if !drone.isInitialized() {
		log.Println("Your drone has not been initialized yet.")
		w.WriteHeader(500)
		return
	}
	actFunc()
}

func connect(w http.ResponseWriter, r *http.Request) {
	if drone.isInitialized() {
		log.Println("Your drone has already been initialized.")
		return
	}
	drone.Start(&channels)
}

func takeoff(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		drone.Driver.TakeOff()
	})
}

func land(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		drone.Driver.Land()
	})
}

func disconnect(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {

		drone.Driver.Land()
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

	channels.Init()

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
	rtpSender, err := rtcPeerConnection.AddTrack(videoTrack)
	if err != nil {
		writeErr(err)
		return
	}

	go func() {
		rtcpBuf := make([]byte, 1500)

		select {
		case <-channels.RtcEventLoopStopChannel:
			log.Println("Stops WebRTC event loop.")
			return
		default:
			for {
				n, _, rtcpErr := rtpSender.Read(rtcpBuf)
				if rtcpErr != nil {
					return
				}
				rtcpPacket := rtcpBuf[:n]

				/*
					payloadType := rtcpPacket[1]
					if payloadType == 206 {
						log.Printf("rtcp detail %v %v", pkts, rtcpPacket)
						// 'R' 'E' 'M' 'B' = 82 69 77 66
					}*/

				pkts, err := rtcp.Unmarshal(rtcpPacket)
				if err != nil {
					log.Println(err)
					return
				}

				for _, pkt := range pkts {
					var _p interface{} = pkt

					switch _pkt := _p.(type) {
					case *rtcp.PictureLossIndication:
						log.Printf("Receives RTCP PictureLossIndication. %v", _pkt)
						drone.Driver.StartVideo()

					case *rtcp.ReceiverEstimatedMaximumBitrate:
						log.Printf("Receives RTCP ReceiverEstimatedMaximumBitrate. %v", _pkt)
						bitrate := float64(_pkt.Bitrate)

						// Using the bitrate(MB) value corresponding to the one that 'rtcp.Receiver Estimated Maximum Bitrate.String()' shows.
						// Reference: github.com/pion/rtcp receiver_estimated_maximum_bitrate.go
						bitrateMB := bitrate / 1000.0 / 1000.0 // :MB
						var changeTo float64

						switch {
						case bitrateMB >= 4.0:
							drone.Driver.SetVideoEncoderRate(tello.VideoBitRate4M)
							changeTo = 4.0
						case bitrateMB >= 3.0:
							drone.Driver.SetVideoEncoderRate(tello.VideoBitRate3M)
							changeTo = 3.0
						case bitrateMB >= 2.0:
							drone.Driver.SetVideoEncoderRate(tello.VideoBitRate2M)
							changeTo = 2.0
						case bitrateMB >= 1.5:
							drone.Driver.SetVideoEncoderRate(tello.VideoBitRate15M)
							changeTo = 1.5
						default:
							drone.Driver.SetVideoEncoderRate(tello.VideoBitRate1M)
							changeTo = 1
						}
						log.Printf("ReceiverEstimation = %.2f MB. Changes to %v MB", bitrateMB, changeTo)
					}
				}
			}
		}
	}()

	rtcPeerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
	})

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

			frame, ok := <-channels.VideoFrameChannel
			if !ok {
				rtcPeerConnection.Close()
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

func videoOff(w http.ResponseWriter, r *http.Request) {
	channels.VideoOff()
}

func moveXy(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		xy := make(map[string]float32)
		json.NewDecoder(r.Body).Decode(&xy)

		droneX := xy["y"]
		droneY := xy["x"]

		safetySignal.ConsumeSignal(droneX, droneY, &drone)

		_, _, z, psi := drone.Driver.Vector()
		drone.Driver.SetVector(droneX, droneY, z, psi)
	})
}

func moveZr(w http.ResponseWriter, r *http.Request) {
	checkThenAct(w, func() {
		zr := make(map[string]float32)
		json.NewDecoder(r.Body).Decode(&zr)

		droneZ := zr["z"]
		droneR := zr["r"]

		safetySignal.ConsumeSignal(droneZ, droneR, &drone)

		x, y, _, _ := drone.Driver.Vector()
		drone.Driver.SetVector(x, y, droneZ, droneR)
	})
}

func main() {

	drone = NewMyDrone()
	safetySignal = NewSafetySignal()
	channels = NewChannels()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, loadFile(r))
	})
	http.HandleFunc("/connect", connect)
	http.HandleFunc("/takeoff", takeoff)
	http.HandleFunc("/land", land)
	http.HandleFunc("/disconnect", disconnect)
	http.HandleFunc("/videoOff", videoOff)
	http.HandleFunc("/offer", offer)
	http.HandleFunc("/moveXy", moveXy)
	http.HandleFunc("/moveZr", moveZr)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
