import { create } from "zustand";

interface LocationStore {
  lat: number | null;
  lng: number | null;
  setLocation: (lat: number, lng: number) => void;
}

export const useLocationStore = create<LocationStore>((set) => ({
  lat: null,
  lng: null,
  setLocation: (lat, lng) => set({ lat, lng }),
}));
