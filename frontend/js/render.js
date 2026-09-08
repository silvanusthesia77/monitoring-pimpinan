import { el, escapeHtml, formatDate } from "./dom.js";
import { navigateToView, pathForView } from "./router.js";
import { selectedAgenda, state } from "./state.js";

export function showLogin() {
  document.body.classList.remove("app-mode");
  el("loginView").classList.remove("hidden");
  el("appView").classList.add("hidden");
  el("logoutBtn").classList.add("hidden");
}

export function showApp() {
  document.body.classList.add("app-mode");
  el("loginView").classList.add("hidden");
  el("appView").classList.remove("hidden");
  el("logoutBtn").classList.remove("hidden");
}

export function render() {
  renderHeader();
  renderNavigation();
  renderStats();
  renderAgendaList();
  renderAgendaTable();
  renderFormSelectors();
  renderReports();
  renderActiveView();
  renderDetail();
  renderNotifications();
}

function renderHeader() {
  el("activeRole").textContent = state.user.role === "staf" ? "Staf" : "Pimpinan";
  el("activeUser").textContent = `${state.user.name}\n${state.user.position}`;
  el("navbarRole").textContent = `${state.user.name} - ${state.user.role === "staf" ? "Staf" : "Pimpinan"}`;
}

function renderStats() {
  el("statWaiting").textContent = state.agendas.filter((agenda) => agenda.phase === "menunggu_validasi").length;
  el("statValidated").textContent = state.agendas.filter((agenda) =>
    ["tervalidasi_hadir", "tervalidasi_diwakili"].includes(agenda.phase),
  ).length;
  el("statLocked").textContent = state.agendas.filter((agenda) => agenda.is_locked).length;
  el("statReports").textContent = state.agendas.filter((agenda) => agenda.phase === "selesai").length;
}

function renderAgendaList() {
  el("agendaList").innerHTML = state.agendas.length
    ? state.agendas.slice(0, 8).map(agendaItem).join("")
    : `<p class="empty-text">Belum ada agenda.</p>`;
  document.querySelectorAll(".agenda-item").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedId = Number(button.dataset.id);
      navigateToView("detail");
      render();
    });
  });
}

function renderNavigation() {
  el("sidebarNav").innerHTML = navItems()
    .map(
      (item) => `
        <button class="nav-item ${state.activeView === item.id ? "active" : ""}" data-view="${item.id}" data-path="${pathForView(item.id)}" type="button">
          <span class="nav-icon nav-icon-${item.icon}" aria-hidden="true"></span>
          <span>${item.label}</span>
          ${item.count === undefined ? "" : `<strong>${item.count}</strong>`}
        </button>
      `,
    )
    .join("");

  document.querySelectorAll(".nav-item").forEach((button) => {
    button.addEventListener("click", () => {
      navigateToView(button.dataset.view);
      render();
    });
  });
}

function renderActiveView() {
  const isStaff = state.user.role === "staf";
  const allowedViews = navItems().map((item) => item.id);
  if (!allowedViews.includes(state.activeView)) {
    navigateToView("dashboard");
  }

  toggleView("dashboardPanel", state.activeView === "dashboard");
  toggleView("agendaPanel", state.activeView === "agenda");
  toggleView("detailPanel", state.activeView === "detail");
  toggleView("staffPanel", isStaff && state.activeView === "input");
  toggleView("leaderPanel", !isStaff && state.activeView === "validasi");
  toggleView("documentationPanel", isStaff && state.activeView === "dokumentasi");
  toggleView("reportsPanel", state.activeView === "laporan");
  toggleView("notificationPanel", state.activeView === "notifikasi");
}

function renderDetail() {
  const agenda = selectedAgenda();
  const title = el("detailTitle");
  const status = el("detailStatus");
  const body = el("detailBody");
  const downloads = el("downloadBox");

  if (!agenda) {
    title.textContent = "Belum ada agenda";
    status.textContent = "-";
    status.className = "badge";
    body.innerHTML = "";
    downloads.innerHTML = "";
    updateValidationControls(null);
    updateDocumentationControls(null);
    return;
  }

  title.textContent = agenda.title;
  status.textContent = statusLabel(agenda);
  status.className = `badge ${agenda.phase || agenda.status}`;
  body.innerHTML = agendaDetails(agenda);
  downloads.innerHTML = agendaDownloads(agenda);
  updateValidationControls(agenda);
  updateDocumentationControls(agenda);
}

function renderFormSelectors() {
  syncSelectOptions("validationAgendaId", state.agendas);
  syncSelectOptions("documentationAgendaId", state.agendas);
}

