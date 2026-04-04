import { create } from "zustand";

interface CheckoutStore {
  scheduledStart: Date | null;
  scheduledEnd: Date | null;
  paymentMethodId: string | null;
  holdAmount: number; // cents
  rentalFee: number; // cents
  totalImpact: number; // cents (holdAmount + rentalFee)
  setSchedule: (start: Date, end: Date) => void;
  setPaymentMethod: (id: string) => void;
  setAmounts: (holdAmount: number, rentalFee: number) => void;
  reset: () => void;
}

export const useCheckoutStore = create<CheckoutStore>((set) => ({
  scheduledStart: null,
  scheduledEnd: null,
  paymentMethodId: null,
  holdAmount: 0,
  rentalFee: 0,
  totalImpact: 0,
  setSchedule: (start, end) => set({ scheduledStart: start, scheduledEnd: end }),
  setPaymentMethod: (id) => set({ paymentMethodId: id }),
  setAmounts: (holdAmount, rentalFee) =>
    set({ holdAmount, rentalFee, totalImpact: holdAmount + rentalFee }),
  reset: () =>
    set({
      scheduledStart: null,
      scheduledEnd: null,
      paymentMethodId: null,
      holdAmount: 0,
      rentalFee: 0,
      totalImpact: 0,
    }),
}));
