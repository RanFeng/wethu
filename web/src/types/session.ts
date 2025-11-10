import { RoomState } from './state';

export interface RoomSession {
  roomId: string;
  userId: string;
  displayName: string;
  isHost: boolean;
  token: string;
  initialState: RoomState;
  videoUrl: string;
}

