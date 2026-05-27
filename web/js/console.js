import { state, $ } from "./state.js";

export function collapse(value) {
  state.consoleCollapsed = value;
  $("developer-console").classList.toggle("collapsed", value);
  $("console-toggle").textContent = value ? "+" : "-";
}
