package main

import (
	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
	"log"
	"net/http"
	"time"
)

type RoomServer struct {
	ginEngine *gin.Engine
	sfu       *SFUServer
}

const (
	OK = iota
	Fail
)

type SignalRequest struct {
	RoomID string `json:"roomID"`
	SDP    string `json:"sdp"`
}

type SignalResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	SDP  string `json:"sdp"`
}

func NewRoomServer(sfu *SFUServer) (*RoomServer, error) {
	return &RoomServer{
		sfu:       sfu,
		ginEngine: gin.Default()}, nil
}

func (r *RoomServer) Start() error {

	var err error
	r.ginEngine.LoadHTMLFiles("publisher.html", "player.html")
	r.ginEngine.GET("/publisher", func(c *gin.Context) {
		c.HTML(http.StatusOK, "publisher.html", nil)
	})
	r.ginEngine.GET("/player", func(c *gin.Context) {
		c.HTML(http.StatusOK, "player.html", nil)
	})
	r.ginEngine.Use(TlsHandler())
	r.ginEngine.Use(middleware())

	r.ginEngine.POST("/publish", func(context *gin.Context) {
		var s SignalRequest
		if err = context.BindJSON(&s); err != nil {
			context.JSON(200, SignalResponse{
				Code: Fail,
				Msg:  err.Error(),
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
				Msg:  err.Error(),
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

func TlsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		secureMiddleware := secure.New(secure.Options{
			SSLRedirect: true,
			SSLHost:     "localhost:8080",
		})
		err := secureMiddleware.Process(c.Writer, c.Request)

		// If there was an error, do not continue.
		if err != nil {
			return
		}

		c.Next()
	}
}

func middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		c.Set("example", "123456")
		// c.Next()后就执行真实的路由函数，路由执行完成之后接着走time.Since(t)

		c.Next()

		// 从time.Now()到目前为止过了多长时间
		latency := time.Since(t)
		log.Print("--", latency)

		// gin设置响应头，设置跨域
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Action, Module, X-PINGOTHER, Content-Type, Content-Disposition")

		//设置中间件的响应头，路由的响应头可以在路由返回中设置，参考/ping
		// c.Writer.WriteHeader(http.StatusMovedPermanently)
		status := c.Writer.Status()
		log.Println("==", status)
	}
}
