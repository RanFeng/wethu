package httpapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"wethu/internal/protocol"
	"wethu/internal/rooms"
	"wethu/internal/ws"
)

type Server struct {
	rooms  *rooms.Manager
	ws     *ws.Handler
	router *echo.Echo
}

type createRoomRequest struct {
	DisplayName string `json:"displayName"`
	VideoURL    string `json:"videoUrl"`
}

type joinRoomRequest struct {
	DisplayName string `json:"displayName"`
}

func NewServer(manager *rooms.Manager) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	server := &Server{
		rooms:  manager,
		ws:     ws.NewHandler(manager),
		router: e,
	}

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	e.POST("/api/rooms", server.handleCreateRoom)
	e.POST("/api/rooms/:roomId/join", server.handleJoinRoom)
	e.GET("/api/rooms/:roomId", server.handleGetRoom)
	e.GET("/ws/rooms/:roomId", server.handleWebSocket)

	return server
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) handleCreateRoom(c echo.Context) error {
	var payload createRoomRequest
	if err := c.Bind(&payload); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid_request", "invalid request body")
	}
	if payload.DisplayName == "" || payload.VideoURL == "" {
		return respondError(c, http.StatusBadRequest, "invalid_request", "displayName and videoUrl are required")
	}
	session, err := s.rooms.CreateRoom(payload.DisplayName, payload.VideoURL)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "create_failed", err.Error())
	}
	return c.JSON(http.StatusCreated, session)
}

func (s *Server) handleJoinRoom(c echo.Context) error {
	roomID := c.Param("roomId")
	var payload joinRoomRequest
	if err := c.Bind(&payload); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid_request", "invalid request body")
	}
	if payload.DisplayName == "" {
		return respondError(c, http.StatusBadRequest, "invalid_request", "displayName is required")
	}
	session, err := s.rooms.JoinRoom(roomID, payload.DisplayName)
	if err != nil {
		if errors.Is(err, rooms.ErrRoomNotFound) {
			return respondError(c, http.StatusNotFound, "room_not_found", err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "join_failed", err.Error())
	}
	return c.JSON(http.StatusOK, session)
}

func (s *Server) handleGetRoom(c echo.Context) error {
	roomID := c.Param("roomId")
	state, err := s.rooms.GetState(roomID)
	if err != nil {
		if errors.Is(err, rooms.ErrRoomNotFound) {
			return respondError(c, http.StatusNotFound, "room_not_found", err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "state_fetch_failed", err.Error())
	}
	return c.JSON(http.StatusOK, state)
}

func (s *Server) handleWebSocket(c echo.Context) error {
	roomID := c.Param("roomId")
	// Set the roomId in the URL path so the WebSocket handler can extract it
	c.Request().URL.Path = "/ws/rooms/" + roomID
	// WebSocket handler takes full control of the connection
	// Return nil to prevent Echo from writing additional response
	s.ws.ServeHTTP(c.Response(), c.Request())
	return nil
}

func respondError(c echo.Context, status int, code, message string) error {
	return c.JSON(status, protocol.Envelope{
		Kind: "ERROR",
		Data: protocol.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}
