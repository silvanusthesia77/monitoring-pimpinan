import { state } from "./state.js";

const routes = {
  "/": "dashboard",
  "/dashboard": "dashboard",
  "/users": "users",
  "/input-jadwal": "input",
  "/validasi": "validasi",
  "/agenda": "agenda",
  "/detail": "detail",
  "/dokumentasi": "dokumentasi",
  "/buat-laporan": "dokumentasi",
  "/laporan": "laporan",
  "/notifikasi": "notifikasi",
};

const viewPaths = Object.fromEntries(Object.entries(routes).map(([path, view]) => [view, path]));

export function viewFromPath(pathname = window.location.pathname) {
  return routes[pathname] ?? "dashboard";
}

export function pathForView(view) {
  return viewPaths[view] ?? "/dashboard";
}

export function syncViewFromPath() {
  state.activeView = viewFromPath();
}

export function navigateToView(view) {
  const path = pathForView(view);
  if (window.location.pathname !== path) {
    window.history.pushState({ view }, "", path);
  }
  state.activeView = view;
}
