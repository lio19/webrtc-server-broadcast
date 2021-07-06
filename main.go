package main

import "flag"

func main() {
	hi, wp := parseFlag()
	sfu, err := NewSFUServer(hi, wp)
	if err != nil {
		panic(err.Error())
	}

	s, err := NewRoomServer(sfu)
	if err != nil {
		panic(err.Error())
	}

	if err = s.Start(); err != nil {
		err.Error()
	}
}

func parseFlag() (string, int) {
	hostIP := flag.String("ip", "127.0.0.1", "specify listen port")
	webrtcPort := flag.Int("wp", 8900, "specify webrtc mux port")
	flag.Parse()
	return *hostIP, *webrtcPort
}