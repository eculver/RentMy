import { useEffect, useRef } from "react";
import { useAuthStore } from "../auth";

const PUSHER_KEY = process.env.EXPO_PUBLIC_PUSHER_KEY ?? "app-key";
const PUSHER_HOST = process.env.EXPO_PUBLIC_PUSHER_HOST ?? "127.0.0.1";
const PUSHER_PORT = Number(process.env.EXPO_PUBLIC_PUSHER_PORT ?? "6001");
const API_URL = process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080";

/**
 * Subscribes to a Pusher channel and binds a single event handler.
 * Automatically unsubscribes and disconnects on unmount.
 *
 * Channel auth uses `POST /api/v1/pusher/auth` with the user's JWT.
 */
export function usePusher(
  channelName: string | null,
  eventName: string,
  onEvent: (data: unknown) => void,
): void {
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (!channelName) return;

    let cancelled = false;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let pusher: any = null;

    // Dynamic import keeps Pusher out of the critical bundle path.
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const PusherCtor = require("pusher-js/react-native").default as new (
      key: string,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      options: Record<string, any>,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ) => any;

    if (cancelled) return;

    const token = useAuthStore.getState().token;

    pusher = new PusherCtor(PUSHER_KEY, {
      wsHost: PUSHER_HOST,
      wsPort: PUSHER_PORT,
      wssPort: PUSHER_PORT,
      forceTLS: false,
      disableStats: true,
      enabledTransports: ["ws"],
      channelAuthorization: {
        endpoint: `${API_URL}/api/v1/pusher/auth`,
        transport: "ajax",
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      },
    });

    const channel = pusher.subscribe(channelName);
    channel.bind(eventName, (data: unknown) => {
      onEventRef.current(data);
    });

    return () => {
      cancelled = true;
      if (pusher) {
        channel.unbind(eventName);
        pusher.unsubscribe(channelName);
        pusher.disconnect();
      }
    };
  }, [channelName, eventName]); // eslint-disable-line react-hooks/exhaustive-deps
}
