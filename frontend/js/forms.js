import { api } from "./api.js";
import { el, toInputTime } from "./dom.js";
import { navigateToView, syncViewFromPath } from "./router.js";
import { clearSessionState, selectedAgenda, setDashboardData, state } from "./state.js";
import { render, showApp, showLogin } from "./render.js";

export function bindEvents() {
  el("loginForm").addEventListener("submit", login);
  el("registerForm").addEventListener("submit", register);
  el("role").addEventListener("change", fillDemoAccount);
  el("showLoginForm").addEventListener("click", () => showAuthForm("login"));
  el("showRegisterForm").addEventListener("click", () => showAuthForm("register"));
  el("logoutBtn").addEventListener("click", logout);
  el("togglePassword").addEventListener("click", togglePassword);
  el("toggleRegisterPassword").addEventListener("click", () => togglePasswordField("registerPassword", "toggleRegisterPassword"));
  el("agendaForm").addEventListener("submit", createAgenda);
  el("validationForm").addEventListener("submit", validateAgenda);
  el("validationAgendaId").addEventListener("change", selectAgendaFromControl);
  el("documentationAgendaId").addEventListener("change", selectAgendaFromControl);
  el("reportForm").addEventListener("submit", uploadDocumentation);
  document.addEventListener("click", deleteAgenda);
  window.addEventListener("popstate", () => {
    if (!state.user) {
      showLogin();
      return;
    }
    syncViewFromPath();
    render();
  });
}

export async function bootstrap() {
  if (window.location.pathname === "/login") {
    await api("/api/logout", { method: "POST" }).catch(() => {});
    clearSessionState();
    showLogin();
    return;
  }

  try {
    state.user = await api("/api/me");
    syncViewFromPath();
    await refresh({ alertNew: false });
    showApp();
    startNotificationPolling();
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
        role: el("role").value,
        email: el("email").value,
        password: el("password").value,
      }),
      headers: { "Content-Type": "application/json" },
    });
    await refresh({ alertNew: false });
    navigateToView("dashboard");
    showApp();
    startNotificationPolling();
  } catch (error) {
    el("loginError").textContent = error.message;
    el("loginError").classList.remove("hidden");
  }
}

async function register(event) {
  event.preventDefault();
  el("registerError").classList.add("hidden");

  try {
    state.user = await api("/api/register", {
      method: "POST",
      body: JSON.stringify({
        role: el("registerRole").value,
        name: el("registerName").value,
        position: el("registerPosition").value,
        email: el("registerEmail").value,
        password: el("registerPassword").value,
      }),
      headers: { "Content-Type": "application/json" },
    });
    event.target.reset();
    await refresh({ alertNew: false });
    navigateToView("dashboard");
    showApp();
    startNotificationPolling();
  } catch (error) {
    el("registerError").textContent = error.message;
    el("registerError").classList.remove("hidden");
  }
}

function fillDemoAccount() {
  const role = el("role").value;
  el("email").value = role === "pimpinan" ? "pimpinan@sorsel.go.id" : "staf@sorsel.go.id";
  el("password").value = "agenda123";
}

async function logout() {
  await api("/api/logout", { method: "POST" });
  clearSessionState();
  window.history.pushState({ view: "login" }, "", "/login");
  showLogin();
}

async function refresh({ alertNew = true } = {}) {
  const [agendas, notifications] = await Promise.all([
    api("/api/agendas"),
    api("/api/notifications"),
  ]);
  const oldIds = new Set(state.seenNotificationIds);
  setDashboardData(agendas, notifications);
  render();
  showNotificationAlerts(notifications, oldIds, alertNew);
  state.seenNotificationIds = new Set(notifications.map((notice) => notice.id));
}

async function deleteAgenda(event) {
  const button = event.target.closest("#deleteAgendaBtn");
  if (!button) return;

  const agendaID = Number(button.dataset.id);
  const agendaTitle = button.dataset.title || "agenda ini";
  if (!agendaID) {
    showToast("Hapus ditolak", "Pilih agenda terlebih dahulu.");
    return;
  }
  if (!window.confirm(`Hapus ${agendaTitle}? Agenda akan hilang dari sistem.`)) {
    return;
  }

  try {
    await api(`/api/agendas/${agendaID}`, { method: "DELETE" });
    state.selectedId = null;
    await refresh({ alertNew: false });
    navigateToView("agenda");
    render();
    showToast("Agenda dihapus", "Agenda berhasil dihapus dari sistem.");
  } catch (error) {
    showToast("Hapus ditolak", error.message);
  }
}

async function createAgenda(event) {
  event.preventDefault();
  const errorMessage = agendaFormError(event.target);
  if (errorMessage) {
    showToast("Agenda belum lengkap", errorMessage);
    return;
  }

  try {
    await api("/api/agendas", { method: "POST", body: new FormData(event.target) });
    event.target.reset();
    setDefaultDates();
    await refresh({ alertNew: false });
    showToast("Agenda terkirim", "Agenda baru sudah dikirim ke pimpinan.");
  } catch (error) {
    showToast("Gagal menyimpan agenda", error.message);
  }
}

