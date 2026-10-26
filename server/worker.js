// Knox Launcher community server: stories and mod packs that players upload.
// Runs as a Cloudflare Worker with a D1 database bound as DB.
// Settings (Worker > Settings > Variables and Secrets):
//   STEAM_KEY  secret  your Steam Web API key, used to check who is uploading
//   ADMIN      text    your SteamID64 (comma separated for more than one); admins can delete anything and ban people

const LIMITS = { uploadsPerDay: 20, commentsPerDay: 60, dataBytes: 64000, tags: 8, tagLen: 20, name: 60, about: 400, comment: 300 };
const HIDE_AT_REPORTS = 5;

// The tables are made on first use, so there's nothing to run by hand.
const SCHEMA = [
  "CREATE TABLE IF NOT EXISTS items (id TEXT PRIMARY KEY, kind TEXT NOT NULL, name TEXT NOT NULL, author TEXT, author_id TEXT NOT NULL, about TEXT, tags TEXT, data TEXT, downloads INTEGER DEFAULT 0, likes INTEGER DEFAULT 0, comments INTEGER DEFAULT 0, reports INTEGER DEFAULT 0, hidden INTEGER DEFAULT 0, created INTEGER, updated INTEGER)",
  "CREATE INDEX IF NOT EXISTS items_kind ON items (kind, hidden, updated)",
  "CREATE INDEX IF NOT EXISTS items_author ON items (author_id, created)",
  "CREATE TABLE IF NOT EXISTS likes (item TEXT, user TEXT, PRIMARY KEY (item, user))",
  "CREATE TABLE IF NOT EXISTS downloads (item TEXT, user TEXT, PRIMARY KEY (item, user))",
  "CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY AUTOINCREMENT, item TEXT, author TEXT, author_id TEXT, text TEXT, created INTEGER)",
  "CREATE INDEX IF NOT EXISTS comments_item ON comments (item, created)",
  "CREATE TABLE IF NOT EXISTS reports (item TEXT, user TEXT, reason TEXT, created INTEGER, PRIMARY KEY (item, user))",
  "CREATE TABLE IF NOT EXISTS sessions (ticket TEXT PRIMARY KEY, user TEXT, name TEXT, expires INTEGER)",
  "CREATE TABLE IF NOT EXISTS bans (user TEXT PRIMARY KEY, created INTEGER)",
];
let ready = null;
function setup(env) {
  if (!ready) ready = env.DB.batch(SCHEMA.map((q) => env.DB.prepare(q))).catch((e) => { ready = null; throw e; });
  return ready;
}

export default {
  async fetch(req, env) {
    try {
      if (!env.DB) fail("the server has no database yet (add a D1 binding called DB)", 503);
      await setup(env);
      const url = new URL(req.url);
      const route = req.method + " " + url.pathname.replace(/\/+$/, "");
      const body = req.method === "POST" ? await req.json().catch(() => ({})) : {};
      switch (route) {
        case "GET ":
        case "GET /":
          return json({ ok: true, name: "Knox community", version: 1 });
        case "GET /list":
          return json(await list(env, req, url.searchParams.get("kind")));
        case "GET /comments":
          return json(await comments(env, url.searchParams.get("item")));
        case "POST /upload":
          return json(await upload(env, await who(env, req), body));
        case "POST /delete":
          return json(await remove(env, await who(env, req), body));
        case "POST /like":
          return json(await like(env, await who(env, req), body));
        case "POST /download":
          return json(await download(env, await who(env, req), body));
        case "POST /comment":
          return json(await comment(env, await who(env, req), body));
        case "POST /uncomment":
          return json(await uncomment(env, await who(env, req), body));
        case "POST /report":
          return json(await report(env, await who(env, req), body));
        case "POST /ban":
          return json(await ban(env, await who(env, req), body));
        case "GET /me":
          return json(await who(env, req));
      }
      return json({ error: "not found" }, 404);
    } catch (e) {
      return json({ error: e.message || String(e) }, e.status || 500);
    }
  },
};

