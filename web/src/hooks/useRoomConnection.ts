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
      const socket = new WebSocket(wsUrl);
      socketRef.current = socket;

      socket.onopen = () => {
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
        setError('WebSocket 连接异常');
      };

      socket.onclose = (event) => {
        if (!isMounted) return;
        setStatus('closed');
        // Only attempt to reconnect if it wasn't a clean close and component is still mounted
        if (event.code !== 1000 && isMounted) {
          reconnectTimeout = setTimeout(() => {
            if (isMounted) {
              setStatus('connecting');
              connect();
            }
          }, 3000);
        }
      };
    };

    connect();

    return () => {
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

