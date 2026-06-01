/// <reference types="vite/client" />

// OIDC client config is fetched at runtime from the server (GET /api/auth/mode
// → `oidc`), not injected at build time — so there are no VITE_OIDC_* env vars
// to declare here. See lib/oidc.ts / lib/authMode.ts.
