package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type room struct {
	players [2]*websocket.Conn
	seed    int64
}

type roomManager struct {
	mu      sync.Mutex
	waiting *websocket.Conn
}

var manager = &roomManager{}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct{}

func NewWSHandler() *WSHandler {
	return &WSHandler{}
}

func (h *WSHandler) Match(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("웹소켓 업그레이드 실패")
		return
	}

	manager.mu.Lock()

	if manager.waiting == nil {
		manager.waiting = ws
		manager.mu.Unlock()

		log.Debug().Msg("플레이어 대기 중")
		ws.WriteJSON(gin.H{"type": "waiting", "message": "상대방을 기다리는 중..."})

		disconnected := make(chan struct{})
		go func() {
			for {
				if _, _, err := ws.ReadMessage(); err != nil {
					close(disconnected)
					return
				}
			}
		}()

		select {
		case <-disconnected:
			cleanWaiting(ws)
			log.Debug().Msg("대기 중 연결 끊김")

		case <-time.After(60 * time.Second):
			cleanWaiting(ws)
			ws.WriteJSON(gin.H{"type": "timeout", "message": "매칭 시간이 초과되었습니다."})
			ws.Close()
			log.Debug().Msg("매칭 타임아웃")
		}

	} else {
		opponent := manager.waiting
		manager.waiting = nil
		manager.mu.Unlock()

		seed := time.Now().UnixMilli() // (크롬 에러 방지용 UnixMilli 유지)
		matchData := gin.H{
			"type":       "matched",
			"seed":       seed,
			"difficulty": "medium",
		}

		opponent.WriteJSON(matchData)
		ws.WriteJSON(matchData)

		log.Info().Int64("seed", seed).Msg("매칭 성사")

		startRelay(ws, opponent)
	}
}

// 👇 [핵심 수정] 한 명이 나가면 생존자에게 'opponent_left' 메시지를 날림!
func startRelay(conn1, conn2 *websocket.Conn) {
	var once sync.Once

	// 생존자(survivor)에게 메시지를 쏘고 둘 다 안전하게 닫는 함수
	closeAll := func(survivor *websocket.Conn) {
		once.Do(func() {
			if survivor != nil {
				survivor.WriteJSON(gin.H{"type": "opponent_left"})
				// 메시지가 날아갈 0.1초의 시간을 벌어줍니다.
				time.Sleep(100 * time.Millisecond)
			}
			conn1.Close()
			conn2.Close()
			log.Debug().Msg("대전 종료: 연결 닫힘")
		})
	}

	go func() {
		for {
			msgType, msg, err := conn1.ReadMessage()
			if err != nil {
				closeAll(conn2) // conn1이 나갔으니 conn2가 생존자
				return
			}
			if err := conn2.WriteMessage(msgType, msg); err != nil {
				closeAll(conn1)
				return
			}
		}
	}()

	for {
		msgType, msg, err := conn2.ReadMessage()
		if err != nil {
			closeAll(conn1) // conn2가 나갔으니 conn1이 생존자
			return
		}
		if err := conn1.WriteMessage(msgType, msg); err != nil {
			closeAll(conn2)
			return
		}
	}
}

func cleanWaiting(ws *websocket.Conn) {
	manager.mu.Lock()
	if manager.waiting == ws {
		manager.waiting = nil
	}
	manager.mu.Unlock()
}
