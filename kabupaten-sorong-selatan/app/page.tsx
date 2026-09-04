"use client";

import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";

type Role = "staf" | "pimpinan";
type AttendanceStatus = "menunggu" | "hadir" | "diwakili";

type StoredFile = {
  name: string;
  type: string;
  size: number;
  dataUrl: string;
};

type Agenda = {
  id: string;
  title: string;
  location: string;
  startAt: string;
  endAt: string;
  organizer: string;
  staffNote: string;
  invitation?: StoredFile;
  status: AttendanceStatus;
  delegate: string;
  leaderNote: string;
  validatedAt?: string;
  pulledBackAt?: string;
  documentation?: StoredFile;
  reportNote?: string;
};

type Notice = {
  id: string;
  audience: Role | "semua";
  title: string;
  body: string;
  at: string;
};

const representatives = [
  "Sekretaris Daerah",
  "Asisten Pemerintahan",
  "Kepala Bagian Protokol",
  "Kepala OPD Terkait",
  "Staf Ahli Bupati",
];

const seedAgendas: Agenda[] = [
  {
    id: "agenda-rapat-forkopimda",
    title: "Rapat Koordinasi Forkopimda",
    location: "Aula Kantor Bupati Sorong Selatan",
    startAt: futureDate(30, 9),
    endAt: futureDate(30, 12),
    organizer: "Bagian Tata Pemerintahan",
    staffNote: "Agenda prioritas, undangan fisik diterima melalui protokol.",
    invitation: {
      name: "undangan-forkopimda.txt",
      type: "text/plain",
      size: 92,
      dataUrl:
        "data:text/plain;charset=utf-8,Undangan%20Rapat%20Koordinasi%20Forkopimda%20Kabupaten%20Sorong%20Selatan",
    },
    status: "menunggu",
    delegate: "",
    leaderNote: "",
  },
  {
    id: "agenda-kunjungan-distrik",
    title: "Kunjungan Kerja Distrik Teminabuan",
    location: "Kantor Distrik Teminabuan",
    startAt: futureDate(2, 8),
    endAt: futureDate(2, 15),
    organizer: "Dinas Pemberdayaan Masyarakat Kampung",
    staffNote: "Sudah mendekati waktu kegiatan sehingga perubahan validasi terkunci.",
    status: "hadir",
    delegate: "",
    leaderNote: "Saya hadir langsung.",
    validatedAt: new Date().toISOString(),
  },
];

function futureDate(days: number, hour: number) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  date.setHours(hour, 0, 0, 0);
  return toDateTimeInput(date);
}

