export interface RoomState {
  roomId: string;
  videoUrl: string;
  isPlaying: boolean;
  position: number;
  ownerId: string;
  updatedAt: string;
}

export interface ControlMessage {
  type: 'PLAY' | 'PAUSE' | 'SEEK' | 'SYNC_REQUEST' | 'SYNC_STATE';
  roomId: string;
  senderId: string;
  payload: {
    position: number;
    videoUrl?: string;
    isPlaying?: boolean;
    issuedAt: string;
  };
}

export interface RoomStatePayload {
  room: RoomState;
}

export type InboundMessage =
  | {
      kind: 'ROOM_STATE';
      data: RoomStatePayload;
    }
  | {
      kind: 'CONTROL';
      data: ControlMessage;
    }
  | {
      kind: 'ERROR';
      data: {
        code: string;
        message: string;
      };
    };

export type OutboundMessage =
  | {
      kind: 'CONTROL';
      data: ControlMessage;
    }
  | {
      kind: 'SYNC_REQUEST';
      data: {
        roomId: string;
        senderId: string;
      };
    };

