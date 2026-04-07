import { create } from "zustand";
import * as SecureStore from "expo-secure-store";
import ky from "ky";

const API_URL = process.env.EXPO_PUBLIC_API_URL || "http://localhost:8080";

// Minimal ky instance for auth calls — no auth header needed
const authApi = ky.create({ prefixUrl: API_URL, timeout: 10000, retry: 0 });

export type IdentityStatus = "PENDING" | "VERIFIED" | "REJECTED" | "ESCALATED";

export interface User {
  id: string;
  name: string;
  email: string;
  identityStatus?: IdentityStatus;
}

interface AuthResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
}

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (accessToken: string, refreshToken: string, user: User) => Promise<void>;
  logout: () => Promise<void>;
  loadToken: () => Promise<void>;
  loginWithCredentials: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string, referralCode?: string) => Promise<void>;
  refreshTokens: () => Promise<boolean>;
  setIdentityStatus: (status: IdentityStatus) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  refreshToken: null,
  user: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (accessToken: string, refreshToken: string, user: User) => {
    await SecureStore.setItemAsync("auth_token", accessToken);
    await SecureStore.setItemAsync("auth_refresh_token", refreshToken);
    await SecureStore.setItemAsync("auth_user", JSON.stringify(user));
    set({ token: accessToken, refreshToken, user, isAuthenticated: true });
  },

  logout: async () => {
    await SecureStore.deleteItemAsync("auth_token");
    await SecureStore.deleteItemAsync("auth_refresh_token");
    await SecureStore.deleteItemAsync("auth_user");
    set({ token: null, refreshToken: null, user: null, isAuthenticated: false });
  },

  loadToken: async () => {
    try {
      const token = await SecureStore.getItemAsync("auth_token");
      const refreshToken = await SecureStore.getItemAsync("auth_refresh_token");
      const userStr = await SecureStore.getItemAsync("auth_user");
      if (token && userStr) {
        const user = JSON.parse(userStr) as User;
        set({ token, refreshToken, user, isAuthenticated: true, isLoading: false });
      } else {
        set({ isLoading: false });
      }
    } catch {
      set({ isLoading: false });
    }
  },

  loginWithCredentials: async (email: string, password: string) => {
    const data = await authApi
      .post("api/v1/auth/login", { json: { email, password } })
      .json<AuthResponse>();
    await get().login(data.accessToken, data.refreshToken, data.user);
  },

  register: async (name: string, email: string, password: string, referralCode?: string) => {
    const body: Record<string, unknown> = { name, email, password };
    if (referralCode) body.referralCode = referralCode;
    const data = await authApi
      .post("api/v1/auth/register", { json: body })
      .json<AuthResponse>();
    await get().login(data.accessToken, data.refreshToken, data.user);
  },

  setIdentityStatus: (status: IdentityStatus) => {
    const { user } = get();
    if (!user) return;
    const updated = { ...user, identityStatus: status };
    void SecureStore.setItemAsync("auth_user", JSON.stringify(updated));
    set({ user: updated });
  },

  refreshTokens: async (): Promise<boolean> => {
    const { refreshToken } = get();
    if (!refreshToken) return false;
    try {
      const data = await authApi
        .post("api/v1/auth/refresh", { json: { refreshToken } })
        .json<AuthResponse>();
      await SecureStore.setItemAsync("auth_token", data.accessToken);
      await SecureStore.setItemAsync("auth_refresh_token", data.refreshToken);
      await SecureStore.setItemAsync("auth_user", JSON.stringify(data.user));
      set({ token: data.accessToken, refreshToken: data.refreshToken, user: data.user });
      return true;
    } catch {
      return false;
    }
  },
}));