function json(v, status = 200) {
  return new Response(JSON.stringify(v), { status, headers: { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*" } });
}

function fail(msg, status = 400) {
  const e = new Error(msg);
  e.status = status;
  throw e;
}

const now = () => Math.floor(Date.now() / 1000);
const isAdmin = (env, user) => !!user && String(env.ADMIN || "").split(",").map((s) => s.trim()).includes(user);

async function sha256(s) {
  const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// who checks the Steam ticket the launcher sends and returns {id, name}. Checked tickets are
// remembered for a day so Steam isn't asked on every click.
async function who(env, req, optional) {
  const ticket = req.headers.get("X-Knox-Ticket") || "";
  if (!ticket) {
    if (optional) return null;
    fail("sign in with Steam first", 401);
  }
  // local testing only: TEST_MODE=1 accepts "test:<steamid>:<name>" tickets
  let user, name;
  const key = await sha256(ticket);
  const s = env.TEST_MODE === "1" && ticket.startsWith("test:") ? { user: ticket.split(":")[1], name: ticket.split(":")[2] || "Tester" }
    : await env.DB.prepare("SELECT user, name FROM sessions WHERE ticket = ? AND expires > ?").bind(key, now()).first();
  if (s) {
    user = s.user;
    name = s.name;
  } else {
    if (!env.STEAM_KEY) fail("the server has no Steam key yet", 503);
    const identity = req.headers.get("X-Knox-Identity") || "knox";
    const r = await fetch(`https://api.steampowered.com/ISteamUserAuth/AuthenticateUserTicket/v1/?key=${env.STEAM_KEY}&appid=108600&ticket=${encodeURIComponent(ticket)}` + (identity === "-" ? "" : `&identity=${encodeURIComponent(identity)}`));
    const j = await r.json().catch(() => null);
    const p = j && j.response && j.response.params;
    if (!p || p.result !== "OK" || p.vacbanned || p.publisherbanned) {
      if (optional) return null;
      fail("Steam didn't accept your sign-in", 401);
    }
    user = p.steamid;
    name = "Survivor";
    const sr = await fetch(`https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/?key=${env.STEAM_KEY}&steamids=${user}`);
    const sj = await sr.json().catch(() => null);
    const pl = sj && sj.response && sj.response.players && sj.response.players[0];
    if (pl && pl.personaname) name = String(pl.personaname).slice(0, 40);
    await env.DB.prepare("INSERT OR REPLACE INTO sessions (ticket, user, name, expires) VALUES (?, ?, ?, ?)").bind(key, user, name, now() + 86400).run();
  }
  const banned = await env.DB.prepare("SELECT 1 FROM bans WHERE user = ?").bind(user).first();
  if (banned) {
    if (optional) return null;
    fail("you can't post here", 403);
  }
  return { id: user, name, admin: isAdmin(env, user) };
}

function cleanTags(tags) {
  const out = [];
  for (let t of Array.isArray(tags) ? tags : []) {
    t = String(t).toLowerCase().replace(/[^a-z0-9 +'&-]/g, "").replace(/\s+/g, " ").trim().slice(0, LIMITS.tagLen);
    if (t && !out.includes(t)) out.push(t);
    if (out.length >= LIMITS.tags) break;
  }
  return out;
}

const row = (r, likedSet) => ({
  id: r.id, kind: r.kind, name: r.name, author: r.author, authorId: r.author_id, about: r.about,
  tags: JSON.parse(r.tags || "[]"), data: JSON.parse(r.data || "null"),
  downloads: r.downloads, likes: r.likes, comments: r.comments, created: r.created, updated: r.updated,
  liked: likedSet ? likedSet.has(r.id) : false,
});

async function list(env, req, kind) {
  const me = await who(env, req, true).catch(() => null);
  const q = kind ? "SELECT * FROM items WHERE kind = ? AND hidden = 0 ORDER BY updated DESC LIMIT 1000" : "SELECT * FROM items WHERE hidden = 0 ORDER BY updated DESC LIMIT 1000";
  const st = kind ? env.DB.prepare(q).bind(kind) : env.DB.prepare(q);
  const { results } = await st.all();
  let liked = new Set();
  if (me) {
    const l = await env.DB.prepare("SELECT item FROM likes WHERE user = ?").bind(me.id).all();
    liked = new Set(l.results.map((x) => x.item));
  }
  return { items: results.map((r) => row(r, liked)), me };
}

async function upload(env, me, b) {
  const kind = b.kind === "pack" ? "pack" : b.kind === "story" ? "story" : fail("kind must be story or pack");
  const name = String(b.name || "").trim().slice(0, LIMITS.name);
  if (!name) fail("give it a name");
  const data = JSON.stringify(b.data ?? null);
  if (data.length > LIMITS.dataBytes) fail("that's too big to upload");
  const about = String(b.about || "").trim().slice(0, LIMITS.about);
  const tags = JSON.stringify(cleanTags(b.tags));
  const t = now();
  if (b.id) {
    const old = await env.DB.prepare("SELECT author_id FROM items WHERE id = ?").bind(b.id).first();
    if (!old) fail("that upload is gone", 404);
    if (old.author_id !== me.id && !me.admin) fail("that isn't yours", 403);
    await env.DB.prepare("UPDATE items SET name = ?, about = ?, tags = ?, data = ?, updated = ? WHERE id = ?").bind(name, about, tags, data, t, b.id).run();
    return { id: b.id };
  }
  const day = await env.DB.prepare("SELECT COUNT(*) AS n FROM items WHERE author_id = ? AND created > ?").bind(me.id, t - 86400).first();
  if (day.n >= LIMITS.uploadsPerDay && !me.admin) fail("that's enough uploads for today");
  const id = crypto.randomUUID().replace(/-/g, "").slice(0, 12);
  await env.DB.prepare("INSERT INTO items (id, kind, name, author, author_id, about, tags, data, downloads, likes, comments, reports, hidden, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, ?, ?)")
    .bind(id, kind, name, me.name, me.id, about, tags, data, t, t).run();
  return { id };
}

async function remove(env, me, b) {
  const it = await env.DB.prepare("SELECT author_id FROM items WHERE id = ?").bind(b.id || "").first();
  if (!it) return { ok: true };
  if (it.author_id !== me.id && !me.admin) fail("that isn't yours", 403);
  await env.DB.batch([
    env.DB.prepare("DELETE FROM items WHERE id = ?").bind(b.id),
    env.DB.prepare("DELETE FROM likes WHERE item = ?").bind(b.id),
    env.DB.prepare("DELETE FROM downloads WHERE item = ?").bind(b.id),
    env.DB.prepare("DELETE FROM comments WHERE item = ?").bind(b.id),
    env.DB.prepare("DELETE FROM reports WHERE item = ?").bind(b.id),
  ]);
  return { ok: true };
}

async function like(env, me, b) {
  const had = await env.DB.prepare("SELECT 1 FROM likes WHERE item = ? AND user = ?").bind(b.id || "", me.id).first();
  if (had) await env.DB.prepare("DELETE FROM likes WHERE item = ? AND user = ?").bind(b.id, me.id).run();
  else await env.DB.prepare("INSERT OR IGNORE INTO likes (item, user) SELECT id, ? FROM items WHERE id = ?").bind(me.id, b.id || "").run();
  await env.DB.prepare("UPDATE items SET likes = (SELECT COUNT(*) FROM likes WHERE item = ?) WHERE id = ?").bind(b.id, b.id).run();
  return { liked: !had };
}

async function download(env, me, b) {
  const r = await env.DB.prepare("INSERT OR IGNORE INTO downloads (item, user) SELECT id, ? FROM items WHERE id = ?").bind(me.id, b.id || "").run();
  if (r.meta && r.meta.changes) await env.DB.prepare("UPDATE items SET downloads = downloads + 1 WHERE id = ?").bind(b.id).run();
  return { ok: true };
}

async function comments(env, item) {
  const { results } = await env.DB.prepare("SELECT id, author, author_id, text, created FROM comments WHERE item = ? ORDER BY created DESC LIMIT 200").bind(item || "").all();
  return { comments: results.map((c) => ({ id: c.id, author: c.author, authorId: c.author_id, text: c.text, created: c.created })) };
}

async function comment(env, me, b) {
  const text = String(b.text || "").trim().slice(0, LIMITS.comment);
  if (!text) fail("write something first");
  const t = now();
  const day = await env.DB.prepare("SELECT COUNT(*) AS n FROM comments WHERE author_id = ? AND created > ?").bind(me.id, t - 86400).first();
  if (day.n >= LIMITS.commentsPerDay && !me.admin) fail("that's enough comments for today");
  const it = await env.DB.prepare("SELECT 1 FROM items WHERE id = ?").bind(b.id || "").first();
  if (!it) fail("that upload is gone", 404);
  await env.DB.prepare("INSERT INTO comments (item, author, author_id, text, created) VALUES (?, ?, ?, ?, ?)").bind(b.id, me.name, me.id, text, t).run();
  await env.DB.prepare("UPDATE items SET comments = (SELECT COUNT(*) FROM comments WHERE item = ?) WHERE id = ?").bind(b.id, b.id).run();
  return { ok: true };
}

async function uncomment(env, me, b) {
  const c = await env.DB.prepare("SELECT item, author_id FROM comments WHERE id = ?").bind(b.id || 0).first();
  if (!c) return { ok: true };
  if (c.author_id !== me.id && !me.admin) fail("that isn't yours", 403);
  await env.DB.prepare("DELETE FROM comments WHERE id = ?").bind(b.id).run();
  await env.DB.prepare("UPDATE items SET comments = (SELECT COUNT(*) FROM comments WHERE item = ?) WHERE id = ?").bind(c.item, c.item).run();
  return { ok: true };
}

async function report(env, me, b) {
  const reason = String(b.reason || "").slice(0, 200);
  await env.DB.prepare("INSERT OR IGNORE INTO reports (item, user, reason, created) SELECT id, ?, ?, ? FROM items WHERE id = ?").bind(me.id, reason, now(), b.id || "").run();
  await env.DB.prepare("UPDATE items SET reports = (SELECT COUNT(*) FROM reports WHERE item = ?), hidden = CASE WHEN (SELECT COUNT(*) FROM reports WHERE item = ?) >= ? THEN 1 ELSE hidden END WHERE id = ?")
    .bind(b.id, b.id, HIDE_AT_REPORTS, b.id).run();
  return { ok: true };
}

// ban (admins only) removes someone's uploads and comments and stops them posting.
async function ban(env, me, b) {
  if (!me.admin) fail("admins only", 403);
  const user = String(b.user || "");
  if (!user) fail("who?");
  await env.DB.batch([
    env.DB.prepare("INSERT OR IGNORE INTO bans (user, created) VALUES (?, ?)").bind(user, now()),
    env.DB.prepare("DELETE FROM items WHERE author_id = ?").bind(user),
    env.DB.prepare("DELETE FROM comments WHERE author_id = ?").bind(user),
  ]);
  return { ok: true };
}
