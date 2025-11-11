package rooms

import (
	"testing"
	"time"

	"wethu/internal/protocol"
)

// TestCreateRoom 测试创建房间功能
func TestCreateRoom(t *testing.T) {
	manager := NewManager()
	displayName := "Test User"
	videoURL := "https://example.com/video"

	session, err := manager.CreateRoom(displayName, videoURL)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	if session == nil {
		t.Fatal("session should not be nil")
	}

	if session.RoomID == "" {
		t.Error("RoomID should not be empty")
	}

	if session.UserID == "" {
		t.Error("UserID should not be empty")
	}

	if session.Token == "" {
		t.Error("Token should not be empty")
	}

	if !session.IsHost {
		t.Error("Creator should be host")
	}

	// 验证房间状态
	if session.State.RoomID != session.RoomID {
		t.Error("RoomID mismatch in state")
	}

	if session.State.VideoURL != videoURL {
		t.Errorf("VideoURL mismatch: expected %s, got %s", videoURL, session.State.VideoURL)
	}

	if session.State.IsPlaying {
		t.Error("New room should not be playing")
	}

	if session.State.Position != 0 {
		t.Error("New room position should be 0")
	}
}

// TestJoinRoom 测试加入房间功能
func TestJoinRoom(t *testing.T) {
	manager := NewManager()

	// 先创建一个房间
	creatorName := "Host"
	videoURL := "https://example.com/video"
	session, err := manager.CreateRoom(creatorName, videoURL)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// 再加入这个房间
	joinerName := "Participant"
	joinSession, err := manager.JoinRoom(session.RoomID, joinerName)
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	if joinSession == nil {
		t.Fatal("joinSession should not be nil")
	}

	if joinSession.RoomID != session.RoomID {
		t.Error("RoomID mismatch")
	}

	if joinSession.UserID == "" {
		t.Error("UserID should not be empty")
	}

	if joinSession.Token == "" {
		t.Error("Token should not be empty")
	}

	if joinSession.IsHost {
		t.Error("Joiner should not be host")
	}

	// 验证房间状态
	if joinSession.State.RoomID != session.RoomID {
		t.Error("RoomID mismatch in state")
	}

	if joinSession.State.VideoURL != videoURL {
		t.Errorf("VideoURL mismatch: expected %s, got %s", videoURL, joinSession.State.VideoURL)
	}
}

// TestJoinNonExistentRoom 测试加入不存在的房间
func TestJoinNonExistentRoom(t *testing.T) {
	manager := NewManager()
	_, err := manager.JoinRoom("non-existent-room", "User")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

// TestGetState 测试获取房间状态
func TestGetState(t *testing.T) {
	manager := NewManager()

	// 创建房间
	session, err := manager.CreateRoom("Host", "https://example.com/video")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// 获取状态
	state, err := manager.GetState(session.RoomID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.RoomID != session.RoomID {
		t.Error("RoomID mismatch")
	}

	if state.VideoURL != "https://example.com/video" {
		t.Errorf("VideoURL mismatch: expected %s, got %s", "https://example.com/video", state.VideoURL)
	}
}

// TestGetStateNonExistentRoom 测试获取不存在房间的状态
func TestGetStateNonExistentRoom(t *testing.T) {
	manager := NewManager()
	_, err := manager.GetState("non-existent-room")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

// TestLookupParticipant 测试查找参与者
func TestLookupParticipant(t *testing.T) {
	manager := NewManager()

	// 创建房间
	session, err := manager.CreateRoom("Host", "https://example.com/video")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// 查找参与者
	room, participant, err := manager.LookupParticipant(session.RoomID, session.Token)
	if err != nil {
		t.Fatalf("LookupParticipant failed: %v", err)
	}

	if room == nil {
		t.Fatal("room should not be nil")
	}

	if participant == nil {
		t.Fatal("participant should not be nil")
	}

	if participant.ID != session.UserID {
		t.Error("Participant ID mismatch")
	}

	if participant.IsHost != session.IsHost {
		t.Error("Host status mismatch")
	}
}

// TestLookupParticipantInvalidToken 测试使用无效token查找参与者
func TestLookupParticipantInvalidToken(t *testing.T) {
	manager := NewManager()

	// 创建房间
	session, err := manager.CreateRoom("Host", "https://example.com/video")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// 使用无效token查找
	_, _, err = manager.LookupParticipant(session.RoomID, "invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

// TestApplyControl 测试应用控制命令
func TestApplyControl(t *testing.T) {
	manager := NewManager()

	// 创建房间
	session, err := manager.CreateRoom("Host", "https://example.com/video1")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// 获取房间
	room, participant, err := manager.LookupParticipant(session.RoomID, session.Token)
	if err != nil {
		t.Fatalf("LookupParticipant failed: %v", err)
	}

	// 创建控制命令
	videoURL := "https://example.com/video2"
	playing := true
	position := 10.5
	issuedAt := time.Now().UTC()

	control := protocol.ControlMessage{
		Payload: protocol.ControlPayload{
			VideoURL:  &videoURL,
			Playing:   &playing,
			Position:  position,
			IssuedAt:  issuedAt,
		},
	}

	// 应用控制
	state, err := room.ApplyControl(participant.ID, control)
	if err != nil {
		t.Fatalf("ApplyControl failed: %v", err)
	}

	// 验证状态更新
	if state.VideoURL != videoURL {
		t.Errorf("VideoURL mismatch: expected %s, got %s", videoURL, state.VideoURL)
	}

	if state.IsPlaying != playing {
		t.Errorf("IsPlaying mismatch: expected %v, got %v", playing, state.IsPlaying)
	}

	if state.Position != position {
		t.Errorf("Position mismatch: expected %f, got %f", position, state.Position)
	}

	if !state.UpdatedAt.Equal(issuedAt) {
		t.Errorf("UpdatedAt mismatch: expected %v, got %v", issuedAt, state.UpdatedAt)
	}
}