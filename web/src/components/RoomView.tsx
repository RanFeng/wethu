import { useEffect, useMemo, useRef } from 'react';
import { useRoomConnection } from '@/hooks/useRoomConnection';
import { RoomSession } from '@/types/session';
import VideoPlayer from '@/components/VideoPlayer';

interface RoomViewProps {
  session: RoomSession;
  onLeave: () => void;
}

function RoomView({ session, onLeave }: RoomViewProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const { roomState, status, error, updateWithControl, sendSeek, requestSync } = useRoomConnection(session);
  const statusText = useMemo(() => {
    switch (status) {
      case 'connecting':
        return '正在连接服务器...';
      case 'open':
        return '连接正常';
      case 'closed':
        return '连接已关闭';
      case 'error':
        return '连接异常';
      default:
        return '未知状态';
    }
  }, [status]);

  useEffect(() => {
    if (session.isHost) {
      return;
    }
    const video = videoRef.current;
    if (!video) {
      return;
    }

    const adjustPlayback = () => {
      const diff = Math.abs(video.currentTime - roomState.position);
      if (diff > 0.4) {
        video.currentTime = roomState.position;
      }
      if (roomState.isPlaying && video.paused) {
        void video.play();
      }
      if (!roomState.isPlaying && !video.paused) {
        video.pause();
      }
    };

    adjustPlayback();
  }, [roomState, session.isHost]);

  useEffect(() => {
    if (!session.isHost) {
      return;
    }
    const video = videoRef.current;
    if (!video) {
      return;
    }
    const interval = window.setInterval(() => {
      if (!video.paused) {
        updateWithControl(video.currentTime, true);
      }
    }, 2000);

    return () => window.clearInterval(interval);
  }, [session.isHost, updateWithControl]);

  useEffect(() => {
    requestSync();
  }, [requestSync]);

  const handlePlay = () => {
    if (session.isHost) {
      const position = videoRef.current?.currentTime ?? 0;
      updateWithControl(position, true);
    }
  };

  const handlePause = () => {
    if (session.isHost) {
      const position = videoRef.current?.currentTime ?? 0;
      updateWithControl(position, false);
    }
  };

  const handleSeeked = () => {
    if (!session.isHost) {
      return;
    }
    const position = videoRef.current?.currentTime ?? 0;
    sendSeek(position);
    updateWithControl(position, !videoRef.current?.paused);
  };

  const handleLoadedMetadata = () => {
    if (!session.isHost) {
      requestSync();
    }
  };

  return (
    <div className="container">
      <header className="room-header">
        <div>
          <h1>房间：{session.roomId}</h1>
          <p>
            当前状态：{statusText}
            {error ? `，错误：${error}` : ''}
          </p>
        </div>
        <button onClick={onLeave}>离开房间</button>
      </header>

      <VideoPlayer
        ref={videoRef}
        src={roomState.videoUrl}
        isHost={session.isHost}
        onPlay={handlePlay}
        onPause={handlePause}
        onSeeked={handleSeeked}
        onLoadedMetadata={handleLoadedMetadata}
      />

      <section className="card">
        <h2>播放信息</h2>
        <p>视频地址：{roomState.videoUrl}</p>
        <p>播放进度：{roomState.position.toFixed(2)} 秒</p>
        <p>播放状态：{roomState.isPlaying ? '播放中' : '已暂停'}</p>
        <p>房主 ID：{roomState.ownerId}</p>
      </section>
    </div>
  );
}

export default RoomView;

