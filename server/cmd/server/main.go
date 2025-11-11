package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"wethu/internal/rooms"
	"wethu/internal/hertzapi"
)

func main() {
	// 创建房间管理器
	roomManager := rooms.NewManager()
	
	// 创建Hertz服务器
	serverConfig := server.Default(server.WithHostPorts(":8080"))
	
	// 初始化API路由
	router := hertzapi.NewRouter(serverConfig, roomManager)
	
	// 启动服务器
	go func() {
		log.Println("Starting Hertz server on :8080")
	// 启动服务器
	router.Spin()
	}()

	// 优雅关闭
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := router.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v\n", err)
	}

	log.Println("Server stopped")
}
