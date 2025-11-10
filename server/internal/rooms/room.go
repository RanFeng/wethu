package rooms

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"wethu/internal/protocol"
)

var (
	ErrUnauthorizedControl = errors.New("only host can control playback")
)

type Room struct {
	id           string
	ownerID      string
	videoURL     string
	isPlaying    bool
	position     float64
	updatedAt    time.Time
	participants map[string]*Participant
	tokenIndex   map[string]string
	mu           sync.RWMutex
}

type Participant struct {
	ID          string
	Name        string
	Token       string
	IsHost      bool
	conn        *websocket.Conn
	send        chan []byte
	connectedAt time.Time
	room        *Room
}

func NewRoom(roomID, ownerID, videoURL string, now time.Time) *Room {
	room := &Room{
		id:           roomID,
		ownerID:      ownerID,
		videoURL:     videoURL,
		isPlaying:    false,
		position:     0,
		updatedAt:    now,
		participants: make(map[string]*Participant),
		tokenIndex:   make(map[string]string),
	}
	return room
}

func (r *Room) AttachParticipant(userID, name, token string, isHost bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.participants[userID]; exists {
		participant := r.participants[userID]
		participant.Token = token
		return nil
	}

	r.participants[userID] = &Participant{
		ID:          userID,
		Name:        name,
		Token:       token,
		IsHost:      isHost,
		send:        make(chan []byte, 8),
		connectedAt: time.Now().UTC(),
		room:        r,
	}
	r.tokenIndex[token] = userID
	if isHost {
		r.ownerID = userID
	}

	return nil
}

func (r *Room) FindByToken(token string) (*Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userID, ok := r.tokenIndex[token]
	if !ok {
		return nil, ErrInvalidToken
	}
	participant, ok := r.participants[userID]
	if !ok {
		return nil, ErrParticipantNotFound
	}
	return participant, nil
}

func (r *Room) StateSnapshot() protocol.RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return protocol.RoomState{
		RoomID:    r.id,
		VideoURL:  r.videoURL,
		IsPlaying: r.isPlaying,
		Position:  r.position,
		OwnerID:   r.ownerID,
		UpdatedAt: r.updatedAt,
	}
}

func (r *Room) Broadcast(envelope protocol.Envelope) {
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, participant := range r.participants {
		if participant.send != nil {
			select {
			case participant.send <- data:
			default:
			}
		}
	}
}

func (r *Room) ApplyControl(senderID string, control protocol.ControlMessage) (protocol.RoomState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	participant, ok := r.participants[senderID]
	if !ok || !participant.IsHost {
		return protocol.RoomState{}, ErrUnauthorizedControl
	}

	r.position = control.Payload.Position
	if control.Payload.VideoURL != nil {
		r.videoURL = *control.Payload.VideoURL
	}
	if control.Payload.Playing != nil {
		r.isPlaying = *control.Payload.Playing
	}
	r.updatedAt = control.Payload.IssuedAt

	return protocol.RoomState{
		RoomID:    r.id,
		VideoURL:  r.videoURL,
		IsPlaying: r.isPlaying,
		Position:  r.position,
		OwnerID:   r.ownerID,
		UpdatedAt: r.updatedAt,
	}, nil
}

func (r *Room) DetachParticipant(participantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if participant, ok := r.participants[participantID]; ok {
		if participant.Token != "" {
			delete(r.tokenIndex, participant.Token)
		}
		close(participant.send)
		delete(r.participants, participantID)
	}
}

func (r *Room) ParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.participants)
}

func (r *Room) ID() string {
	return r.id
}

func (p *Participant) BindConnection(conn *websocket.Conn) {
	p.conn = conn
}

func (p *Participant) SendLoop() {
	defer p.Close()
	for msg := range p.send {
		if p.conn == nil {
			continue
		}
		if err := p.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
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
