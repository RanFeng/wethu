package protocol

import (
	"encoding/json"
	"time"
)

type RoomState struct {
	RoomID    string    `json:"roomId"`
	VideoURL  string    `json:"videoUrl"`
	IsPlaying bool      `json:"isPlaying"`
	Position  float64   `json:"position"`
	OwnerID   string    `json:"ownerId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ControlMessage struct {
	Type    string         `json:"type"`
	RoomID  string         `json:"roomId"`
	Sender  string         `json:"senderId"`
	Payload ControlPayload `json:"payload"`
}

type ControlPayload struct {
	Position float64   `json:"position"`
	VideoURL *string   `json:"videoUrl,omitempty"`
	Playing  *bool     `json:"isPlaying,omitempty"`
	IssuedAt time.Time `json:"issuedAt"`
}

type RoomStatePayload struct {
	Room RoomState `json:"room"`
}

type SyncRequest struct {
	RoomID   string `json:"roomId"`
	SenderID string `json:"senderId"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	Kind string      `json:"kind"`
	Data interface{} `json:"data"`
}

type InboundEnvelope struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}
