/// <reference types="vite/client" />

declare global {
  interface ImportMetaEnv {
    readonly VITEST?: boolean;
    readonly VITE_PUBLIC_RELEASE_TRACK?: 'v5' | 'v6';
  }
}

export {};
