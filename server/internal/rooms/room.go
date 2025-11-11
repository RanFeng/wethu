package rooms

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/RanFeng/ilog"
	"sync"
	"time"

	"github.com/hertz-contrib/websocket"

	"wethu/internal/protocol"
)

var (
	ErrUnauthorizedControl = errors.New("only host can control playback")
)

type Room struct {
	Id           string                  `json:"id,omitempty"`
	OwnerID      string                  `json:"owner_id,omitempty"`
	VideoURL     string                  `json:"video_url,omitempty"`
	IsPlaying    bool                    `json:"is_playing,omitempty"`
	Position     float64                 `json:"position,omitempty"`
	UpdatedAt    time.Time               `json:"updated_at"`
	Participants map[string]*Participant `json:"participants,omitempty"`
	TokenIndex   map[string]string       `json:"token_index,omitempty"`
	mu           sync.RWMutex
}

type Participant struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Token       string `json:"token,omitempty"`
	IsHost      bool   `json:"is_host,omitempty"`
	conn        *websocket.Conn
	send        chan []byte
	connectedAt time.Time
	room        *Room
}

func NewRoom(roomID, ownerID, videoURL string, now time.Time) *Room {
	room := &Room{
		Id:           roomID,
		OwnerID:      ownerID,
		VideoURL:     videoURL,
		IsPlaying:    false,
		Position:     0,
		UpdatedAt:    now,
		Participants: make(map[string]*Participant),
		TokenIndex:   make(map[string]string),
	}
	return room
}

func (r *Room) AttachParticipant(userID, name, token string, isHost bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.Participants[userID]; exists {
		participant := r.Participants[userID]
		participant.Token = token
		return nil
	}

	r.Participants[userID] = &Participant{
		ID:          userID,
		Name:        name,
		Token:       token,
		IsHost:      isHost,
		send:        make(chan []byte, 8),
		connectedAt: time.Now().UTC(),
		room:        r,
	}
	r.TokenIndex[token] = userID
	if isHost {
		r.OwnerID = userID
	}

	return nil
}

func (r *Room) FindByToken(ctx context.Context, token string) (*Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ilog.EventInfo(ctx, "find_room_by_token", "token", token, "room_token", r.TokenIndex)
	userID, ok := r.TokenIndex[token]
	if !ok {
		ilog.EventError(ctx, ErrRoomNotFound, "room not found", "token", token, "room_token", r.TokenIndex)
		return nil, ErrInvalidToken
	}
	participant, ok := r.Participants[userID]
	if !ok {
		return nil, ErrParticipantNotFound
	}
	return participant, nil
}

func (r *Room) StateSnapshot() protocol.RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return protocol.RoomState{
		RoomID:    r.Id,
		VideoURL:  r.VideoURL,
		IsPlaying: r.IsPlaying,
		Position:  r.Position,
		OwnerID:   r.OwnerID,
		UpdatedAt: r.UpdatedAt,
	}
}

func (r *Room) Broadcast(envelope protocol.Envelope) {
	// 预先序列化消息
	data, err := json.Marshal(envelope)
	if err != nil {
		ilog.EventError(context.Background(), err, "Failed to marshal broadcast envelope")
		return
	}

	// 复制参与者列表以减少锁持有时间
	var participants []*Participant
	r.mu.RLock()
	for _, p := range r.Participants {
		if p.send != nil {
			participants = append(participants, p)
		}
	}
	r.mu.RUnlock()

	// 并发发送消息给多个参与者
	if len(participants) > 10 {
		// 对于大量参与者，使用worker池
		var wg sync.WaitGroup
		workChan := make(chan *Participant, len(participants))

		// 启动多个worker
		workerCount := min(10, len(participants))
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for p := range workChan {
					r.sendToParticipant(p, data)
				}
			}()
		}

		// 分发工作
		for _, p := range participants {
			workChan <- p
		}
		close(workChan)
		wg.Wait()
	} else {
		// 对于少量参与者，直接发送
		for _, p := range participants {
			r.sendToParticipant(p, data)
		}
	}
}

