# wethu 同步播放器

一个支持房主控制、多人多端同步播放远程视频的演示项目。前端使用 Vite + React + TypeScript，后端使用 Go 实现 REST API 与 WebSocket 同步服务。

## 目录结构

- `web/` 前端项目（Vite）
- `server/` 后端服务（Go）

## 开发环境准备

```bash
# 前端依赖
cd web
npm install
npm run dev

# 另起终端启动后端
cd server
go mod tidy
go run ./cmd/server
```

默认情况下：

- 前端开发服务器运行在 `http://localhost:5173`
- 后端监听 `http://localhost:8080`，WebSocket 为 `ws://localhost:8080/ws/rooms/:roomId`

## 功能概述

- 房主创建房间并输入视频地址，其他用户可通过房间号加入
- 房主通过浏览器的 `<video>` 控件控制播放/暂停/拖动，状态经 WebSocket 广播给房间内所有用户
- 观众自动校准播放进度，保持与房主同步
- 加入房间时先通过 REST API 拉取当前状态，随后通过 WebSocket 持续同步

## 待办方向

- 增加房间/成员持久化与清理策略
- 支持断线重连、自动重试与客户端重同步
- 引入鉴权机制（例如房主口令或登录）
- 扩展消息类型（聊天、播放列表等）

