import { useEffect, useRef, useState } from "react";
import { DeviceMotion } from "expo-sensors";

export interface Orientation {
  roll: number; // degrees (-90 to 90)
  pitch: number; // degrees (-180 to 180)
  yaw: number; // degrees (0 to 360)
}

const RAD_TO_DEG = 180 / Math.PI;

/**
 * Returns the current device orientation in degrees using DeviceMotion sensor fusion.
 * roll = gamma axis (left/right tilt)
 * pitch = beta axis (forward/back tilt)
 * yaw = alpha axis (compass heading)
 */
export function useGyroscope(updateIntervalMs = 100): Orientation {
  const [orientation, setOrientation] = useState<Orientation>({
    roll: 0,
    pitch: 0,
    yaw: 0,
  });

  useEffect(() => {
    DeviceMotion.setUpdateInterval(updateIntervalMs);

    const subscription = DeviceMotion.addListener((measurement) => {
      const { rotation } = measurement;
      setOrientation({
        roll: rotation.gamma * RAD_TO_DEG,
        pitch: rotation.beta * RAD_TO_DEG,
        yaw: rotation.alpha * RAD_TO_DEG,
      });
    });

    return () => subscription.remove();
  }, [updateIntervalMs]);

  return orientation;
}

/**
 * Euclidean angular distance between two orientations in degrees.
 * Handles yaw wrap-around at 360°.
 */
export function angularDistance(a: Orientation, b: Orientation): number {
  const dRoll = a.roll - b.roll;
  const dPitch = a.pitch - b.pitch;
  let dYaw = a.yaw - b.yaw;
  if (dYaw > 180) dYaw -= 360;
  if (dYaw < -180) dYaw += 360;
  return Math.sqrt(dRoll * dRoll + dPitch * dPitch + dYaw * dYaw);
}
