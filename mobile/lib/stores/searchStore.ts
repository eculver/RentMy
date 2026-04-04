import { create } from "zustand";
import { SearchFilters } from "../hooks/useDiscovery";

interface SearchStore {
  query: string;
  filters: SearchFilters;
  setQuery: (query: string) => void;
  setFilters: (filters: SearchFilters) => void;
  resetFilters: () => void;
}

const defaultFilters: SearchFilters = {};

export const useSearchStore = create<SearchStore>((set) => ({
  query: "",
  filters: defaultFilters,
  setQuery: (query) => set({ query }),
  setFilters: (filters) => set({ filters }),
  resetFilters: () => set({ filters: defaultFilters }),
}));
