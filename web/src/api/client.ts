import { RoomSession } from '@/types/session';
import { RoomState } from '@/types/state';

const API_BASE = '/api';

interface CreateRoomPayload {
  displayName: string;
  videoUrl: string;
}

interface JoinRoomPayload {
  displayName: string;
}

interface SessionResponse {
  roomId: string;
  userId: string;
  token: string;
  isHost: boolean;
  state: RoomState;
}

async function request<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {})
    },
    ...init
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || response.statusText);
  }

  return response.json() as Promise<T>;
}

export async function createRoom(displayName: string, videoUrl: string): Promise<RoomSession> {
  const payload: CreateRoomPayload = { displayName, videoUrl };
  const data = await request<SessionResponse>(`${API_BASE}/rooms/create`, {
    method: 'POST',
    body: JSON.stringify(payload)
  });
  return {
    roomId: data.roomId,
    userId: data.userId,
    displayName,
    isHost: data.isHost,
    token: data.token,
    initialState: data.state,
    videoUrl
  };
}

export async function joinRoom(roomId: string, displayName: string): Promise<RoomSession> {
  const payload: JoinRoomPayload = { displayName };
  const data = await request<SessionResponse>(`${API_BASE}/rooms/join/${roomId}`, {
    method: 'POST',
    body: JSON.stringify(payload)
  });
  return {
    roomId: data.roomId,
    userId: data.userId,
    displayName,
    isHost: data.isHost,
    token: data.token,
    initialState: data.state,
    videoUrl: data.state.videoUrl
  };
}

export async function fetchRoomState(roomId: string): Promise<RoomState> {
  return request<RoomState>(`${API_BASE}/rooms/${roomId}`);
}

