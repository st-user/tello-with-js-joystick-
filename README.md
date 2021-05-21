# tello-with-js-joystick (& video controlled with RTCP)

A web application for controlling [Tello/Tello EDU](https://www.ryzerobotics.com/jp/tello) on a browser by a UI emulating joysticks. Video streaming is controlled with **RTCP**.

This application written in [GO language](https://golang.org/)(server side) and pure javascript/html/css(client side).
The video streaming is achieved using WebRTC.

This application is made possible thanks to [pion/webrtc](https://github.com/pion/webrtc) and [gobot](https://gobot.io/). 

Also I learned a lot from [oliverpool/tello-webrtc-fpv](https://github.com/oliverpool/tello-webrtc-fpv). Thanks to this project, I was able to figure out the way to send h264 packets from Tello to the browser.

## Demo

![tello-joystick-fly-animation](./assests/tello-joystick-fly.gif)

## How to use

 - Install [Go](https://golang.org/doc/install)
 - Clone this repository and run the application:
 
   ````
   git clone https://github.com/st-user/tello-with-js-joystick-.git
   cd tello-with-js-joystick-
   go run .
   ````

   For the first time you run the commands above, some dependencies are downloaded.

 - Connect your PC to Tello via Wi-fi.
 - Open your browser and access:

   ```
   http://localhost:8080
   ```

## Motivations

 - To control Tello through a browser UI emulating joysticks.
 - To control Tello's video streaming with RTCP protocol. RTCP packets are received from the browser.

## How to control Tello with joysticks.

 The important method is gobot's `SetVector` method([GoDoc](https://pkg.go.dev/gobot.io/x/gobot/platforms/dji/tello#Driver.SetVector)). This method sets Tello's current motion vector.


## What type of RTCP packet does this application handle?

Currently, this application can handle two types of RTCP packets:

 ### PictureLossIndication

 When a `PictureLossIndication` packet is received, this application tells Tello to send start info for video stream. To do it, this application call gobot's `StartVideo` method([GoDoc](https://pkg.go.dev/gobot.io/x/gobot/platforms/dji/tello#Driver.StartVideo)).

 ### ReceiverEstimatedMaximumBitrate

 When a `ReceiverEstimatedMaximumBitrate` packet is received, this application tells Tello to change the bit rate for the streaming video. 
  
 Through gobot's `SetVideoEncoderRate` method([GoDoc](https://pkg.go.dev/gobot.io/x/gobot/platforms/dji/tello#Driver.SetVideoEncoderRate)), Tello can accept 1Mb/s, 1.5Mb/s, 2Mb/s, 3Mb/s and 4Mb/s. From these five bit rates, this application chooses the largest bit rate not exceeding the receiver(browser) estimation. If a receiver estimation is less than 1Mb/s, this application chooses 1Mb/s.

 For more information about RTCP, please see the references below.

## Environment on which I tested

### GO

 - GO 1.16

### Browsers

 - macOS Big Sur 11.3.1
    - Google Chrome
    - Microsoft Edge
    - Safari
    - FireFox
 - Windows 10
    - Google Chrome
    - Microsoft Edge
    - FireFox

## Reference

### RTCP

 - [WebRTC for the Curious - Media Communication](https://webrtcforthecurious.com/docs/06-media-communication/)



 