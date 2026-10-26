// The Knox Country map (the game's own map pictures) with the start building pinned.
import { html, useEffect, useRef, S, TOKEN, rpc, act, toast, update, placeText } from "./core.js";

const tiles = {};
let mapInfo = null;
const view = { cx: 10800, cy: 9700, scale: 0.12 };

export function SpawnMap({ spot, canPick, height }) {
  const wrap = useRef(null);
  const canvas = useRef(null);
  const draw = useRef(() => {});
  const lastSpot = useRef(null);

  useEffect(() => {
    const cv = canvas.current, el = wrap.current;
    const ctx = cv.getContext("2d");
    let drag = null;
    const toScreen = (wx, wy) => [(wx - view.cx) * view.scale + cv.width / 2, (wy - view.cy) * view.scale + cv.height / 2];
    const toWorld = (sx, sy) => [(sx - cv.width / 2) / view.scale + view.cx, (sy - cv.height / 2) / view.scale + view.cy];
    const tile = (z, x, y) => {
      const key = `${z}/${x}/${y}`;
      let t = tiles[key];
      if (!t) {
        t = tiles[key] = new Image();
        t.onload = () => draw.current();
        t.onerror = () => { t.bad = true; };
        t.src = `/tile/${key}.png?t=${TOKEN}`;
      }
      return t;
    };
    draw.current = () => {
      if (!cv.isConnected) return;
      ctx.fillStyle = "#0b0c0d";
      ctx.fillRect(0, 0, cv.width, cv.height);
      const levels = (mapInfo && mapInfo.levels) || 5;
      const z = Math.max(0, Math.min(levels - 1, Math.floor(Math.log2(1 / view.scale))));
      const span = 256 * Math.pow(2, z);
      const [x0, y0] = toWorld(0, 0), [x1, y1] = toWorld(cv.width, cv.height);
      ctx.imageSmoothingEnabled = view.scale < 2;
      for (let ty = Math.max(0, Math.floor(y0 / span)); ty <= Math.floor(y1 / span); ty++) {
        for (let tx = Math.max(0, Math.floor(x0 / span)); tx <= Math.floor(x1 / span); tx++) {
          const img = tile(z, tx, ty);
          if (img.complete && !img.bad && img.naturalWidth) {
            const [sx, sy] = toScreen(tx * span, ty * span);
            ctx.drawImage(img, sx, sy, span * view.scale + 0.5, span * view.scale + 0.5);
          }
        }
      }
      // town labels
      if (S.cat && view.scale < 0.6) {
        ctx.font = "600 12px Segoe UI, sans-serif";
        ctx.textAlign = "center";
        for (const r of (S.presets && S.presets.towns) || S.cat.spawns || []) {
          if (!r.points.length) continue;
          const mx = r.points.reduce((a, p) => a + p.x, 0) / r.points.length, my = r.points.reduce((a, p) => a + p.y, 0) / r.points.length;
          const [sx, sy] = toScreen(mx, my);
          ctx.fillStyle = "rgba(0,0,0,.6)";
          ctx.fillText(r.name.replace(", KY", ""), sx + 1, sy + 1);
          ctx.fillStyle = "#f3e7d3";
          ctx.fillText(r.name.replace(", KY", ""), sx, sy);
        }
      }
      const sp = lastSpot.current;
      if (sp) {
        if (sp.points && sp.points.length) {
          ctx.beginPath();
          sp.points.forEach(([px, py], i) => { const [sx, sy] = toScreen(px, py); if (i) ctx.lineTo(sx, sy); else ctx.moveTo(sx, sy); });
          ctx.closePath();
          ctx.fillStyle = "rgba(217,103,43,.3)"; ctx.fill();
          ctx.strokeStyle = "#f08a4b"; ctx.lineWidth = 2; ctx.stroke();
        }
        const [sx, sy] = toScreen(sp.x + 0.5, sp.y + 0.5);
        ctx.fillStyle = "#f08a4b"; ctx.strokeStyle = "#1b0c03"; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.arc(sx, sy - 16, 10, 0, Math.PI * 2); ctx.fill(); ctx.stroke();
        ctx.beginPath(); ctx.moveTo(sx - 7, sy - 10); ctx.lineTo(sx, sy); ctx.lineTo(sx + 7, sy - 10); ctx.fill();
        ctx.fillStyle = "#1b0c03"; ctx.beginPath(); ctx.arc(sx, sy - 16, 3.5, 0, Math.PI * 2); ctx.fill();
      }
    };
    const resize = () => { cv.width = el.clientWidth; cv.height = el.clientHeight; draw.current(); };
    const ro = new ResizeObserver(resize);
    ro.observe(el);
    const wheel = (e) => {
      e.preventDefault();
      const [wx, wy] = toWorld(e.offsetX, e.offsetY);
      view.scale = Math.max(0.025, Math.min(6, view.scale * (e.deltaY < 0 ? 1.25 : 0.8)));
      const [nx, ny] = toWorld(e.offsetX, e.offsetY);
      view.cx += wx - nx; view.cy += wy - ny;
      draw.current();
    };
    const down = (e) => { drag = { x: e.clientX, y: e.clientY, cx: view.cx, cy: view.cy, moved: false }; };
    const move = (e) => {
      if (!drag) return;
      const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
      if (Math.abs(dx) + Math.abs(dy) > 4) drag.moved = true;
      view.cx = drag.cx - dx / view.scale; view.cy = drag.cy - dy / view.scale;
      draw.current();
    };
    const up = async (e) => {
      const d = drag; drag = null;
      if (!d || d.moved || e.target !== cv || !canPickRef.current) return;
      const [wx, wy] = toWorld(e.offsetX, e.offsetY);
      const r = await act(() => rpc("mapPick", { x: Math.floor(wx), y: Math.floor(wy) }));
      if (r && r.ok) {
        Object.assign(S.cfg.host, { spawn: r.spot, spawnMode: "pick", together: true });
        update();
        toast("Everyone starts here: " + placeText(r.spot));
      } else if (r) toast("No building there. Click on a house.");
    };
    cv.addEventListener("wheel", wheel, { passive: false });
    cv.addEventListener("mousedown", down);
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
    if (!mapInfo) act(() => rpc("mapInfo")).then((i) => { mapInfo = i; draw.current(); });
    return () => {
      ro.disconnect();
      cv.removeEventListener("wheel", wheel);
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
  }, []);

  const canPickRef = useRef(canPick);
  canPickRef.current = canPick;

  // move to the pin whenever it changes (a new random house, a pick, a story)
  useEffect(() => {
    const prev = lastSpot.current;
    lastSpot.current = spot;
    if (spot && (!prev || prev.x !== spot.x || prev.y !== spot.y)) {
      view.cx = spot.x; view.cy = spot.y;
      if (view.scale < 0.5) view.scale = 0.9;
    }
    draw.current();
  }, [spot && spot.x, spot && spot.y]);

  return html`<div class="map-wrap" ref=${wrap} style=${height ? { height: height + "px" } : null}><canvas ref=${canvas} style=${{ cursor: canPick ? "crosshair" : "grab" }}></canvas>
    <div class="map-help">Drag to move, scroll to zoom${canPick ? ", click a building to spawn there" : ""}</div></div>`;
}
