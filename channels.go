package main

type Channels struct {
	VideoFrameChannel       chan []byte
	RtcEventLoopStopChannel chan struct{}
}

func NewChannels() Channels {
	return Channels{}
}

func (c *Channels) Init() {
	c.VideoFrameChannel = make(chan []byte)
	c.RtcEventLoopStopChannel = make(chan struct{})
}

func (c *Channels) VideoOff() {
	close(c.VideoFrameChannel)
	close(c.RtcEventLoopStopChannel)
}

func (c *Channels) WriteToVideoFrameChannel(data []byte) {
	if c.VideoFrameChannel != nil {
		c.VideoFrameChannel <- data
	}
}
