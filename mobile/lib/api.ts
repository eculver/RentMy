import ky from "ky";
import { useAuthStore } from "./auth";

const API_URL = process.env.EXPO_PUBLIC_API_URL || "http://localhost:8080";

export const api = ky.create({
  prefixUrl: API_URL,
  timeout: 10000,
  retry: { limit: 2, methods: ["get"] },
  hooks: {
    beforeRequest: [
      (request) => {
        const token = useAuthStore.getState().token;
        if (token) {
          request.headers.set("Authorization", `Bearer ${token}`);
        }
      },
    ],
    afterResponse: [
      async (_request, _options, response) => {
        if (response.status === 401) {
          const refreshed = await useAuthStore.getState().refreshTokens();
          if (!refreshed) {
            await useAuthStore.getState().logout();
          }
        }
      },
    ],
  },
});
