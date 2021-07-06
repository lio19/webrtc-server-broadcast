package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type RoomServer struct {
	ginEngine *gin.Engine
	sfu *SFUServer
}



const (
	OK = iota
	Fail
)

type SignalRequest struct {
	RoomID string `json:"roomID"`
	SDP string `json:"sdp"`
}

type SignalResponse struct {
	Code int `json:"code"`
	Msg string `json:"message"`
	SDP string `json:"sdp"`
}

func NewRoomServer(sfu *SFUServer) (*RoomServer, error){
	return &RoomServer{
		sfu: sfu,
		ginEngine: gin.Default()}, nil
}

func (r *RoomServer) Start() error{

	var err error
	r.ginEngine.LoadHTMLFiles("publisher.html", "player.html")
	r.ginEngine.GET("/publisher", func(c *gin.Context) {
		c.HTML(http.StatusOK, "publisher.html", nil)
	})
	r.ginEngine.GET("/player", func(c *gin.Context) {
		c.HTML(http.StatusOK, "player.html", nil)
	})

	r.ginEngine.POST("/publish", func(context *gin.Context) {
		var s SignalRequest
		 if err = context.BindJSON(&s); err != nil {
		 	context.JSON(200, SignalResponse{
				Code: Fail,
				Msg: err.Error(),
				SDP:  "",
			})
		 } else if answer, err := r.sfu.NewPublish(s.RoomID, s.SDP); err != nil {
			 context.JSON(200, SignalResponse{
				 Code: Fail,
				 Msg:  err.Error(),
				 SDP:  "",
			 })
		 } else {
		 	context.JSON(200, SignalResponse{
				Code: OK,
				Msg:  "",
				SDP:  answer,
			})
		 }
	})
	
	r.ginEngine.POST("/play", func(context *gin.Context) {
		var s SignalRequest
		if err = context.BindJSON(&s); err != nil {
			context.JSON(200, SignalResponse{
				Code: Fail,
				Msg: err.Error(),
				SDP:  "",
			})
		} else if answer, err := r.sfu.NewPlay(s.RoomID, s.SDP); err != nil {
			context.JSON(200, SignalResponse{
				Code: Fail,
				Msg:  err.Error(),
				SDP:  "",
			})
		} else {
			context.JSON(200, SignalResponse{
				Code: OK,
				Msg:  "",
				SDP:  answer,
			})
		}
	})


	if err = r.ginEngine.Run(":8080"); err != nil {
		return err
	}
	return nil
}