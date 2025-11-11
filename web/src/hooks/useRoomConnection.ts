import { useCallback, useEffect, useRef, useState } from 'react';
import { InboundMessage, OutboundMessage, RoomState } from '@/types/state';
import { RoomSession } from '@/types/session';

type ConnectionStatus = 'connecting' | 'open' | 'closed' | 'error';

export function useRoomConnection(session: RoomSession) {
  const [roomState, setRoomState] = useState<RoomState>(session.initialState);
  const [status, setStatus] = useState<ConnectionStatus>('connecting');
  const [error, setError] = useState<string | null>(null);
  const socketRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    let reconnectTimeout: NodeJS.Timeout | null = null;
    let isMounted = true;

    const connect = () => {
      if (!isMounted) return;

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws/rooms/${session.roomId}?token=${encodeURIComponent(session.token)}`;
      console.log('Connecting to WebSocket:', wsUrl);
      console.log('Session info:', { roomId: session.roomId, userId: session.userId, token: session.token.substring(0, 8) + '...' });
      const socket = new WebSocket(wsUrl);
      socketRef.current = socket;

      socket.onopen = () => {
        console.log('Connecting WebSocket success!', wsUrl);
        if (!isMounted) {
          socket.close();
          return;
        }
        setStatus('open');
        setError(null);
        if (!session.isHost) {
          const syncPayload: OutboundMessage = {
            kind: 'SYNC_REQUEST',
            data: {
              roomId: session.roomId,
              senderId: session.userId
            }
          };
          socket.send(JSON.stringify(syncPayload));
        }
      };

      socket.onmessage = (event) => {
        if (!isMounted) return;
        try {
          const message: InboundMessage = JSON.parse(event.data);
          switch (message.kind) {
            case 'ROOM_STATE':
              setRoomState(message.data.room);
              break;
            case 'CONTROL':
              setRoomState((current) => {
                if (current.updatedAt >= message.data.payload.issuedAt) {
                  return current;
                }
                return {
                  ...current,
                  isPlaying: message.data.payload.isPlaying ?? current.isPlaying,
                  position: message.data.payload.position,
                  videoUrl: message.data.payload.videoUrl ?? current.videoUrl,
                  updatedAt: message.data.payload.issuedAt
                };
              });
              break;
            case 'ERROR':
              setError(message.data.message);
              break;
            default:
              break;
          }
        } catch (err) {
          setError(err instanceof Error ? err.message : '解析服务端消息失败');
        }
      };

      socket.onerror = (error) => {
        if (!isMounted) return;
        console.error('WebSocket error:', error);
        setStatus('error');
        // 提供更详细的错误信息
        setError(`WebSocket 连接异常: ${error instanceof Error ? error.message : String(error)}`);
      };

      socket.onclose = (event) => {
        if (!isMounted) return;
        setStatus('closed');
        console.log(`WebSocket disconnected: code=${event.code}, reason=${event.reason}, wasClean=${event.wasClean}`);

        // 如果是401错误，显示未授权错误信息
        if (event.code === 1008) {
          setError('连接被拒绝: 未授权访问，请检查token是否有效');
        } else if (event.code !== 1000) {
          setError(`连接意外断开: 错误代码 ${event.code}`);
          // Only attempt to reconnect if it wasn't a clean close and component is still mounted
          reconnectTimeout = setTimeout(() => {
            if (isMounted) {
              console.log('Attempting to reconnect...');
              setStatus('connecting');
              connect();
            }
          }, 3000);
        }
      };
    };

    connect();

    return () => {
      console.log("ccccccccc")
      isMounted = false;
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
      }
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
    };
  }, [session.isHost, session.roomId, session.token, session.userId]);

  const sendMessage = useCallback((message: OutboundMessage) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return;
    }
    socket.send(JSON.stringify(message));
  }, []);

  const updateWithControl = useCallback(
    (position: number, isPlaying: boolean, videoUrl?: string) => {
      const issuedAt = new Date().toISOString();
      sendMessage({
        kind: 'CONTROL',
        data: {
          type: isPlaying ? 'PLAY' : 'PAUSE',
          roomId: session.roomId,
          senderId: session.userId,
          payload: {
            position,
            isPlaying,
            videoUrl,
            issuedAt
          }
        }
      });
      setRoomState((current) => ({
        ...current,
        position,
        isPlaying,
        videoUrl: videoUrl ?? current.videoUrl,
        updatedAt: issuedAt
      }));
    },
    [sendMessage, session.roomId, session.userId]
  );

  const sendSeek = useCallback(
    (position: number) => {
      const issuedAt = new Date().toISOString();
      sendMessage({
        kind: 'CONTROL',
        data: {
          type: 'SEEK',
          roomId: session.roomId,
          senderId: session.userId,
          payload: {
            position,
            issuedAt
          }
        }
      });
    },
    [sendMessage, session.roomId, session.userId]
  );

  const requestSync = useCallback(() => {
    sendMessage({
      kind: 'SYNC_REQUEST',
      data: {
        roomId: session.roomId,
        senderId: session.userId
      }
    });
  }, [sendMessage, session.roomId, session.userId]);

  return {
    roomState,
    status,
    error,
    updateWithControl,
    sendSeek,
    requestSync
  };
}