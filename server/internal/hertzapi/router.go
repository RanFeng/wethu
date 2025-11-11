package hertzapi

import (
	"context"
	"github.com/RanFeng/ilog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"wethu/internal/hertzws"
	"wethu/internal/rooms"
)

// NewRouter 初始化Hertz路由
func NewRouter(h *server.Hertz, roomManager *rooms.Manager) *server.Hertz {
	// 创建WebSocket处理器
	wsHandler := hertzws.NewHandler(roomManager)

	// 注册中间件
	h.Use(recoveryMiddleware())
	h.Use(loggerMiddleware())

	// 健康检查接口
	h.GET("/healthz", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "ok")
	})

	// API路由组
	api := h.Group("/api")
	{
		// 房间相关接口
		roomsGroup := api.Group("/rooms")
		{
			roomsGroup.POST("/create", handleCreateRoom(roomManager))
			roomsGroup.POST("/join/:roomId", handleJoinRoom(roomManager))
			roomsGroup.GET("/:roomId", handleGetRoom(roomManager))
		}
	}

	// WebSocket路由
	h.GET("/ws/rooms/:roomId", wsHandler.HandleWebSocket)

	return h
}

// recoveryMiddleware 恢复中间件
func recoveryMiddleware() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				ctx.String(consts.StatusInternalServerError, "Internal Server Error")
			}
		}()
		ctx.Next(c)
	}
}

// loggerMiddleware 日志中间件
func loggerMiddleware() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		ctx.Next(c)
	}
}

// handleCreateRoom 创建房间处理函数
func handleCreateRoom(roomManager *rooms.Manager) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		var payload createRoomRequest
		if err := ctx.Bind(&payload); err != nil {
			respondError(ctx, consts.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}

		if payload.DisplayName == "" || payload.VideoURL == "" {
			respondError(ctx, consts.StatusBadRequest, "invalid_request", "DisplayName and videoUrl are required")
			return
		}

		session, err := roomManager.CreateRoom(payload.DisplayName, payload.VideoURL)
		ilog.EventInfo(c, "CreateRoom", "session", session)
		if err != nil {
			respondError(ctx, consts.StatusInternalServerError, "create_failed", err.Error())
			return
		}

		ctx.JSON(consts.StatusCreated, session)
	}
}

// handleJoinRoom 加入房间处理函数
func handleJoinRoom(roomManager *rooms.Manager) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		roomID := ctx.Param("roomId")
		var payload joinRoomRequest
		if err := ctx.Bind(&payload); err != nil {
			respondError(ctx, consts.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}

		if payload.DisplayName == "" {
			respondError(ctx, consts.StatusBadRequest, "invalid_request", "DisplayName is required")
			return
		}

		ilog.EventInfo(c, "JoinRoom_start", "request", payload)

		session, err := roomManager.JoinRoom(roomID, payload.DisplayName)
		if err != nil {
			if err == rooms.ErrRoomNotFound {
				respondError(ctx, consts.StatusNotFound, "room_not_found", err.Error())
				return
			}
			respondError(ctx, consts.StatusInternalServerError, "join_failed", err.Error())
			return
		}
		ilog.EventInfo(c, "JoinRoom_end", "session", session)

		ctx.JSON(consts.StatusOK, session)
	}
}

// handleGetRoom 获取房间状态处理函数
func handleGetRoom(roomManager *rooms.Manager) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		roomID := ctx.Param("roomId")
		state, err := roomManager.GetState(roomID)
		if err != nil {
			if err == rooms.ErrRoomNotFound {
				respondError(ctx, consts.StatusNotFound, "room_not_found", err.Error())
				return
			}
			respondError(ctx, consts.StatusInternalServerError, "state_fetch_failed", err.Error())
			return
		}

		ctx.JSON(consts.StatusOK, state)
	}
}

// 请求结构体定义
type createRoomRequest struct {
	DisplayName string `json:"displayName"`
	VideoURL    string `json:"videoUrl"`
}

type joinRoomRequest struct {
	DisplayName string `json:"displayName"`
}

// respondError 返回错误响应
func respondError(ctx *app.RequestContext, status int, code, message string) {
	ctx.JSON(status, map[string]interface{}{
		"kind": "ERROR",
		"data": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