function syncSelectOptions(id, agendas) {
  const select = el(id);
  const selectedValue = String(state.selectedId ?? agendas[0]?.id ?? "");
  select.innerHTML = agendas.length
    ? agendas.map((agenda) => `<option value="${agenda.id}">${escapeHtml(agenda.title)} - ${escapeHtml(agenda.display_status || statusLabel(agenda))}</option>`).join("")
    : `<option value="">Belum ada agenda</option>`;
  select.value = selectedValue;
}

function renderAgendaTable() {
  el("agendaCount").textContent = `${state.agendas.length} agenda`;
  el("agendaTable").innerHTML = state.agendas.length
    ? state.agendas.map(agendaRow).join("")
    : `<p class="empty-text">Belum ada agenda yang tersimpan.</p>`;

  document.querySelectorAll(".agenda-row").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedId = Number(button.dataset.id);
      navigateToView("detail");
      render();
    });
  });
}

function renderNotifications() {
  el("notificationList").innerHTML = state.notifications.length
    ? state.notifications.map(notificationItem).join("")
    : `<p class="text-sm font-bold text-slate-500">Belum ada notifikasi.</p>`;
}

function agendaItem(agenda) {
  return `
    <button class="agenda-item ${agenda.id === state.selectedId ? "active" : ""}" data-id="${agenda.id}" type="button">
      <span class="status-dot ${agenda.phase || agenda.status}"></span>
      <span>
        <strong class="block">${escapeHtml(agenda.title)}</strong>
        <small class="block text-xs font-bold text-slate-500">${formatDate(agenda.start_at)}</small>
      </span>
    </button>
  `;
}

function agendaRow(agenda) {
  return `
    <button class="agenda-row" data-id="${agenda.id}" type="button">
      <span>
        <strong>${escapeHtml(agenda.title)}</strong>
        <small>${escapeHtml(agenda.location)}</small>
      </span>
      <span>${formatDate(agenda.start_at)}</span>
      <span class="badge ${agenda.phase || agenda.status}">${statusLabel(agenda)}</span>
      <span>${agenda.can_pullback ? "Bisa diubah" : agenda.display_status || "Terkunci"}</span>
    </button>
  `;
}

function agendaDetails(agenda) {
  return [
    detailRow("Lokasi", agenda.location),
    detailRow("Waktu", `${formatDate(agenda.start_at)} sampai ${formatDate(agenda.end_at)}`),
    detailRow("Penyelenggara", agenda.organizer),
    detailRow("Status agenda", agenda.display_status || statusLabel(agenda)),
    detailRow("Sisa waktu", `${Math.max(0, agenda.hours_until)} jam`),
    detailRow("Keterangan staf", agenda.staff_note),
    detailRow("Keterangan pimpinan", agenda.leader_note || "Belum ada keterangan."),
    detailRow("Perwakilan", agenda.delegate || "Tidak ada"),
    detailRow(
      "Aturan perubahan",
      agenda.can_revise
        ? "Validasi masih dapat diperbarui."
        : "Perubahan ditutup karena kegiatan kurang dari 24 jam atau sudah berjalan.",
    ),
  ].join("");
}

function agendaDownloads(agenda) {
  return [
    agenda.invitation ? downloadLink("Download undangan", agenda.invitation) : muted("Undangan belum diunggah"),
    agenda.documentation ? downloadLink("Download dokumentasi", agenda.documentation) : muted("Dokumentasi belum tersedia"),
  ].join("");
}

function renderReports() {
  const reports = state.agendas.filter((agenda) => agenda.documentation);
  el("reportsList").innerHTML = reports.length
    ? reports.map(reportCard).join("")
    : `<p class="empty-text">Belum ada laporan kegiatan. Laporan muncul setelah staf mengupload dokumentasi.</p>`;
}

function reportCard(agenda) {
  const attendance = agenda.status === "diwakili" ? `Diwakili oleh ${agenda.delegate}` : "Pimpinan hadir sendiri";
  return `
    <article class="report-card">
      <div class="report-card-header">
        <span>Berita Acara</span>
        <strong>${escapeHtml(agenda.title)}</strong>
        <small>${formatDate(agenda.start_at)} sampai ${formatDate(agenda.end_at)}</small>
      </div>
      <dl class="report-details">
        ${detailRow("Lokasi", agenda.location)}
        ${detailRow("Penyelenggara", agenda.organizer)}
        ${detailRow("Kehadiran", attendance)}
        ${detailRow("Keterangan pimpinan", agenda.leader_note || "Tidak ada keterangan.")}
        ${detailRow("Ringkasan laporan", agenda.report_note || "Tidak ada ringkasan laporan.")}
      </dl>
      <div class="report-actions">
        ${downloadLink("Download dokumentasi", agenda.documentation)}
      </div>
    </article>
  `;
}

