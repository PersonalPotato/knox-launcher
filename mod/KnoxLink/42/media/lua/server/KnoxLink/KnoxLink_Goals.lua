-- Knox Link goals, server side. Looks only: goals give Knox Launcher XP and badges, never anything in game.
-- Each session the squad gets one squad goal and everyone gets a personal one. A world also has long goals
-- that carry over between sessions, and a story can add its own goal. The clients count what they did
-- (KnoxLink_Track.lua) and send it here; the server keeps the totals and decides when a goal is done.
-- Goals scale with the world's difficulty: easy worlds ask for more, hard ones ask for less and pay more.
if not isServer() then return end
require "KnoxLink/KnoxLink_Json"

KnoxGoals = {}

local MODULE = "KnoxLink"
local TIER = { easy = { need = 1.5, xp = 0.75 }, normal = { need = 1, xp = 1 }, hard = { need = 0.7, xp = 1.5 } }

-- what = the track counter (or a special rule), scaled = the count grows with the tier
local SQUAD = {
    { id = "generator", name = "Lights On", text = "Get a generator running", what = "gen", need = 1, xp = 150 },
    { id = "kills200", name = "Body Count", text = "Kill %d zombies together", what = "kills", need = 200, xp = 150, scaled = true },
    { id = "church", name = "Sanctuary", text = "Spend a night in a church", what = "church", need = 1, xp = 200 },
    { id = "tank", name = "Fill Her Up", text = "Fill a car's tank", what = "tank", need = 1, xp = 100 },
    { id = "garden", name = "Victory Garden", text = "Plant %d seeds", what = "planted", need = 10, xp = 150, scaled = true },
    { id = "nodeath", name = "Clean Sheet", text = "%d days with nobody dying", what = "nodeath", need = 3, xp = 250, scaled = true },
}
local PERSONAL = {
    { id = "blunt", name = "Batter Up", text = "%d kills with a blunt weapon", what = "blunt", need = 25, xp = 100, scaled = true },
    { id = "cook", name = "Home Cooking", text = "Cook a meal", what = "cooked", need = 1, xp = 50 },
    { id = "book", name = "Bookworm", text = "Read a whole book", what = "books", need = 1, xp = 100 },
    { id = "medic", name = "Patch Job", text = "Bandage a squad mate", what = "bandOthers", need = 1, xp = 75, squadOnly = true },
    { id = "drive", name = "Road Trip", text = "Drive %d km", what = "drive", need = 5, xp = 100, scaled = true, unit = 1000 },
    { id = "gun", name = "Packing Heat", text = "Find a gun", what = "gun", need = 1, xp = 75 },
    { id = "skill", name = "Fast Learner", text = "Get a skill to level 3", what = "skill3", need = 1, xp = 150 },
}
local LONG = {
    { id = "longhaul", name = "Long Haul", text = "Reach day %d", what = "day", need = 30, xp = 300, scaled = true },
    { id = "thousand", name = "Thousand Club", text = "Kill %d zombies on this world", what = "worldKills", need = 1000, xp = 300, scaled = true },
}
local STORY = {
    prisonbreak = { id = "story:prisonbreak", name = "Jailbreak", text = "Get out of the prison alive", what = "escape", need = 1, xp = 200 },
    ["ehe-evac"] = { id = "story:ehe-evac", name = "Made the Evac", text = "Reach the military base", what = "military", need = 1, xp = 200 },
    farmhands = { id = "story:farmhands", name = "Green Thumb", text = "Harvest 20 crops", what = "harvested", need = 20, xp = 200 },
    firecrew = { id = "story:firecrew", name = "Fire Crew", text = "Put out a fire", what = "fireOut", need = 1, xp = 200 },
}

local data, session = nil, nil

local function safe(f)
    local ok, r = pcall(f)
    if ok then return r end
    return nil
end

local function today()
    return safe(function() return getGameTime():getNightsSurvived() end) or 0
end

local function tier() return TIER[data.difficulty or "normal"] or TIER.normal end

-- make: a goal from a definition, scaled to the world
local function make(def, kind, user)
    local t = tier()
    local need = def.need
    if def.scaled then need = math.max(1, math.floor(need * t.need + 0.5)) end
    local g = { id = def.id, name = def.name, text = string.format(def.text, need), what = def.what, need = need * (def.unit or 1),
        show = need, unit = def.unit or 1, xp = math.floor(def.xp * t.xp / 5 + 0.5) * 5, kind = kind, have = 0, done = false, user = user }
    return g
end

