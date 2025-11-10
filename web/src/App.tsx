import { useState } from 'react';
import Lobby from '@/components/Lobby';
import RoomView from '@/components/RoomView';
import { RoomSession } from '@/types/session';

function App() {
  const [session, setSession] = useState<RoomSession | null>(null);

  if (session) {
    return <RoomView session={session} onLeave={() => setSession(null)} />;
  }

  return <Lobby onSessionReady={setSession} />;
}

export default App;