function notificationItem(notice) {
  return `
    <article class="notification">
      <strong class="block">${escapeHtml(notice.title)}</strong>
      <p class="my-1 text-sm leading-6 text-slate-600">${escapeHtml(notice.body)}</p>
      <time class="text-xs font-bold text-slate-500">${formatDate(notice.created_at)}${notice.email_sent ? " - email simulasi" : ""}</time>
    </article>
  `;
}

function detailRow(label, value) {
  return `<div class="detail-row"><dt>${label}</dt><dd>${escapeHtml(value)}</dd></div>`;
}

function downloadLink(label, file) {
  return `<a class="download-link" href="/api/files/${file.id}/download">${label}: ${escapeHtml(file.original_name)}</a>`;
}

function muted(text) {
  return `<span class="badge">${text}</span>`;
}

function statusLabel(status) {
  const agenda = typeof status === "object" ? status : { status };
  if (agenda.display_status) return agenda.display_status;
  if (agenda.status === "hadir") return "Tervalidasi - Hadir";
  if (agenda.status === "diwakili") return "Tervalidasi - Diwakili";
  return "Menunggu Validasi";
}

function updateValidationControls(agenda) {
  const validationSubmit = el("validationSubmit");
  const validationForm = el("validationForm");

  if (!agenda) {
    validationSubmit.disabled = true;
    return;
  }

  validationSubmit.disabled = !agenda.can_validate;
  validationSubmit.textContent = agenda.status === "menunggu" ? "Validasi" : "Simpan Perubahan";

  const active = document.activeElement;
  const editingForm = validationForm.contains(active) && active.id !== "validationAgendaId";
  if (!editingForm) {
    validationForm.elements.status.value = agenda.status === "diwakili" ? "diwakili" : "hadir";
    validationForm.elements.delegate.value = agenda.delegate || "Sekretaris Daerah";
    validationForm.elements.leader_note.value = agenda.leader_note || "";
  }
}

function updateDocumentationControls(agenda) {
  const reportSubmit = el("reportSubmit");
  const reportHint = el("documentationHint");
  const reportAgendaBox = el("documentationAgendaBox");

  if (!agenda) {
    reportSubmit.disabled = true;
    reportAgendaBox.innerHTML = `<span>Agenda aktif</span><strong>Belum ada agenda dipilih</strong>`;
    reportHint.textContent = "Pilih agenda terlebih dahulu dari dropdown.";
    return;
  }

  reportSubmit.disabled = !agenda.can_upload_documentation;
  reportAgendaBox.innerHTML = `
    <span>Agenda aktif</span>
    <strong>${escapeHtml(agenda.title)}</strong>
    <small>${escapeHtml(agenda.display_status || statusLabel(agenda))} - ${formatDate(agenda.end_at)}</small>
  `;

  if (agenda.can_upload_documentation) {
    reportHint.textContent = "Kegiatan sudah selesai. Dokumentasi dan catatan laporan bisa diupload.";
  } else if (agenda.status === "menunggu") {
    reportHint.textContent = "Agenda harus divalidasi pimpinan sebelum laporan kegiatan diupload.";
  } else if (agenda.documentation) {
    reportHint.textContent = "Dokumentasi sudah tersimpan dan dapat didownload pada detail agenda.";
  } else {
    reportHint.textContent = "Upload dokumentasi dibuka setelah waktu selesai kegiatan.";
  }
}

function navItems() {
  const common = [
    { id: "dashboard", label: "Dashboard", icon: "dashboard" },
    { id: "agenda", label: "Agenda", icon: "calendar", count: state.agendas.length },
    { id: "detail", label: "Detail Agenda", icon: "file" },
    { id: "laporan", label: "Laporan", icon: "file", count: state.agendas.filter((agenda) => agenda.documentation).length },
    { id: "notifikasi", label: "Notifikasi", icon: "bell", count: state.notifications.length },
  ];

  if (state.user.role === "staf") {
    return [
      common[0],
      { id: "input", label: "Input Jadwal", icon: "plus" },
      common[1],
      common[2],
      { id: "dokumentasi", label: "Dokumentasi", icon: "upload" },
      common[3],
      common[4],
    ];
  }

  return [
    common[0],
    { id: "validasi", label: "Validasi", icon: "check" },
    common[1],
    common[2],
    common[3],
    common[4],
  ];
}

function toggleView(id, visible) {
  el(id).classList.toggle("hidden", !visible);
}
