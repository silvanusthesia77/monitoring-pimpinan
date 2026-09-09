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
  syncActiveSelectionForView();
  renderFormSelectors();
  renderReports();
  renderActiveView();
  renderDetail();
  renderNotifications();
}

function renderHeader() {
  el("activeRole").textContent = roleLabel(state.user.role);
  el("activeUser").textContent = `${state.user.name}\n${state.user.position}`;
  el("navbarRole").textContent = `${state.user.name} - ${roleLabel(state.user.role)}`;
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
  const isLeader = state.user.role === "pimpinan";
  const allowedViews = navItems().map((item) => item.id);
  if (!allowedViews.includes(state.activeView)) {
    navigateToView("dashboard");
  }

  el("agendaSidebarCard").classList.remove("hidden");
  toggleView("dashboardPanel", state.activeView === "dashboard");
  toggleView("agendaPanel", state.activeView === "agenda");
  toggleView("detailPanel", state.activeView === "detail");
  toggleView("staffPanel", isStaff && state.activeView === "input");
  toggleView("leaderPanel", isLeader && state.activeView === "validasi");
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
  downloads.innerHTML = agendaDownloads(agenda) + leaderDeleteAction(agenda);
  updateValidationControls(agenda);
  updateDocumentationControls(agenda);
}

function renderFormSelectors() {
  syncSelectOptions("validationAgendaId", validationAgendas(), "Belum ada agenda yang bisa divalidasi");
  syncSelectOptions("documentationAgendaId", documentationAgendas(), "Belum ada agenda yang siap dibuat laporan");
}

function syncSelectOptions(id, agendas, emptyText) {
  const select = el(id);
  const selectedExists = agendas.some((agenda) => agenda.id === state.selectedId);
  const selectedValue = String(selectedExists ? state.selectedId : agendas[0]?.id ?? "");
  select.innerHTML = agendas.length
    ? agendas.map((agenda) => `<option value="${agenda.id}">${escapeHtml(agenda.title)} - ${escapeHtml(agenda.display_status || statusLabel(agenda))}</option>`).join("")
    : `<option value="">${emptyText}</option>`;
  select.value = selectedValue;
}

function validationAgendas() {
  return state.agendas;
}

function documentationAgendas() {
  return state.agendas.filter((agenda) => agenda.status !== "menunggu" || agenda.documentation);
}

function syncActiveSelectionForView() {
  if (state.activeView === "validasi") {
    ensureSelection(validationAgendas(), (agenda) => agenda.can_validate);
  }
  if (state.activeView === "dokumentasi") {
    ensureSelection(documentationAgendas(), (agenda) => agenda.can_upload_documentation);
  }
}

function ensureSelection(agendas, preferred = () => true) {
  if (!agendas.length) {
    state.selectedId = null;
    return;
  }
  if (!agendas.some((agenda) => agenda.id === state.selectedId)) {
    state.selectedId = (agendas.find(preferred) ?? agendas[0]).id;
  }
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
    agenda.documentation ? reportPDFLink(agenda) : "",
  ].join("");
}

function leaderDeleteAction(agenda) {
  if (state.user.role !== "pimpinan") return "";
  return `<button id="deleteAgendaBtn" class="button-danger" data-id="${agenda.id}" data-title="${escapeHtml(agenda.title)}" type="button">Hapus Agenda</button>`;
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
        ${reportPDFLink(agenda)}
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
      ${notice.email_message ? `<p class="email-status ${notice.email_sent ? "sent" : "failed"}">${escapeHtml(notice.email_message)}</p>` : ""}
      <time class="text-xs font-bold text-slate-500">${formatDate(notice.created_at)}</time>
    </article>
  `;
}

function detailRow(label, value) {
  return `<div class="detail-row"><dt>${label}</dt><dd>${escapeHtml(value)}</dd></div>`;
}

function downloadLink(label, file) {
  return `<a class="download-link" href="/api/files/${file.id}/download">${label}: ${escapeHtml(file.original_name)}</a>`;
}

function reportPDFLink(agenda) {
  return `<a class="download-link" href="/api/agendas/${agenda.id}/report-pdf">Download PDF Berita Acara</a>`;
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
  const pullbackSubmit = el("pullbackSubmit");
  const validationForm = el("validationForm");
  const validationHint = el("validationHint");

  validationSubmit.removeAttribute("disabled");
  pullbackSubmit.classList.add("hidden");
  if (!agenda) {
    validationHint.textContent = "Belum ada agenda yang dapat dipilih untuk validasi.";
    return;
  }

  validationSubmit.textContent = agenda.status === "menunggu" ? "Validasi" : "Simpan Perubahan";
  pullbackSubmit.classList.toggle("hidden", agenda.status === "menunggu");
  if (agenda.can_validate && agenda.status === "menunggu") {
    validationHint.textContent = "Agenda belum divalidasi dan masih bisa diproses pimpinan.";
  } else if (agenda.can_validate) {
    validationHint.textContent = "Agenda sudah divalidasi. Pimpinan bisa tarik validasi untuk mengganti pesan selama belum terkunci 24 jam.";
  } else if (agenda.status === "menunggu") {
    validationHint.textContent = "Agenda belum divalidasi, tetapi waktu kegiatan sudah berjalan atau sudah lewat.";
  } else {
    validationHint.textContent = "Perubahan validasi ditutup karena kegiatan kurang dari 24 jam atau sudah berjalan.";
  }

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

  reportSubmit.removeAttribute("disabled");
  if (!agenda) {
    reportAgendaBox.innerHTML = `<span>Agenda aktif</span><strong>Belum ada agenda dipilih</strong>`;
    reportHint.textContent = "Pilih agenda terlebih dahulu dari dropdown.";
    return;
  }

  reportAgendaBox.innerHTML = `
    <span>Agenda aktif</span>
    <strong>${escapeHtml(agenda.title)}</strong>
    <small>${escapeHtml(agenda.display_status || statusLabel(agenda))} - ${formatDate(agenda.end_at)}</small>
  `;

  if (agenda.can_upload_documentation) {
    reportHint.textContent = "Agenda sudah divalidasi. Dokumentasi dan ringkasan laporan bisa diupload.";
  } else if (agenda.status === "menunggu") {
    reportHint.textContent = "Agenda harus divalidasi pimpinan sebelum laporan kegiatan diupload.";
  } else if (agenda.documentation) {
    reportHint.textContent = "Dokumentasi sudah tersimpan dan dapat didownload pada detail agenda.";
  } else {
    reportHint.textContent = "Agenda sudah divalidasi. Lengkapi dokumentasi dan ringkasan laporan.";
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
      { id: "dokumentasi", label: "Buat Laporan", icon: "upload" },
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

function roleLabel(role) {
  if (role === "staf") return "Staf";
  return "Pimpinan";
}

function toggleView(id, visible) {
  el(id).classList.toggle("hidden", !visible);
}
