package hertzws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/websocket"

	"wethu/internal/protocol"
	"wethu/internal/rooms"
)

// Handler WebSocket处理器
type Handler struct {
	manager  *rooms.Manager
	upgrader websocket.HertzUpgrader
}

// NewHandler 创建新的WebSocket处理器
func NewHandler(manager *rooms.Manager) *Handler {
	return &Handler{
		manager: manager,
		upgrader: websocket.HertzUpgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(ctx *app.RequestContext) bool {
				return true
			},
		},
	}
}

// HandleWebSocket 处理WebSocket连接
func (h *Handler) HandleWebSocket(c context.Context, ctx *app.RequestContext) {
	// 使用context处理请求超时
	ctxWithTimeout, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()

	roomID := ctx.Param("roomId")
	token := ctx.Query("token")
	
	if token == "" {
		log.Printf("WebSocket: missing token for room %s", roomID)
		ctx.String(401, "missing token")
		return
	}
	
	// 查找房间和参与者
	room, participant, err := h.manager.LookupParticipant(roomID, token)
	if err != nil {
		log.Printf("WebSocket: lookup failed for room %s: %v", roomID, err)
		ctx.String(401, err.Error())
		return
	}
	
	// 升级HTTP连接为WebSocket连接
	err = h.upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		
		// 绑定连接到参与者
		participant.BindConnection(conn)
		
		// 启动发送循环
		sendDone := make(chan struct{})
		go func() {
			participant.SendLoop()
			close(sendDone)
		}()
		
		// 发送房间状态
		participant.Send(protocol.Envelope{
			Kind: "ROOM_STATE",
			Data: protocol.RoomStatePayload{Room: room.StateSnapshot()},
		})
		
		// 启动接收消息循环
		readDone := make(chan struct{})
		go func() {
			h.readLoop(ctxWithTimeout, room, participant, conn)
			close(readDone)
		}()
		
		// 等待任一goroutine完成
		waitForCompletion := make(chan struct{})
		go func() {
			select {
			case <-readDone:
			case <-sendDone:
			case <-ctxWithTimeout.Done():
			}
			close(waitForCompletion)
		}()
		
		select {
		case <-waitForCompletion:
			// 确保连接关闭
			conn.Close()
			// 连接关闭后清理
			room.DetachParticipant(participant.ID)
			h.manager.CleanupRoom(room)
		}
	})
	
	if err != nil {
		log.Printf("WebSocket: upgrade failed for room %s: %v", roomID, err)
		return
	}
}

// readLoop 读取WebSocket消息循环
func (h *Handler) readLoop(ctx context.Context, room *rooms.Room, participant *rooms.Participant, conn *websocket.Conn) {
	defer participant.Close()
	
	// 预分配消息缓冲区以减少内存分配
	messagePool := sync.Pool{
		New: func() interface{} {
			b := make([]byte, 8192)
			return &b
		},
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 读取消息
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("WebSocket: read error: %v", err)
				}
				break
			}
			
			// 只处理文本消息
			if msgType != websocket.TextMessage {
				continue
			}
			
			// 使用缓冲区处理消息以提高性能
			var inbound protocol.InboundEnvelope
			if len(data) > 8192 {
				// 对于大消息，直接解析
				if err := json.Unmarshal(data, &inbound); err != nil {
					log.Printf("WebSocket: unmarshal large message error: %v", err)
					continue
				}
			} else {
				// 对于小消息，使用对象池减少GC压力
			bufferPtr := messagePool.Get().(*[]byte)
			buffer := *bufferPtr
			if len(buffer) < len(data) {
				// 如果缓冲区太小，扩容
				buffer = make([]byte, len(data))
			}
			copy(buffer, data)
			if err := json.Unmarshal(buffer[:len(data)], &inbound); err != nil {
				log.Printf("WebSocket: unmarshal message error: %v", err)
				messagePool.Put(bufferPtr)
				continue
			}
			messagePool.Put(bufferPtr)
			}
			
			// 根据消息类型处理
			switch inbound.Kind {
			case "CONTROL":
				h.handleControlMessage(room, participant, inbound.Data)
			case "SYNC_REQUEST":
				h.handleSyncRequest(room, participant, inbound.Data)
			default:
				participant.Send(protocol.Envelope{
					Kind: "ERROR",
					Data: protocol.ErrorPayload{
						Code:    "unknown_kind",
						Message: "Unsupported message type",
					},
				})
			}
			
			// 更新读取超时
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
	}
}

// handleControlMessage 处理控制消息
func (h *Handler) handleControlMessage(room *rooms.Room, participant *rooms.Participant, data json.RawMessage) {
	// 检查是否为房主
	if !participant.IsHost {
		participant.Send(protocol.Envelope{
			Kind: "ERROR",
			Data: protocol.ErrorPayload{
				Code:    "unauthorized",
				Message: "Only host can control playback",
			},
		})
		return
	}
	
	// 解析控制消息
	var control protocol.ControlMessage
	if err := json.Unmarshal(data, &control); err != nil {
		log.Printf("WebSocket: unmarshal control message error: %v", err)
		return
	}
	
	// 设置时间戳
	if control.Payload.IssuedAt.IsZero() {
		control.Payload.IssuedAt = time.Now().UTC()
	}
	
	// 应用控制命令
	state, err := room.ApplyControl(participant.ID, control)
	if err != nil {
		participant.Send(protocol.Envelope{
			Kind: "ERROR",
			Data: protocol.ErrorPayload{
				Code:    "control_failed",
				Message: err.Error(),
			},
		})
		return
	}
	
	// 广播房间状态更新
	room.Broadcast(protocol.Envelope{
		Kind: "ROOM_STATE",
		Data: protocol.RoomStatePayload{Room: state},
	})
}

// handleSyncRequest 处理同步请求
func (h *Handler) handleSyncRequest(room *rooms.Room, participant *rooms.Participant, data json.RawMessage) {
	// 解析同步请求
	var req protocol.SyncRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("WebSocket: unmarshal sync request error: %v", err)
		return
	}
	
	// 发送房间状态
	participant.Send(protocol.Envelope{
		Kind: "ROOM_STATE",
		Data: protocol.RoomStatePayload{Room: room.StateSnapshot()},
	})
}