function toDateTimeInput(date: Date) {
  const offset = date.getTimezoneOffset();
  const local = new Date(date.getTime() - offset * 60_000);
  return local.toISOString().slice(0, 16);
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("id-ID", {
    weekday: "short",
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function hoursUntil(startAt: string) {
  return (new Date(startAt).getTime() - Date.now()) / 36e5;
}

function canRevise(agenda: Agenda) {
  return hoursUntil(agenda.startAt) >= 24;
}

function fileToStored(file: File): Promise<StoredFile> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () =>
      resolve({
        name: file.name,
        type: file.type || "application/octet-stream",
        size: file.size,
        dataUrl: String(reader.result),
      });
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

export default function Home() {
  const hydrated = useRef(false);
  const [role, setRole] = useState<Role>("staf");
  const [agendas, setAgendas] = useState<Agenda[]>(seedAgendas);
  const [notices, setNotices] = useState<Notice[]>([
    {
      id: "notice-awal",
      audience: "semua",
      title: "Sistem siap digunakan",
      body: "Staf dapat menginput jadwal dan pimpinan dapat melakukan validasi agenda.",
      at: new Date().toISOString(),
    },
  ]);
  const [selectedId, setSelectedId] = useState(seedAgendas[0].id);
  const [form, setForm] = useState({
    title: "",
    location: "",
    startAt: futureDate(7, 10),
    endAt: futureDate(7, 12),
    organizer: "",
    staffNote: "",
  });
  const [invitation, setInvitation] = useState<StoredFile | undefined>();
  const [documentation, setDocumentation] = useState<StoredFile | undefined>();
  const [reportNote, setReportNote] = useState("");
  const [leaderChoice, setLeaderChoice] = useState<AttendanceStatus>("hadir");
  const [delegate, setDelegate] = useState(representatives[0]);
  const [leaderNote, setLeaderNote] = useState("");

  useEffect(() => {
    const savedAgendas = localStorage.getItem("sorsel-agendas");
    const savedNotices = localStorage.getItem("sorsel-notices");
    if (savedAgendas) {
      const parsed = JSON.parse(savedAgendas) as Agenda[];
      setAgendas(parsed);
      setSelectedId(parsed[0]?.id ?? "");
    }
    if (savedNotices) setNotices(JSON.parse(savedNotices) as Notice[]);
    hydrated.current = true;
  }, []);

  useEffect(() => {
    if (!hydrated.current) return;
    localStorage.setItem("sorsel-agendas", JSON.stringify(agendas));
  }, [agendas]);

  useEffect(() => {
    if (!hydrated.current) return;
    localStorage.setItem("sorsel-notices", JSON.stringify(notices));
  }, [notices]);

  const selectedAgenda = useMemo(
    () => agendas.find((agenda) => agenda.id === selectedId) ?? agendas[0],
    [agendas, selectedId],
  );

  const stats = useMemo(
    () => ({
      waiting: agendas.filter((agenda) => agenda.status === "menunggu").length,
      validated: agendas.filter((agenda) => agenda.status !== "menunggu").length,
      locked: agendas.filter((agenda) => !canRevise(agenda)).length,
      reports: agendas.filter((agenda) => agenda.documentation).length,
    }),
    [agendas],
  );

  function pushNotice(audience: Notice["audience"], title: string, body: string) {
    setNotices((current) => [
      { id: crypto.randomUUID(), audience, title, body, at: new Date().toISOString() },
      ...current,
    ]);
  }

  async function handleFile(
    event: ChangeEvent<HTMLInputElement>,
    setter: (file: StoredFile | undefined) => void,
  ) {
    const file = event.target.files?.[0];
    if (!file) return;
    setter(await fileToStored(file));
  }

  function submitAgenda(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const agenda: Agenda = {
      id: crypto.randomUUID(),
      ...form,
      invitation,
      status: "menunggu",
      delegate: "",
      leaderNote: "",
    };
    setAgendas((current) => [agenda, ...current]);
    setSelectedId(agenda.id);
    pushNotice(
      "pimpinan",
      "Agenda baru menunggu validasi",
      `${form.title} telah disubmit staf. Email pemberitahuan disimulasikan terkirim ke pimpinan.`,
    );
    setForm({
      title: "",
      location: "",
      startAt: futureDate(7, 10),
      endAt: futureDate(7, 12),
      organizer: "",
      staffNote: "",
    });
    setInvitation(undefined);
  }

  function validateAgenda() {
    if (!selectedAgenda) return;
    const nextDelegate = leaderChoice === "diwakili" ? delegate : "";
    setAgendas((current) =>
      current.map((agenda) =>
        agenda.id === selectedAgenda.id
          ? {
              ...agenda,
              status: leaderChoice,
              delegate: nextDelegate,
              leaderNote,
              validatedAt: new Date().toISOString(),
              pulledBackAt: undefined,
            }
          : agenda,
      ),
    );
    pushNotice(
      "staf",
      "Agenda telah divalidasi pimpinan",
      `${selectedAgenda.title} diputuskan ${leaderChoice === "hadir" ? "dihadiri langsung" : `diwakili oleh ${nextDelegate}`}.`,
    );
  }

  function pullBackValidation() {
    if (!selectedAgenda || !canRevise(selectedAgenda)) return;
    setAgendas((current) =>
      current.map((agenda) =>
        agenda.id === selectedAgenda.id
          ? {
              ...agenda,
              status: "menunggu",
              delegate: "",
              leaderNote: "",
              pulledBackAt: new Date().toISOString(),
            }
          : agenda,
      ),
    );
    setLeaderChoice("hadir");
    setLeaderNote("");
    pushNotice(
      "staf",
      "Validasi agenda ditarik kembali",
      `${selectedAgenda.title} dikembalikan ke status menunggu karena pimpinan melakukan perubahan.`,
    );
  }

  function submitDocumentation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedAgenda || !documentation) return;
    setAgendas((current) =>
      current.map((agenda) =>
        agenda.id === selectedAgenda.id
          ? { ...agenda, documentation, reportNote }
          : agenda,
      ),
    );
    pushNotice(
      "pimpinan",
      "Dokumentasi kegiatan tersedia",
      `Laporan dokumentasi untuk ${selectedAgenda.title} telah diunggah staf.`,
    );
    setDocumentation(undefined);
    setReportNote("");
  }

  const visibleNotices = notices.filter(
    (notice) => notice.audience === role || notice.audience === "semua",
  );

  return (
    <main className="min-h-screen bg-[#f6f7f2] text-[#1d2433]">
      <section className="hero-band">
        <div className="hero-content">
          <div>
            <p className="eyebrow">Kabupaten Sorong Selatan</p>
            <h1>Aplikasi Monitor Agenda Pimpinan</h1>
            <p className="hero-copy">
              Monitoring jadwal kunjungan, validasi kehadiran, delegasi, undangan,
              notifikasi, dan dokumentasi kegiatan dalam satu ruang kerja.
            </p>
          </div>
          <div className="role-switch" aria-label="Pilih aktor">
            <button
              className={role === "staf" ? "active" : ""}
              onClick={() => setRole("staf")}
              type="button"
            >
              Staf
            </button>
            <button
              className={role === "pimpinan" ? "active" : ""}
              onClick={() => setRole("pimpinan")}
              type="button"
            >
              Pimpinan
            </button>
          </div>
        </div>
      </section>

      <section className="dashboard-shell">
        <aside className="sidebar">
          <div className="metric-grid">
            <Metric label="Menunggu" value={stats.waiting} />
            <Metric label="Tervalidasi" value={stats.validated} />
            <Metric label="Terkunci" value={stats.locked} />
            <Metric label="Laporan" value={stats.reports} />
          </div>

          <div className="panel">
            <div className="panel-heading">
              <h2>Daftar Agenda</h2>
              <span>{agendas.length} item</span>
            </div>
            <div className="agenda-list">
              {agendas.map((agenda) => (
                <button
                  key={agenda.id}
                  className={`agenda-item ${selectedAgenda?.id === agenda.id ? "selected" : ""}`}
                  onClick={() => setSelectedId(agenda.id)}
                  type="button"
                >
                  <span className={`status-dot ${agenda.status}`} />
                  <span>
                    <strong>{agenda.title}</strong>
                    <small>{formatDate(agenda.startAt)}</small>
                  </span>
                </button>
              ))}
            </div>
          </div>
        </aside>

        <div className="content-grid">
          {role === "staf" ? (
            <section className="panel work-panel">
              <div className="panel-heading">
                <div>
                  <p className="eyebrow">Input Staf</p>
                  <h2>Tambah Jadwal Kunjungan</h2>
                </div>
              </div>
              <form className="form-grid" onSubmit={submitAgenda}>
                <label>
                  Nama kegiatan
                  <input
                    required
                    value={form.title}
                    onChange={(event) => setForm({ ...form, title: event.target.value })}
                    placeholder="Contoh: Pembukaan Musrenbang Distrik"
                  />
                </label>
                <label>
                  Lokasi
                  <input
                    required
                    value={form.location}
                    onChange={(event) => setForm({ ...form, location: event.target.value })}
                    placeholder="Lokasi kegiatan"
                  />
                </label>
                <label>
                  Mulai
                  <input
                    required
                    type="datetime-local"
                    value={form.startAt}
                    onChange={(event) => setForm({ ...form, startAt: event.target.value })}
                  />
                </label>
                <label>
                  Selesai
                  <input
                    required
                    type="datetime-local"
                    value={form.endAt}
                    onChange={(event) => setForm({ ...form, endAt: event.target.value })}
                  />
                </label>
                <label>
                  Penyelenggara
                  <input
                    required
                    value={form.organizer}
                    onChange={(event) => setForm({ ...form, organizer: event.target.value })}
                    placeholder="Instansi atau panitia"
                  />
                </label>
                <label>
                  Upload undangan
                  <input
                    type="file"
                    accept=".pdf,.doc,.docx,.jpg,.jpeg,.png,.txt"
                    onChange={(event) => handleFile(event, setInvitation)}
                  />
                  {invitation && <small>{invitation.name} - {formatBytes(invitation.size)}</small>}
                </label>
                <label className="full">
                  Keterangan staf
                  <textarea
                    required
                    value={form.staffNote}
                    onChange={(event) => setForm({ ...form, staffNote: event.target.value })}
                    placeholder="Catatan transportasi, protokol, urgensi, atau arahan awal"
                  />
                </label>
                <button className="primary-action" type="submit">Submit ke Pimpinan</button>
              </form>
            </section>
          ) : (
            <section className="panel work-panel">
              <div className="panel-heading">
                <div>
                  <p className="eyebrow">Validasi Pimpinan</p>
                  <h2>Keputusan Kehadiran</h2>
                </div>
                {selectedAgenda && (
                  <span className={canRevise(selectedAgenda) ? "badge ok" : "badge locked"}>
                    {canRevise(selectedAgenda) ? "Bisa diubah" : "Terkunci < 24 jam"}
                  </span>
                )}
              </div>
              {selectedAgenda && (
                <div className="decision-box">
                  <div className="segmented">
                    <button
                      className={leaderChoice === "hadir" ? "active" : ""}
                      onClick={() => setLeaderChoice("hadir")}
                      type="button"
                    >
                      Hadir sendiri
                    </button>
                    <button
                      className={leaderChoice === "diwakili" ? "active" : ""}
                      onClick={() => setLeaderChoice("diwakili")}
                      type="button"
                    >
                      Diwakili
                    </button>
                  </div>
                  {leaderChoice === "diwakili" && (
                    <label>
                      Pejabat yang mewakili
                      <select value={delegate} onChange={(event) => setDelegate(event.target.value)}>
                        {representatives.map((person) => (
                          <option key={person}>{person}</option>
                        ))}
                      </select>
                    </label>
                  )}
                  <label>
                    Keterangan pimpinan
                    <textarea
                      value={leaderNote}
                      onChange={(event) => setLeaderNote(event.target.value)}
                      placeholder="Arahan atau catatan disposisi"
                    />
                  </label>
                  <div className="action-row">
                    <button className="primary-action" type="button" onClick={validateAgenda}>
                      Validasi
                    </button>
                    <button
                      className="secondary-action"
                      disabled={selectedAgenda.status === "menunggu" || !canRevise(selectedAgenda)}
                      onClick={pullBackValidation}
                      type="button"
                    >
                      Tarik Kembali
                    </button>
                  </div>
                </div>
              )}
            </section>
          )}

          <section className="panel detail-panel">
            <div className="panel-heading">
              <div>
                <p className="eyebrow">Detail Agenda</p>
                <h2>{selectedAgenda?.title ?? "Belum ada agenda"}</h2>
              </div>
              {selectedAgenda && <StatusBadge status={selectedAgenda.status} />}
            </div>

            {selectedAgenda && (
              <>
                <dl className="detail-grid">
                  <div><dt>Lokasi</dt><dd>{selectedAgenda.location}</dd></div>
                  <div><dt>Waktu</dt><dd>{formatDate(selectedAgenda.startAt)} sampai {formatDate(selectedAgenda.endAt)}</dd></div>
                  <div><dt>Penyelenggara</dt><dd>{selectedAgenda.organizer}</dd></div>
                  <div><dt>Sisa waktu</dt><dd>{Math.max(0, Math.floor(hoursUntil(selectedAgenda.startAt)))} jam</dd></div>
                  <div><dt>Keterangan staf</dt><dd>{selectedAgenda.staffNote}</dd></div>
                  <div><dt>Keterangan pimpinan</dt><dd>{selectedAgenda.leaderNote || "Belum ada keterangan."}</dd></div>
                  <div><dt>Perwakilan</dt><dd>{selectedAgenda.delegate || "Tidak ada"}</dd></div>
                  <div><dt>Aturan perubahan</dt><dd>{canRevise(selectedAgenda) ? "Validasi masih dapat ditarik kembali." : "Perubahan ditutup karena kegiatan kurang dari 24 jam atau sudah berjalan."}</dd></div>
                </dl>

                <div className="downloads">
                  {selectedAgenda.invitation ? (
                    <DownloadFile label="Download undangan" file={selectedAgenda.invitation} />
                  ) : (
                    <span className="muted-pill">Undangan belum diunggah</span>
                  )}
                  {selectedAgenda.documentation ? (
                    <DownloadFile label="Download dokumentasi" file={selectedAgenda.documentation} />
                  ) : (
                    <span className="muted-pill">Dokumentasi belum tersedia</span>
                  )}
                </div>
              </>
            )}
          </section>

          {role === "staf" && selectedAgenda && (
            <section className="panel report-panel">
              <div className="panel-heading">
                <div>
                  <p className="eyebrow">Laporan Kegiatan</p>
                  <h2>Upload Dokumentasi</h2>
                </div>
              </div>
              <form className="form-grid compact" onSubmit={submitDocumentation}>
                <label>
                  File dokumentasi
                  <input
                    required
                    type="file"
                    accept=".pdf,.jpg,.jpeg,.png,.zip,.doc,.docx"
                    onChange={(event) => handleFile(event, setDocumentation)}
                  />
                  {documentation && <small>{documentation.name} - {formatBytes(documentation.size)}</small>}
                </label>
                <label>
                  Ringkasan laporan
                  <textarea
                    value={reportNote}
                    onChange={(event) => setReportNote(event.target.value)}
                    placeholder="Ringkasan hasil kegiatan"
                  />
                </label>
                <button className="primary-action" type="submit">Simpan Laporan</button>
              </form>
            </section>
          )}

          <section className="panel notification-panel">
            <div className="panel-heading">
              <div>
                <p className="eyebrow">Notifikasi {role}</p>
                <h2>Alert Sistem</h2>
              </div>
            </div>
            <div className="notice-list">
              {visibleNotices.slice(0, 6).map((notice) => (
                <article key={notice.id} className="notice">
                  <strong>{notice.title}</strong>
                  <p>{notice.body}</p>
                  <time>{formatDate(notice.at)}</time>
                </article>
              ))}
            </div>
          </section>
        </div>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="metric">
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}

function StatusBadge({ status }: { status: AttendanceStatus }) {
  const label = status === "menunggu" ? "Menunggu" : status === "hadir" ? "Hadir" : "Diwakili";
  return <span className={`badge ${status}`}>{label}</span>;
}

function DownloadFile({ label, file }: { label: string; file: StoredFile }) {
  return (
    <a className="download-link" href={file.dataUrl} download={file.name}>
      {label}
      <small>{file.name}</small>
    </a>
  );
}
