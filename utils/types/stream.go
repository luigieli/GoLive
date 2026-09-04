package types

type StreamInfo struct {
	NodeID     int `json:"node_id"`
	PipeWireFD int `json:"pipewire_fd"`
	Width      int `json:"width"`
	Height     int `json:"height"`
}

type CaptureOptions struct {
	Width        int
	Height       int
	Framerate    int
	VideoBitrate int
	Encoder      string
	CPUThreads   int
	NodeID       int
	PipeWireFD   int
	AudioSource  string
}
