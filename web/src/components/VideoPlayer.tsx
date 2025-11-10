import { ForwardedRef, forwardRef } from 'react';

interface VideoPlayerProps {
  src: string;
  isHost: boolean;
  onPlay: () => void;
  onPause: () => void;
  onSeeked: () => void;
  onLoadedMetadata: () => void;
}

function VideoPlayerComponent(
  { src, isHost, onPlay, onPause, onSeeked, onLoadedMetadata }: VideoPlayerProps,
  ref: ForwardedRef<HTMLVideoElement>
) {
  return (
    <div className="video-wrapper">
      <video
        ref={ref}
        src={src}
        controls
        playsInline
        preload="auto"
        onPlay={onPlay}
        onPause={onPause}
        onSeeked={onSeeked}
        onLoadedMetadata={onLoadedMetadata}
      />
      <div className="role-indicator">{isHost ? '房主控制' : '观众同步'}</div>
    </div>
  );
}

const VideoPlayer = forwardRef(VideoPlayerComponent);
export default VideoPlayer;

