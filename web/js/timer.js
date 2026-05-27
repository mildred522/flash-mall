import { state, $ } from "./state.js";

export function timerTick() {
  const total = Math.max(0, state.timerSeconds);
  const hh = String(Math.floor(total / 3600)).padStart(2, "0");
  const mm = String(Math.floor((total % 3600) / 60)).padStart(2, "0");
  const ss = String(total % 60).padStart(2, "0");
  $("hero-timer").textContent = `${hh}:${mm}:${ss}`;
  if (state.timerSeconds > 0) state.timerSeconds -= 1;
}
