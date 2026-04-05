/**
 * useProximity — orchestrates the GPS + PIN + photo state for check-in and
 * check-out handoff flows. Combines continuous location tracking with proximity
 * API calls (POST /proximity/verify, POST /proximity/pin) and local photo
 * capture state.
 *
 * ProofType "CHECK_IN" is used at check-in; "CHECK_OUT" is used at check-out.
 */
import { useState, useCallback, useEffect } from "react";
import * as Location from "expo-location";
import { api } from "../api";
import type { CapturedPhoto } from "../../components/camera/AngleEnforcedCamera";

export type ProofType = "CHECK_IN" | "CHECK_OUT";

export interface ProximityState {
  // GPS
  gpsVerified: boolean;
  gpsVerifying: boolean;
  gpsError: string | null;
  currentLat: number | null;
  currentLng: number | null;

  // PIN (check-in only — renter enters PIN from host)
  pinVerified: boolean;
  pinVerifying: boolean;
  pinError: string | null;

  // Photos
  photos: CapturedPhoto[];

  // Actions
  verifyGPS: () => Promise<void>;
  verifyPIN: (pin: string) => Promise<void>;
  addPhoto: (photo: CapturedPhoto) => void;
  removePhoto: (index: number) => void;

  // Derived
  canComplete: boolean;
}

const MIN_PHOTOS = 3;

export function useProximity(
  transactionId: string,
  proofType: ProofType,
  isRenter: boolean,
): ProximityState {
  const [gpsVerified, setGpsVerified] = useState(false);
  const [gpsVerifying, setGpsVerifying] = useState(false);
  const [gpsError, setGpsError] = useState<string | null>(null);
  const [currentLat, setCurrentLat] = useState<number | null>(null);
  const [currentLng, setCurrentLng] = useState<number | null>(null);

  const [pinVerified, setPinVerified] = useState(false);
  const [pinVerifying, setPinVerifying] = useState(false);
  const [pinError, setPinError] = useState<string | null>(null);

  const [photos, setPhotos] = useState<CapturedPhoto[]>([]);

  // Continuously track location so the GPS status stays fresh.
  useEffect(() => {
    let sub: Location.LocationSubscription | null = null;

    (async () => {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== "granted") return;

      sub = await Location.watchPositionAsync(
        { accuracy: Location.Accuracy.High, distanceInterval: 5 },
        (pos) => {
          setCurrentLat(pos.coords.latitude);
          setCurrentLng(pos.coords.longitude);
        },
      );
    })();

    return () => {
      sub?.remove();
    };
  }, []);

  const verifyGPS = useCallback(async () => {
    if (currentLat === null || currentLng === null) {
      setGpsError("Waiting for GPS signal…");
      return;
    }

    setGpsVerifying(true);
    setGpsError(null);

    try {
      await api
        .post("api/v1/proximity/verify", {
          json: {
            transactionId,
            lat: currentLat,
            lng: currentLng,
            proofType,
          },
        })
        .json<{ verified: boolean }>();
      setGpsVerified(true);
    } catch {
      setGpsError("Location is not within required proximity (≤100 m). Move closer and try again.");
    } finally {
      setGpsVerifying(false);
    }
  }, [transactionId, proofType, currentLat, currentLng]);

  const verifyPIN = useCallback(
    async (pin: string) => {
      setPinVerifying(true);
      setPinError(null);

      try {
        await api
          .post("api/v1/proximity/pin", {
            json: { transactionId, pin },
          })
          .json<{ verified: boolean }>();
        setPinVerified(true);
      } catch {
        setPinError("Incorrect or expired PIN. Ask the host to resend it.");
      } finally {
        setPinVerifying(false);
      }
    },
    [transactionId],
  );

  const addPhoto = useCallback((photo: CapturedPhoto) => {
    setPhotos((prev) => [...prev, photo]);
  }, []);

  const removePhoto = useCallback((index: number) => {
    setPhotos((prev) => prev.filter((_, i) => i !== index));
  }, []);

  // Check-in: renter needs GPS + PIN + photos; host needs GPS + photos.
  // Check-out: both parties need GPS + photos.
  const canComplete =
    proofType === "CHECK_IN" && isRenter
      ? gpsVerified && pinVerified && photos.length >= MIN_PHOTOS
      : gpsVerified && photos.length >= MIN_PHOTOS;

  return {
    gpsVerified,
    gpsVerifying,
    gpsError,
    currentLat,
    currentLng,
    pinVerified,
    pinVerifying,
    pinError,
    photos,
    verifyGPS,
    verifyPIN,
    addPhoto,
    removePhoto,
    canComplete,
  };
}
