import { FormEvent, useState } from 'react';
import { createRoom, joinRoom } from '@/api/client';
import { RoomSession } from '@/types/session';

interface LobbyProps {
  onSessionReady: (session: RoomSession) => void;
}

function Lobby({ onSessionReady }: LobbyProps) {
  const [displayName, setDisplayName] = useState('');
  const [roomId, setRoomId] = useState('');
  const [videoUrl, setVideoUrl] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!displayName || !videoUrl) {
      setError('请填写昵称和视频地址');
      return;
    }
    setError(null);
    setIsSubmitting(true);
    try {
      const session = await createRoom(displayName.trim(), videoUrl.trim());
      onSessionReady(session);
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建房间失败');
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleJoin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!displayName || !roomId) {
      setError('请填写昵称和房间号');
      return;
    }
    setError(null);
    setIsSubmitting(true);
    try {
      const session = await joinRoom(roomId.trim(), displayName.trim());
      onSessionReady(session);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加入房间失败');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="container">
      <header>
        <h1>wethu 同步播放器</h1>
        <p>创建或加入房间，与朋友一起同步观看视频。</p>
      </header>
      <section className="card">
        <h2>创建房间</h2>
        <form onSubmit={handleCreate}>
          <label>
            昵称
            <input
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              placeholder="请输入昵称"
            />
          </label>
          <label>
            视频地址
            <input
              value={videoUrl}
              onChange={(event) => setVideoUrl(event.target.value)}
              placeholder="支持 MP4/HLS 等公开链接"
            />
          </label>
          <button type="submit" disabled={isSubmitting}>
            {isSubmitting ? '创建中...' : '创建房间'}
          </button>
        </form>
      </section>
      <section className="card">
        <h2>加入房间</h2>
        <form onSubmit={handleJoin}>
          <label>
            昵称
            <input
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              placeholder="请输入昵称"
            />
          </label>
          <label>
            房间号
            <input
              value={roomId}
              onChange={(event) => setRoomId(event.target.value)}
              placeholder="请输入房间号"
            />
          </label>
          <button type="submit" disabled={isSubmitting}>
            {isSubmitting ? '加入中...' : '加入房间'}
          </button>
        </form>
      </section>
      {error ? <p className="error">{error}</p> : null}
    </div>
  );
}

export default Lobby;

