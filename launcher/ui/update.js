// Launcher updates from GitHub releases (update.go).
import { html, useState, S, update, rpc, act, toast, inLobby } from "./core.js";

// UpdateBar sits on top of every page when a newer launcher is out.
export function UpdateBar() {
  const [busy, setBusy] = useState(false);
  const u = S.state && S.state.update;
  if (!u || !u.available || S.updLater === u.latest) return null;
  const running = S.state.game && S.state.game.running;
  const run = async () => {
    setBusy(true);
    const r = await act(() => rpc("applyUpdate"));
    if (r === undefined) setBusy(false);
    else toast("Updated, restarting...");
  };
  return html`<div class="java-banner zb-bar">
    <div class="tile-ico">↻</div>
    <div class="grow"><b>Knox Launcher ${u.latest} is out</b>
      <div class="muted small">You have ${S.state.version}. ${running ? "Close the game first." : inLobby() ? "Updating restarts the launcher, so you'll leave the lobby." : "It downloads, checks the SHA256 and restarts."}</div></div>
    <button class="btn ghost" onClick=${() => update({ updLater: u.latest })}>Later</button>
    ${u.page ? html`<button class="btn ghost" onClick=${() => rpc("openURL", { url: u.page })}>What's new</button>` : null}
    <button class="btn primary" disabled=${busy || u.busy || running} onClick=${run}>${busy || u.busy ? "Updating..." : "Update"}</button>
  </div>`;
}

// VersionLine is the version under the nav status, click to check now.
export function VersionLine() {
  const [checking, setChecking] = useState(false);
  const st = S.state || {};
  const u = st.update || {};
  const check = async () => {
    setChecking(true);
    const r = await act(() => rpc("checkUpdate"));
    setChecking(false);
    if (!r) return;
    update({ updLater: null });
    if (r.error) toast(r.error, true);
    else if (!r.available) toast("You're on the latest version");
  };
  return html`<div class="dim small stat-line click" title="Check for updates" onClick=${checking ? null : check}>
    v${st.version || ""} · ${checking ? "checking..." : u.available ? "update ready" : "check for updates"}
  </div>`;
}