local function pick(list, seed, skip)
    local ok = {}
    for _, d in ipairs(list) do if not (skip and skip(d)) then table.insert(ok, d) end end
    if #ok == 0 then return nil end
    return ok[(seed % #ok) + 1]
end

local function count(user, what)
    local t = session.track[user] or {}
    return t[what] or 0
end

local function sum(what)
    local n = 0
    for _, t in pairs(session.track) do n = n + (t[what] or 0) end
    return n
end

local function players()
    local names = {}
    local list = getOnlinePlayers()
    if list then
        for i = 0, list:size() - 1 do table.insert(names, list:get(i):getUsername()) end
    end
    return names
end

-- credit: the players a finished goal goes to
local function finish(g, users)
    if g.done then return end
    g.done = true
    g.have = g.need
    g.by = users
    g.day = today()
    for _, u in ipairs(users) do
        session.done[u] = session.done[u] or {}
        table.insert(session.done[u], { id = g.id, name = g.name, xp = g.xp })
    end
    sendServerCommand(MODULE, "goal", { name = g.name, text = g.text, xp = g.xp, who = g.user or "" })
    print("[KnoxLink] goal done: " .. g.name .. " (" .. table.concat(users, ", ") .. ")")
end

function KnoxGoals.init(d)
    data = d
    data.goalsLong = data.goalsLong or {}
    data.goalsDone = data.goalsDone or {}
    data.track = data.track or {}
    data.lastDeathDay = data.lastDeathDay or today()
    session = { track = {}, done = {}, personal = {}, startDay = today(), kills = {} }
    -- long goals carry over until done; add the ones this world doesn't have yet
    for _, def in ipairs(LONG) do
        if not data.goalsDone[def.id] and not data.goalsLong[def.id] then data.goalsLong[def.id] = make(def, "long") end
    end
    local sd = STORY[data.story or ""]
    if sd and not data.goalsDone[sd.id] then
        -- keep the progress of this story's goal, but a different story gets its own
        if not data.goalsStory or data.goalsStory.id ~= sd.id then data.goalsStory = make(sd, "story") end
    else
        data.goalsStory = nil
    end
    local seed = ZombRand(1000)
    local squadDef = pick(SQUAD, seed)
    session.squad = make(squadDef, "squad")
    print("[KnoxLink] goals: " .. session.squad.name .. " (" .. (data.difficulty or "normal") .. ")")
end

-- personal goal for someone joining this session (once each)
function KnoxGoals.meet(user)
    if not session or session.personal[user] then return end
    local solo = #players() < 2
    local seed = ZombRand(1000)
    local def = pick(PERSONAL, seed, function(d) return d.squadOnly and solo end)
    if def then session.personal[user] = make(def, "personal", user) end
end

-- track: one client's counts since its last report
function KnoxGoals.track(user, args)
    if not session or type(args) ~= "table" then return end
    local s = session.track[user] or {}
    local w = data.track[user] or {}
    for k, v in pairs(args) do
        v = tonumber(v)
        if v and v > 0 and v < 100000 then
            if k == "gun" or k == "skill3" then
                s[k] = math.max(s[k] or 0, v)
                w[k] = math.max(w[k] or 0, v)
            else
                s[k] = (s[k] or 0) + v
                w[k] = (w[k] or 0) + v
            end
        end
    end
    session.track[user] = s
    data.track[user] = w
end

function KnoxGoals.died() if data then data.lastDeathDay = today() end end

-- kills: session kills per player (from the status writer), counted for the squad goal
function KnoxGoals.kills(user, n) if session then session.kills[user] = n end end

-- the military base check for the evac story: someone standing in an army room
-- escape: someone alive far from the start building
function KnoxGoals.position(user, x, y, dead)
    if not data or dead then return end
    local g = data.goalsStory
    if not g or g.done then return end
    if g.what == "escape" and data.base then
        local dx, dy = x - data.base.x, y - data.base.y
        if dx * dx + dy * dy > 150 * 150 then
            local t = session.track[user] or {}
            t.escape = 1
            session.track[user] = t
        end
    end
end

local function progress(g)
    local w = g.what
    if w == "kills" then
        local n = 0
        for _, k in pairs(session.kills) do n = n + k end
        return n
    elseif w == "nodeath" then
        return math.max(0, today() - math.max(session.startDay, data.lastDeathDay or 0))
    elseif w == "day" then
        return today()
    elseif w == "worldKills" then
        local n = 0
        for _, c in pairs(data.cast or {}) do n = n + (c.kills or 0) end
        return n
    elseif g.kind == "personal" then
        return count(g.user, w)
    end
    return sum(w)
end

-- tick: check every goal (called with the status files)
function KnoxGoals.tick()
    if not session then return end
    local online = players()
    local g = session.squad
    if g and not g.done then
        g.have = math.min(g.need, progress(g))
        if g.have >= g.need then finish(g, online) end
    end
    for user, pg in pairs(session.personal) do
        if not pg.done then
            pg.have = math.min(pg.need, progress(pg))
            if pg.have >= pg.need then finish(pg, { user }) end
        end
    end
    for id, lg in pairs(data.goalsLong) do
        if not lg.done then
            lg.have = math.min(lg.need, progress(lg))
            if lg.have >= lg.need then
                finish(lg, online)
                data.goalsDone[id] = true
                data.goalsLong[id] = nil
            end
        end
    end
    local sg = data.goalsStory
    if sg and not sg.done then
        sg.have = math.min(sg.need, progress(sg))
        if sg.have >= sg.need then
            finish(sg, online)
            data.goalsDone[sg.id] = true
        end
    end
end

local function out(g)
    if not g then return nil end
    return { id = g.id, name = g.name, text = g.text, kind = g.kind, user = g.user or "", xp = g.xp, done = g.done == true,
        have = math.floor((g.have or 0) / (g.unit or 1)), need = g.show or g.need }
end

-- state: everything for app_goals.json
function KnoxGoals.state()
    if not session then return nil end
    local list = KnoxLinkJson.array()
    table.insert(list, out(session.squad))
    for _, pg in pairs(session.personal) do table.insert(list, out(pg)) end
    for _, lg in pairs(data.goalsLong) do table.insert(list, out(lg)) end
    if data.goalsStory then table.insert(list, out(data.goalsStory)) end
    return { difficulty = data.difficulty or "normal", story = data.story or "", day = today(), goals = list }
end

-- forPlayer: what a player finished and counted this session (for app_recap.json)
function KnoxGoals.forPlayer(user)
    if not session then return nil, nil end
    local done = KnoxLinkJson.array()
    for _, d in ipairs(session.done[user] or {}) do table.insert(done, d) end
    local track = {}
    for k, v in pairs(session.track[user] or {}) do track[k] = math.floor(v * 10 + 0.5) / 10 end
    return done, track
end
