import { bindEvents, bootstrap, setDefaultDates } from "./js/forms.js";

document.addEventListener("DOMContentLoaded", async () => {
  setDefaultDates();
  bindEvents();
  await bootstrap();
});
