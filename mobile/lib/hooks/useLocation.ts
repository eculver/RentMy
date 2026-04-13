import { useEffect, useState } from "react";
import * as Location from "expo-location";
import { useLocationStore } from "../stores/locationStore";

export interface LocationState {
  lat: number | null;
  lng: number | null;
  loading: boolean;
  error: string | null;
  retry: () => void;
}

export function useLocation(): LocationState {
  const { lat, lng, setLocation } = useLocationStore();
  const [loading, setLoading] = useState(lat === null);
  const [error, setError] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    // If we already have a cached position, skip the permission request
    if (lat !== null && lng !== null) {
      setLoading(false);
      return;
    }

    let cancelled = false;

    (async () => {
      setLoading(true);
      setError(null);
      try {
        const { status } = await Location.requestForegroundPermissionsAsync();
        if (status !== "granted") {
          if (!cancelled) setError("Location permission denied");
          return;
        }
        const pos = await Location.getCurrentPositionAsync({
          accuracy: Location.Accuracy.Balanced,
        });
        if (!cancelled) {
          setLocation(pos.coords.latitude, pos.coords.longitude);
        }
      } catch (e) {
        if (!cancelled) setError("Unable to retrieve location");
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [retryCount]); // eslint-disable-line react-hooks/exhaustive-deps

  const retry = () => setRetryCount((c) => c + 1);

  return { lat, lng, loading, error, retry };
}
