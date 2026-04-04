import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import { User } from "../auth";

export function useUser() {
  return useQuery<User>({
    queryKey: ["user", "me"],
    queryFn: () => api.get("api/v1/users/me").json<{ user: User }>().then((r) => r.user),
  });
}
