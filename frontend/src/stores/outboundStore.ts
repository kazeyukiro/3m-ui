import { create } from 'zustand';
import { fetchOutbounds, saveOutbound, deleteOutbound, setOutboundsEnabled, testOutboundConnection, type ServerOutbound } from '../api/outbounds';
import type { ConnectionCheck } from '../api/listenerRuntime';
import { generateConfig, applyConfigYAML } from '../api/config';

interface OutboundState {
  entries: ServerOutbound[];
  loading: boolean;
  saving: boolean;
  applying: boolean;
  testing: string | null;
  results: Record<string, ConnectionCheck>;
  test: (name: string) => Promise<void>;
  load: () => Promise<void>;
  save: (entry: ServerOutbound, previous?: string) => Promise<void>;
  remove: (name: string) => Promise<void>;
  setEnabled: (names: string[], enabled: boolean) => Promise<void>;
  apply: () => Promise<void>;
}

export const useOutboundStore = create<OutboundState>((set) => ({
  entries: [], loading: false, saving: false, applying: false,
  testing: null, results: {},
  test: async name => {
    set(state => ({ testing: name, results: Object.fromEntries(Object.entries(state.results).filter(([key]) => key !== name)) }));
    try {
      const result = await testOutboundConnection(name);
      set(state => ({ results: { ...state.results, [name]: result } }));
    } finally { set({ testing: null }); }
  },
  load: async () => {
    set({ loading: true });
    try { set({ entries: await fetchOutbounds(), results: {} }); } finally { set({ loading: false }); }
  },
  save: async (entry, previous) => {
    set({ saving: true });
    try {
      const saved = await saveOutbound(entry, previous);
      set({ results: {} });
      set(state => ({ entries: previous ? state.entries.map(e => e.proxy.name === previous ? saved : e.proxy['dialer-proxy'] === previous ? { ...e, proxy: { ...e.proxy, 'dialer-proxy': saved.proxy.name } } : e) : [...state.entries, saved] }));
    } finally { set({ saving: false }); }
  },
  remove: async name => {
    set({ saving: true });
    try { await deleteOutbound(name); set(state => ({ entries: state.entries.filter(e => e.proxy.name !== name), results: {} })); } finally { set({ saving: false }); }
  },
  setEnabled: async (names, enabled) => {
    set({ saving: true });
    try { await setOutboundsEnabled(names, enabled); set(state => ({ entries: state.entries.map(e => names.includes(e.proxy.name) ? { ...e, enabled } : e), results: {} })); } finally { set({ saving: false }); }
  },
  apply: async () => {
    set({ applying: true });
    try {
      const generated = await generateConfig();
      if (!generated.config) throw new Error(generated.message || 'Configuration generation returned no YAML');
      await applyConfigYAML(generated.config);
    } finally { set({ applying: false }); }
  },
}));
