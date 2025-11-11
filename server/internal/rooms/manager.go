package rooms

import (
	"context"
	"errors"
	"fmt"
	"github.com/RanFeng/ilog"
	"math/rand"
	"sync"
	"time"

	"wethu/internal/protocol"
)

var (
	ErrRoomNotFound        = errors.New("room not found")
	ErrParticipantNotFound = errors.New("participant not found")
	ErrInvalidToken        = errors.New("invalid token")
)

type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

type Session struct {
	RoomID string             `json:"roomId"`
	UserID string             `json:"userId"`
	Token  string             `json:"token"`
	IsHost bool               `json:"isHost"`
	State  protocol.RoomState `json:"state"`
}

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

func (m *Manager) CreateRoom(displayName, videoURL string) (*Session, error) {
	roomID := generateID("room")
	userID := generateID("user")
	token := generateID("tok")

	now := time.Now().UTC()

	room := NewRoom(roomID, userID, videoURL, now)

	m.mu.Lock()
	m.rooms[roomID] = room
	m.mu.Unlock()

	if err := room.AttachParticipant(userID, displayName, token, true); err != nil {
		return nil, err
	}

	return &Session{
		RoomID: roomID,
		UserID: userID,
		Token:  token,
		IsHost: true,
		State:  room.StateSnapshot(),
	}, nil
}

func (m *Manager) JoinRoom(roomID, displayName string) (*Session, error) {
	m.mu.RLock()
	room, ok := m.rooms[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrRoomNotFound
	}

	userID := generateID("user")
	token := generateID("tok")
	if err := room.AttachParticipant(userID, displayName, token, false); err != nil {
		return nil, err
	}

	return &Session{
		RoomID: roomID,
		UserID: userID,
		Token:  token,
		IsHost: false,
		State:  room.StateSnapshot(),
	}, nil
}

func (m *Manager) GetState(roomID string) (protocol.RoomState, error) {
	m.mu.RLock()
	room, ok := m.rooms[roomID]
	m.mu.RUnlock()
	if !ok {
		return protocol.RoomState{}, ErrRoomNotFound
	}
	return room.StateSnapshot(), nil
}

func (m *Manager) LookupParticipant(roomID, token string) (*Room, *Participant, error) {
	ctx := context.Background()
	m.mu.RLock()
	ilog.EventInfo(ctx, "Looking up participant", "roomID", roomID, "token", token, "m.rooms", m.rooms)
	room, ok := m.rooms[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil, nil, ErrRoomNotFound
	}
	participant, err := room.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	return room, participant, nil
}

func generateID(prefix string) string {
	//const size = 6
	//buf := make([]byte, size)
	//if _, err := rand.Read(buf); err != nil {
	//	panic(fmt.Errorf("unable to read random bytes: %w", err))
	//}
	r := rand.Intn(900) + 10
	return fmt.Sprintf("%s_%d", prefix, r)
}

func (m *Manager) CleanupRoom(room *Room) {
	if room == nil {
		return
	}
	if room.ParticipantCount() > 0 {
		return
	}
	roomID := room.ID()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.rooms[roomID]
	if ok && current == room {
		delete(m.rooms, roomID)
	}
}
