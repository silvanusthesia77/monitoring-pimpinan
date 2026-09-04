import { api } from "./api.js";
import { el, toInputTime } from "./dom.js";
import { clearSessionState, selectedAgenda, setDashboardData, state } from "./state.js";
import { render, showApp, showLogin } from "./render.js";

export function bindEvents() {
  el("loginForm").addEventListener("submit", login);
  el("logoutBtn").addEventListener("click", logout);
  el("togglePassword").addEventListener("click", togglePassword);
  el("agendaForm").addEventListener("submit", createAgenda);
  el("validationForm").addEventListener("submit", validateAgenda);
  el("pullbackBtn").addEventListener("click", pullBackAgenda);
  el("reportForm").addEventListener("submit", uploadDocumentation);
}

export async function bootstrap() {
  try {
    state.user = await api("/api/me");
    await refresh();
    showApp();
  } catch {
    showLogin();
  }
}

export function setDefaultDates() {
  const start = new Date();
  start.setDate(start.getDate() + 7);
  start.setHours(10, 0, 0, 0);

  const end = new Date(start);
  end.setHours(12, 0, 0, 0);

  document.querySelector("[name='start_at']").value = toInputTime(start);
  document.querySelector("[name='end_at']").value = toInputTime(end);
}

async function login(event) {
  event.preventDefault();
  el("loginError").classList.add("hidden");

  try {
    state.user = await api("/api/login", {
      method: "POST",
      body: JSON.stringify({
        email: el("email").value,
        password: el("password").value,
      }),
      headers: { "Content-Type": "application/json" },
    });
    await refresh();
    showApp();
  } catch (error) {
    el("loginError").textContent = error.message;
    el("loginError").classList.remove("hidden");
  }
}

async function logout() {
  await api("/api/logout", { method: "POST" });
  clearSessionState();
  showLogin();
}

async function refresh() {
  const [agendas, notifications] = await Promise.all([
    api("/api/agendas"),
    api("/api/notifications"),
  ]);
  setDashboardData(agendas, notifications);
  render();
}

async function createAgenda(event) {
  event.preventDefault();
  await api("/api/agendas", { method: "POST", body: new FormData(event.target) });
  event.target.reset();
  setDefaultDates();
  await refresh();
}

async function validateAgenda(event) {
  event.preventDefault();
  const agenda = selectedAgenda();
  if (!agenda) return;

  const form = new FormData(event.target);
  await api(`/api/agendas/${agenda.id}/validation`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      status: form.get("status"),
      delegate: form.get("delegate"),
      leader_note: form.get("leader_note"),
    }),
  });
  await refresh();
}

async function pullBackAgenda() {
  const agenda = selectedAgenda();
  if (!agenda) return;

  try {
    await api(`/api/agendas/${agenda.id}/pullback`, { method: "POST" });
    await refresh();
  } catch (error) {
    alert(error.message);
  }
}

async function uploadDocumentation(event) {
  event.preventDefault();
  const agenda = selectedAgenda();
  if (!agenda) return;

  await api(`/api/agendas/${agenda.id}/documentation`, {
    method: "POST",
    body: new FormData(event.target),
  });
  event.target.reset();
  await refresh();
}

function togglePassword() {
  const input = el("password");
  const button = el("togglePassword");
  const isHidden = input.type === "password";

  input.type = isHidden ? "text" : "password";
  button.setAttribute("aria-label", isHidden ? "Sembunyikan password" : "Lihat password");
  button.setAttribute("title", isHidden ? "Sembunyikan password" : "Lihat password");
  button.classList.toggle("is-visible", isHidden);
}
