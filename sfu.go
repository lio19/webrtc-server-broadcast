package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"io"
	"net"
	"sync"
	"time"
)

type Room struct {
	ID string
	publisher *webrtc.PeerConnection
	rtpSet []*webrtc.TrackLocalStaticRTP
	player []*webrtc.PeerConnection
}

type SFUServer struct {
	rooms map[string]*Room
	roomsLock sync.Mutex

	udpMux ice.UDPMux
	tcpMux ice.TCPMux
	hostIP string
}


func (s *SFUServer)newPeer() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}

	setting := webrtc.SettingEngine{}
	setting.SetICETCPMux(s.tcpMux)
	setting.SetICEUDPMux(s.udpMux)
	setting.SetNAT1To1IPs([]string{s.hostIP}, webrtc.ICECandidateTypeHost)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(setting))

	return api.NewPeerConnection(webrtc.Configuration{})
}

func NewSFUServer(hostIP string, port int) (*SFUServer, error) {
	if hostIP == "" {
		return nil, errors.New("host ip is empty")
	}

	SFU := &SFUServer{
		rooms:     make(map[string]*Room, 10),
		roomsLock: sync.Mutex{},
	}

	uc, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IP{0, 0, 0, 0},
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	tc, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.IP{0, 0, 0, 0},
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	SFU.udpMux = webrtc.NewICEUDPMux(nil, uc)
	SFU.tcpMux = webrtc.NewICETCPMux(nil, tc, 20)
	SFU.hostIP = hostIP

	fmt.Println("New SUF port = ", port, " host ip = ", hostIP)

	return SFU, nil
}


func (s *SFUServer)NewPublish(roomID string, sdpStr string) (string, error) {

	//获取对饮的room id
	s.roomsLock.Lock()
	defer s.roomsLock.Unlock()
	r, ok :=  s.rooms[roomID]
	if ok {
		return "", errors.New("已经存在 room = " + roomID)
	}

	r = &Room{ID:roomID}
	s.rooms[roomID] = r
	
	pc, err := s.newPeer()
	if err != nil {
		return "", err
	}

	var SDP webrtc.SessionDescription

	b, err := base64.StdEncoding.DecodeString(sdpStr)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(b, &SDP)
	if err != nil {
		return "", err
	}

	if err = pc.SetRemoteDescription(SDP); err != nil {
		return "", err
	}

	if localSDP, err := pc.CreateAnswer(nil); err != nil {
		return "", err
	} else if err = pc.SetLocalDescription(localSDP); err != nil {
			return "", err
	}

	//TODO: 将panic正常处理
	pc.OnTrack(func(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if ck := remote.Kind(); ck == webrtc.RTPCodecTypeAudio {
			//音频来了是不是可以直接做转发
			audioTrack, err := webrtc.NewTrackLocalStaticRTP(remote.Codec().RTPCodecCapability, "audio", "lqq")
			//创建失败
			if err != nil {
				panic(err.Error())
			}
			rtpBuf := make([]byte, 1400)

			r.rtpSet = append(r.rtpSet, audioTrack)
			for {
				i, _, readErr := remote.Read(rtpBuf)
				if readErr != nil {
					fmt.Println("read error = ", readErr.Error())
				}
				if _, err := audioTrack.Write(rtpBuf[:i]); err != nil  && !errors.Is(err, io.ErrClosedPipe) {
					fmt.Println("write error = ", err.Error())
				}
			}
		} else if ck == webrtc.RTPCodecTypeVideo {

			//直接设置对端的编码能力
			videoTrack, err := webrtc.NewTrackLocalStaticRTP(remote.Codec().RTPCodecCapability, "video", "lqq")

			//创建失败
			if err != nil {
				//这个时候是不是可以说明失败了
				panic(err.Error())
			}

			r.rtpSet = append(r.rtpSet, videoTrack)

			//隔一段时间请求pli请求
			go func() {
				ticker := time.NewTicker(time.Second * 5)
				for range ticker.C {
					if rtcpSendErr := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())}}); rtcpSendErr != nil {
						fmt.Println(rtcpSendErr)
					}
				}
			}()

			rtpBuf := make([]byte, 1400)
			for {
				i, _, readErr := remote.Read(rtpBuf)
				if readErr != nil {
					//失败
					fmt.Println("room = ", roomID, " err = ", readErr.Error())
				}

				if _, err := videoTrack.Write(rtpBuf[:i]); err != nil  && !errors.Is(err, io.ErrClosedPipe) {
					//失败
					fmt.Println("room = ", roomID, " err = ", err.Error())
				}
			}
		} else {

		}
	})

	pr := webrtc.GatheringCompletePromise(pc)
	<- pr

	//base64编码
	ldBytes, err := json.Marshal(*pc.LocalDescription())
	if err != nil {
		return "", err
	}

	//将sdp返回
	return base64.StdEncoding.EncodeToString(ldBytes), nil
}


func (s *SFUServer)NewPlay(roomID, sdpStr string) (string, error) {

	//获取对饮的room id
	s.roomsLock.Lock()
	defer s.roomsLock.Unlock()
	r, ok :=  s.rooms[roomID]
	if !ok {
		return "", errors.New("不存在这个room")
	} else if r == nil {
		return "", errors.New("房间存在问题")
	}


	pc, err := s.newPeer()
	if err != nil {
		return "", err
	}

	for _, t := range r.rtpSet {
		sender, err := pc.AddTrack(t)
		if err != nil {
			return "", err
		}

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()
	}

	var SDP webrtc.SessionDescription

	b, err := base64.StdEncoding.DecodeString(sdpStr)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(b, &SDP)
	if err != nil {
		return "", err
	}
	if err = pc.SetRemoteDescription(SDP); err != nil {
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		return "", err
	}

	pr := webrtc.GatheringCompletePromise(pc)
	<- pr

	//base64编码
	ldBytes, err := json.Marshal(*pc.LocalDescription())
	if err != nil {
		return "", err
	}

	//将sdp返回
	return base64.StdEncoding.EncodeToString(ldBytes), nil
}