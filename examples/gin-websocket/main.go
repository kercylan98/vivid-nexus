package main

import (
	"net/http"

	"github.com/kercylan98/vivid/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kercylan98/vivid"
	nexus "github.com/kercylan98/vivid-nexus"
	"github.com/kercylan98/vivid-nexus/examples/gin-websocket/session"
	"github.com/kercylan98/vivid/pkg/bootstrap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求，生产环境下应谨慎处理
	},
}

func main() {
	nexusInstance := initNexusActor()
	actorSystem := initActorSystem()
	router := initRouter()

	if _, err := actorSystem.ActorOf(nexusInstance.Actor()); err != nil {
		panic(err)
	}

	router.GET("/ws", func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			panic(err)
		}

		sessionId := c.Query("id")
		if sessionId == "" {
			ws.Close()
			return
		}

		session := session.NewSession(sessionId, ws)
		nexusInstance.TakeoverSession(session)
	})

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}

func initNexusActor() nexus.Nexus {
	nexusActor, err := nexus.New(nexus.SessionActorProviderFN(func() (nexus.SessionActor, error) {
		return new(session.Actor), nil
	}))
	if err != nil {
		panic(err)
	}
	return nexusActor
}

func initActorSystem() vivid.ActorSystem {
	system := bootstrap.NewActorSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelDebug))))
	if err := system.Start(); err != nil {
		panic(err)
	}
	return system
}

func initRouter() *gin.Engine {
	router := gin.Default()
	return router
}
