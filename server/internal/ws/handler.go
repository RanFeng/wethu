package ws

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"wethu/internal/protocol"
	"wethu/internal/rooms"
)

type Handler struct {
	manager  *rooms.Manager
	upgrader websocket.Upgrader
}

func NewHandler(manager *rooms.Manager) *Handler {
	return &Handler{
		manager: manager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	roomID, err := extractRoomID(r.URL.Path)
	if err != nil {
		log.Printf("WebSocket: invalid room path: %s", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid room path"))
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		log.Printf("WebSocket: missing token for room %s", roomID)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("missing token"))
		return
	}

	room, participant, err := h.manager.LookupParticipant(roomID, token)
	if err != nil {
		log.Printf("WebSocket: lookup failed for room %s: %v", roomID, err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(err.Error()))
		return
	}

	log.Printf("WebSocket: upgrading connection for room %s, participant %s", roomID, participant.ID)
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket: upgrade failed for room %s: %v", roomID, err)
		// If upgrade fails, connection may already be partially written
		// Don't try to write error response as it may cause EPIPE
		return
	}
	log.Printf("WebSocket: connection upgraded successfully for room %s", roomID)

	participant.BindConnection(conn)
	go participant.SendLoop()

	participant.Send(protocol.Envelope{
		Kind: "ROOM_STATE",
		Data: protocol.RoomStatePayload{Room: room.StateSnapshot()},
	})

	h.readLoop(room, participant)
	room.DetachParticipant(participant.ID)
	h.manager.CleanupRoom(room)
}

func (h *Handler) readLoop(room *rooms.Room, participant *rooms.Participant) {
	conn := participant.Connection()
	if conn == nil {
		return
	}
	defer participant.Close()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var inbound protocol.InboundEnvelope
		if err := json.Unmarshal(data, &inbound); err != nil {
			continue
		}
		switch inbound.Kind {
		case "CONTROL":
			if !participant.IsHost {
				participant.Send(protocol.Envelope{
					Kind: "ERROR",
					Data: protocol.ErrorPayload{
						Code:    "unauthorized",
						Message: "only host can control playback",
					},
				})
				continue
			}
			var control protocol.ControlMessage
			if err := json.Unmarshal(inbound.Data, &control); err != nil {
				continue
			}
			if control.Payload.IssuedAt.IsZero() {
				control.Payload.IssuedAt = time.Now().UTC()
			}
			state, err := room.ApplyControl(participant.ID, control)
			if err != nil {
				participant.Send(protocol.Envelope{
					Kind: "ERROR",
					Data: protocol.ErrorPayload{
						Code:    "control_failed",
						Message: err.Error(),
					},
				})
				continue
			}
			room.Broadcast(protocol.Envelope{
				Kind: "ROOM_STATE",
				Data: protocol.RoomStatePayload{Room: state},
			})
		case "SYNC_REQUEST":
			var req protocol.SyncRequest
			if err := json.Unmarshal(inbound.Data, &req); err != nil {
				continue
			}
			_ = req
			participant.Send(protocol.Envelope{
				Kind: "ROOM_STATE",
				Data: protocol.RoomStatePayload{Room: room.StateSnapshot()},
			})
		default:
			participant.Send(protocol.Envelope{
				Kind: "ERROR",
				Data: protocol.ErrorPayload{
					Code:    "unknown_kind",
					Message: "unsupported message type",
				},
			})
		}
	}
}

func extractRoomID(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "ws" || parts[1] != "rooms" {
		return "", errors.New("invalid path")
	}
	return parts[2], nil
}
