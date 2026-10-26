# Knox community server

A Cloudflare Worker that stores the stories and mod packs people upload from Knox Launcher.
Free plan is plenty (100k requests a day, 5 GB database).

Files: `worker.js` (the whole server), `schema.sql` (tables; the worker makes them itself on first use).

## Setup (about 10 minutes, all in the Cloudflare dashboard)

1. **Database.** Storage & databases > D1 SQL database > Create. Name it `knox`. Create.
2. **Worker.** Compute (Workers) > Workers & Pages > Create > Start with "Hello World". Name it `knox-community`. Deploy.
3. **Code.** On the new worker press Edit code. Delete everything in worker.js, paste in this folder's `worker.js`, press Deploy.
4. **Connect the database.** Worker > Settings > Bindings > Add > D1 database. Variable name `DB`, database `knox`. Save.
5. **Steam key.** Get one at https://steamcommunity.com/dev/apikey (any domain name works, e.g. the worker's address). Worker > Settings > Variables and Secrets > Add: type Secret, name `STEAM_KEY`, value = the key. Save.
6. **You as admin.** Same place, Add: type Text, name `ADMIN`, value = your SteamID64 (the 17-digit number on Steam > your profile > Account details). Save.
7. **Check it.** Open the worker's address (shown on the worker page, like `https://knox-community.yourname.workers.dev`). It should say `{"ok":true,...}`. Send that address so it can go into the launcher.

## What it does

- Anyone can read the list. Uploading, liking, commenting and reporting need a Steam sign-in: the launcher sends a Steam ticket and the server checks it with Steam (that's what the key is for). The Steam name is shown as the author.
- Limits: 20 uploads and 60 comments per person per day, 64 KB per upload, 8 tags.
- An upload reported by 5 different people is hidden.
- Admins (ADMIN) see Delete on every upload and comment, and Ban on other people's uploads. Ban deletes all their uploads and comments and stops them posting.

## Upkeep

- Mostly none. If something bad gets through, open it in the launcher and press Delete or Ban.
- To look at the data directly: D1 > knox > Console, e.g. `SELECT name, author, reports FROM items ORDER BY reports DESC`.
- To update the server, paste the new `worker.js` in Edit code and Deploy. Nothing else changes.