function agendaFormError(form) {
  const requiredFields = [
    ["title", "Nama kegiatan"],
    ["location", "Lokasi"],
    ["start_at", "Waktu mulai"],
    ["end_at", "Waktu selesai"],
    ["organizer", "Penyelenggara"],
    ["staff_note", "Keterangan"],
  ];

  for (const [name, label] of requiredFields) {
    if (!String(form.elements[name].value || "").trim()) {
      return `${label} wajib diisi.`;
    }
  }

  if (!form.elements.invitation.files.length) {
    return "File undangan wajib diunggah.";
  }

  const startAt = new Date(form.elements.start_at.value);
  const endAt = new Date(form.elements.end_at.value);
  if (Number.isNaN(startAt.getTime()) || Number.isNaN(endAt.getTime())) {
    return "Waktu mulai dan selesai wajib valid.";
  }
  if (endAt <= startAt) {
    return "Waktu selesai harus setelah waktu mulai.";
  }

  return "";
}

async function validateAgenda(event) {
  event.preventDefault();
  const agenda = selectedAgendaByControl("validationAgendaId");
  if (!agenda) {
    showToast("Validasi ditolak", "Pilih agenda terlebih dahulu.");
    return;
  }

  const form = new FormData(event.target);
  try {
    await api(`/api/agendas/${agenda.id}/validation`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        status: form.get("status"),
        delegate: form.get("delegate"),
        leader_note: form.get("leader_note"),
      }),
    });
    await refresh({ alertNew: false });
    showToast("Validasi tersimpan", "Keputusan dan keterangan pimpinan sudah disimpan.");
  } catch (error) {
    showToast("Validasi ditolak", error.message);
  }
}

async function uploadDocumentation(event) {
  event.preventDefault();
  const agenda = selectedAgendaByControl("documentationAgendaId");
  if (!agenda) {
    showToast("Upload ditolak", "Pilih agenda terlebih dahulu.");
    return;
  }
  if (!agenda.can_upload_documentation) {
    showToast("Upload ditolak", documentationErrorMessage(agenda));
    return;
  }

  try {
    await api(`/api/agendas/${agenda.id}/documentation`, {
      method: "POST",
      body: new FormData(event.target),
    });
    event.target.reset();
    await refresh({ alertNew: false });
    showToast("Laporan tersimpan", "Dokumentasi kegiatan sudah dapat dilihat dan didownload.");
  } catch (error) {
    showToast("Upload ditolak", error.message);
  }
}

function documentationErrorMessage(agenda) {
  if (agenda.status === "menunggu") {
    return "Agenda harus divalidasi pimpinan sebelum laporan dibuat.";
  }
  if (agenda.documentation) {
    return "Laporan untuk agenda ini sudah tersimpan dan bisa dilihat di menu Laporan.";
  }
  return "Laporan baru bisa dibuat setelah waktu selesai kegiatan.";
}

function selectAgendaFromControl(event) {
  state.selectedId = Number(event.target.value) || null;
  render();
}

function selectedAgendaByControl(controlId) {
  const agendaID = Number(el(controlId).value);
  const agenda = state.agendas.find((item) => item.id === agendaID) ?? selectedAgenda();
  state.selectedId = agenda?.id ?? null;
  return agenda;
}

function togglePassword() {
  togglePasswordField("password", "togglePassword");
}

function togglePasswordField(inputId, buttonId) {
  const input = el(inputId);
  const button = el(buttonId);
  const isHidden = input.type === "password";

  input.type = isHidden ? "text" : "password";
  button.setAttribute("aria-label", isHidden ? "Sembunyikan password" : "Lihat password");
  button.setAttribute("title", isHidden ? "Sembunyikan password" : "Lihat password");
  button.classList.toggle("is-visible", isHidden);
}

function showAuthForm(mode) {
  const isRegister = mode === "register";
  el("loginForm").classList.toggle("hidden", isRegister);
  el("registerForm").classList.toggle("hidden", !isRegister);
  el("showLoginForm").classList.toggle("active", !isRegister);
  el("showRegisterForm").classList.toggle("active", isRegister);
  el("authTitle").textContent = isRegister ? "Daftar pengguna" : "Login pengguna";
  el("loginError").classList.add("hidden");
  el("registerError").classList.add("hidden");
}

function startNotificationPolling() {
  if (state.pollTimer) return;
  state.pollTimer = setInterval(() => {
    refresh({ alertNew: true }).catch(() => {});
  }, 15000);
}

function showNotificationAlerts(notifications, oldIds, alertNew) {
  if (!alertNew || oldIds.size === 0) return;
  notifications
    .filter((notice) => !oldIds.has(notice.id))
    .reverse()
    .forEach((notice) => showToast(notice.title, notice.body));
}

function showToast(title, body) {
  const container = el("toastContainer");
  if (!container) return;

  const item = document.createElement("article");
  item.className = "toast-alert";

  const heading = document.createElement("strong");
  heading.textContent = title;
  const text = document.createElement("p");
  text.textContent = body;

  item.append(heading, text);
  container.append(item);
  setTimeout(() => item.remove(), 5200);
}