// sendToParticipant 安全地向单个参与者发送消息
func (r *Room) sendToParticipant(p *Participant, data []byte) {
	select {
	case p.send <- data:
		// 消息发送成功
	default:
		// 参与者的消息队列已满，可能是客户端处理缓慢或网络问题
		// 记录警告但不阻塞
		ilog.EventWarn(context.Background(), "Participant message queue full", "participant_id", p.ID)

		// 可以考虑实现背压机制，如暂时关闭连接或减少消息频率
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *Room) ApplyControl(senderID string, control protocol.ControlMessage) (protocol.RoomState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	participant, ok := r.Participants[senderID]
	if !ok || !participant.IsHost {
		return protocol.RoomState{}, ErrUnauthorizedControl
	}

	r.Position = control.Payload.Position
	if control.Payload.VideoURL != nil {
		r.VideoURL = *control.Payload.VideoURL
	}
	if control.Payload.Playing != nil {
		r.IsPlaying = *control.Payload.Playing
	}
	r.UpdatedAt = control.Payload.IssuedAt

	return protocol.RoomState{
		RoomID:    r.Id,
		VideoURL:  r.VideoURL,
		IsPlaying: r.IsPlaying,
		Position:  r.Position,
		OwnerID:   r.OwnerID,
		UpdatedAt: r.UpdatedAt,
	}, nil
}

func (r *Room) DetachParticipant(participantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if participant, ok := r.Participants[participantID]; ok {
		if participant.Token != "" {
			delete(r.TokenIndex, participant.Token)
		}
		close(participant.send)
		delete(r.Participants, participantID)
	}
}

func (r *Room) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Participants)
}

func (r *Room) ID() string {
	return r.Id
}

func (p *Participant) BindConnection(conn *websocket.Conn) {
	p.conn = conn
}

// SendLoop 发送消息循环
func (p *Participant) SendLoop() {
	defer p.Close()
	// 设置写超时
	if p.conn != nil {
		p.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}

	// 使用缓冲区批量发送消息
	batchSize := 5
	messageBatch := make([][]byte, 0, batchSize)
	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()

	for {
		timer.Reset(5 * time.Millisecond)
		select {
		case data, ok := <-p.send:
			if !ok {
				// 通道关闭，发送剩余消息
				//if len(messageBatch) > 0 && p.conn != nil {
				//	p.sendBatch(messageBatch)
				//}
				return
			}

			if p.conn == nil {
				continue
			}

			// 添加消息到批次
			messageBatch = append(messageBatch, data)

			// 如果批次已满，立即发送
			if len(messageBatch) >= batchSize {
				if !timer.Stop() {
					<-timer.C
				}
				p.sendBatch(messageBatch)
				messageBatch = messageBatch[:0]
			}

		case <-timer.C:
			// 时间到，发送当前批次
			if len(messageBatch) > 0 && p.conn != nil {
				p.sendBatch(messageBatch)
				messageBatch = messageBatch[:0]
			}
		}
	}
}

// sendBatch 批量发送消息
func (p *Participant) sendBatch(messages [][]byte) {
	for _, data := range messages {
		// 更新写超时
		p.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

		// 优化：尝试使用WriteJSON直接发送，减少序列化开销
		var envelope interface{}
		if err := json.Unmarshal(data, &envelope); err == nil {
			if err := p.conn.WriteJSON(envelope); err != nil {
				ilog.EventError(context.Background(), err, "WebSocket: write JSON error")
				return
			}
		} else {
			// 回退到原始方法
			if err := p.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				ilog.EventError(context.Background(), err, "WebSocket: write message error")
				return
			}
		}
	}
}

func (p *Participant) Close() {
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

func (p *Participant) Connection() *websocket.Conn {
	return p.conn
}

func (p *Participant) Send(envelope protocol.Envelope) {
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	if p.send == nil {
		return
	}
	select {
	case p.send <- data:
	default:
	}
}
