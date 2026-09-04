export const state = {
  user: null,
  agendas: [],
  notifications: [],
  selectedId: null,
  activeView: "dashboard",
};

export function selectedAgenda() {
  return state.agendas.find((agenda) => agenda.id === state.selectedId) ?? null;
}

export function clearSessionState() {
  state.user = null;
  state.agendas = [];
  state.notifications = [];
  state.selectedId = null;
  state.activeView = "dashboard";
}

export function setDashboardData(agendas, notifications) {
  state.agendas = Array.isArray(agendas) ? agendas : [];
  state.notifications = Array.isArray(notifications) ? notifications : [];

  const selectedExists = state.agendas.some((agenda) => agenda.id === state.selectedId);
  if (!selectedExists) {
    state.selectedId = state.agendas[0]?.id ?? null;
  }
}
