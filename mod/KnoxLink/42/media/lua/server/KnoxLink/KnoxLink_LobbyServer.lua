-- Knox Link lobby, server side.
-- The host writes Zomboid/Lua/KnoxLink/lobby.txt right before it starts the server:
--   line 1: "1" or "0" (lobby on), line 2: spawn town, line 3: players to wait for (Knox Launcher),
--   line 4: starting kit "Base.Item:count;Base.Other:count"
--   line 5: respawn rule (base | squad | random), line 6: start building "x,y,z",
--   line 7: houses for random respawns "x,y,z;x,y,z;..."
--   line 8: lives (0 = unlimited), line 9: "each" or "squad" (one shared pool of spare lives),
--   line 10: world name, line 11: goal tier (easy | normal | hard), line 12: story id
-- We read it once at startup and clear it.
-- While running, the server also writes app_status.json, app_recap.json and app_cast.json for the Knox Launcher.
-- The cast (everyone who ever played this world, alive or dead, lives used) is kept in the world's ModData.
if not isServer() then return end
require "KnoxLink/KnoxLink_Json"
require "KnoxLink/KnoxLink_Goals"

local MODULE = "KnoxLink"
local COUNTDOWN_MS = 5000
local STATUS_MS = 4000
local data = nil
local present = {} -- username -> true once their character is in the world
local ready = {}   -- username -> true
local countdownEnd = nil

local function safe(f)
    local ok, r = pcall(f)
    if ok then return r end
    return nil
end

local function readFlag()
    local reader = getFileReader("KnoxLink/lobby.txt", false)
    if not reader then return false, "", 0, "", nil end
    local open = reader:readLine()
    local town = reader:readLine()
    local expected = reader:readLine()
    local kit = reader:readLine()
    local respawn = { rule = reader:readLine(), base = reader:readLine(), spots = reader:readLine(),
        lives = reader:readLine(), livesMode = reader:readLine(), world = reader:readLine(),
        difficulty = reader:readLine(), story = reader:readLine() }
    reader:close()
    return open ~= nil and open:gsub("%s", "") == "1", town or "", tonumber(expected or "") or 0, kit or "", respawn
end

local function parseSpot(text)
    if not text then return nil end
    local x, y, z = string.match(text, "^%s*(-?%d+)%s*,%s*(-?%d+)%s*,%s*(-?%d+)")
    x, y, z = tonumber(x), tonumber(y), tonumber(z)
    if not x or (x == 0 and y == 0) then return nil end
    return { x = x, y = y, z = z }
end

local function clearFlag()
    local writer = getFileWriter("KnoxLink/lobby.txt", true, false)
    if writer then
        writer:write("0\r\n")
        writer:close()
    end
end

local function onlineNames()
    local names = {}
    local players = getOnlinePlayers()
    if players then
        for i = 0, players:size() - 1 do
            local name = players:get(i):getUsername()
            if present[name] then table.insert(names, name) end
        end
    end
    table.sort(names)
    return names
end

local function stateArgs()
    local names = onlineNames()
    local readyNames = {}
    for _, name in ipairs(names) do
        if ready[name] or name == data.host then table.insert(readyNames, name) end
    end
    return {
        open = data.open == true, host = data.host or "", town = data.town or "",
        players = names, ready = readyNames, expected = data.expected or 0,
    }
end

local function broadcast()
    if data then sendServerCommand(MODULE, "state", stateArgs()) end
end

local function startCountdown()
    if countdownEnd then return end
    countdownEnd = getTimestampMs() + COUNTDOWN_MS
    sendServerCommand(MODULE, "countdown", { ms = COUNTDOWN_MS })
end

-- Starting kit, once per character (a new character after a death gets it again)
local function mark(player, key)
    player:getModData()[key] = true
    sendServerCommand(player, MODULE, "mark", { key = key })
end

local function giveKit(player)
    if not data.kit or data.kit == "" then return end
    if player:getModData().knoxKit or (safe(function() return player:getHoursSurvived() end) or 0) > 1 then return end
    mark(player, "knoxKit")
    local user = player:getUsername()
    local inv = player:getInventory()
    for _, entry in ipairs(KnoxLinkJson.split(data.kit, ";")) do
        local colon = string.find(entry, ":", 1, true)
        local itemType, count = entry, 1
        if colon then
            itemType = string.sub(entry, 1, colon - 1)
            count = tonumber(string.sub(entry, colon + 1)) or 1
        end
        if itemType ~= "" then
            for _ = 1, math.min(count, 50) do
                local item = safe(function() return inv:AddItem(itemType) end)
                if item then pcall(sendAddItemToContainer, inv, item) end
            end
        end
    end
    print("[KnoxLink] starting kit given to " .. user)
end

Events.OnInitGlobalModData.Add(function()
    data = ModData.getOrCreate("KnoxLinkLobby")
    local open, town, expected, kit, respawn = readFlag()
    local fromLauncher = respawn and respawn.rule and respawn.rule ~= ""
    if open then
        data.open = true
        data.town = town
        data.expected = expected
    elseif fromLauncher then
        -- continuing a world: no lobby, even if the last session closed before its countdown
        data.open = false
    end
    -- the launcher always sends the kit line: an empty one means no starting kit any more
    if kit ~= "" or fromLauncher then data.kit = kit end
    if fromLauncher then
        data.respawn = respawn.rule:gsub("%s", "")
        data.base = parseSpot(respawn.base) or data.base
        local spots = {}
        for _, part in ipairs(KnoxLinkJson.split(respawn.spots or "", ";")) do
            local spot = parseSpot(part)
            if spot then table.insert(spots, spot) end
        end
        if #spots > 0 then data.spots = spots end
        local lives = tonumber(respawn.lives or "")
        if lives then data.lives = math.max(0, math.floor(lives)) end
        if respawn.livesMode and respawn.livesMode ~= "" then data.livesMode = respawn.livesMode:gsub("%s", "") end
        if respawn.world and respawn.world ~= "" then data.world = respawn.world end
        if respawn.difficulty and respawn.difficulty ~= "" then data.difficulty = respawn.difficulty:gsub("%s", "") end
        if respawn.story then data.story = respawn.story:gsub("%s", "") end
    end
    data.cast = data.cast or {}
    local gok, gerr = pcall(KnoxGoals.init, data)
    if not gok then print("[KnoxLink] goals error: " .. tostring(gerr)) end
    clearFlag()
    print("[KnoxLink] lobby " .. (data.open and "open" or "off") .. (expected > 0 and (", waiting for " .. expected .. " players") or ""))
end)

-- The cast ------------------------------------------------------------------------
-- data.cast[username] = { display, alive, out, deaths, chars, banked (kills of dead characters), kills,
--   hours, firstDay, lastDay, lastSeen, died = { {day, name, hours, kills}, ... } }

local function today()
    return safe(function() return getGameTime():getNightsSurvived() end) or 0
end

local function totalDeaths()
    local n = 0
    for _, c in pairs(data.cast) do n = n + (c.deaths or 0) end
    return n
end

-- out of lives: "each" = you have N characters, "squad" = the squad has N spare lives between them
local function outOfLives(c)
    local lives = data.lives or 0
    if lives <= 0 then return false end
    if data.livesMode == "squad" then return totalDeaths() > lives end
    return (c.deaths or 0) >= lives
end

local function livesLeft(c)
    local lives = data.lives or 0
    if lives <= 0 then return -1 end
    if data.livesMode == "squad" then return math.max(0, lives - totalDeaths()) end
    return math.max(0, lives - (c.deaths or 0))
end

local function castEntry(user)
    local c = data.cast[user]
    if not c then
        c = { display = user, alive = false, out = false, deaths = 0, chars = 0, banked = 0, kills = 0, hours = 0,
            firstDay = today(), lastDay = today(), died = {} }
        data.cast[user] = c
    end
    c.died = c.died or {}
    return c
end

local function markDead(user, player)
    local c = castEntry(user)
    if not c.alive then return end
    local kills, hours = 0, c.hours or 0
    if player then
        kills = safe(function() return player:getZombieKills() end) or 0
        hours = safe(function() return player:getHoursSurvived() end) or hours
        pcall(function() player:getModData().knoxDead = true end)
    end
    c.alive = false
    c.deaths = (c.deaths or 0) + 1
    c.banked = (c.banked or 0) + kills
    c.kills = c.banked
    table.insert(c.died, { day = today(), name = c.display or user, hours = hours, kills = kills })
    pcall(KnoxGoals.died)
    print("[KnoxLink] " .. user .. " died (" .. c.deaths .. " deaths)")
end

local function tapeEnded()
    local any = false
    for _, c in pairs(data.cast) do
        any = true
        if c.alive or not outOfLives(c) then return false end
    end
    return any
end

local function sendProtect(name)
    -- only the host's game can run admin commands, so it hides out-of-lives players
    local players = getOnlinePlayers()
    if not players then return end
    for i = 0, players:size() - 1 do
        local p = players:get(i)
        if p:getUsername() == data.host then sendServerCommand(p, MODULE, "protect", { name = name }) end
    end
end

local function sendOut(player, c)
    sendServerCommand(player, MODULE, "out", { name = c.display or player:getUsername(), deaths = c.deaths or 0,
        ended = tapeEnded() })
    sendProtect(player:getUsername())
end

local function squadSpot(player)
    local mates = {}
    local players = getOnlinePlayers()
    if players then
        for i = 0, players:size() - 1 do
            local p = players:get(i)
            local c = data.cast[p:getUsername()]
            if p ~= player and p:getUsername() ~= player:getUsername() and not (safe(function() return p:isDead() end))
                and not (c and c.out) then
                table.insert(mates, p)
            end
        end
    end
    if #mates == 0 then return nil end
    local mate = mates[ZombRand(#mates) + 1]
    return { x = math.floor(mate:getX()), y = math.floor(mate:getY()), z = math.floor(mate:getZ()) },
        tostring(safe(function() return mate:getDisplayName() end) or mate:getUsername())
end

-- Respawn rule: a player who already had a character here and now has a brand new one.
local function applyRespawn(player)
    local rule = data.respawn or "base"
    local spot, text = nil, nil
    if rule == "squad" then
        local mate
        spot, mate = squadSpot(player)
        if spot then
            text = "Back with the squad: you're next to " .. mate
        else
            spot, text = data.base, "Nobody's alive, so you're back at the start"
        end
    elseif rule == "random" and data.spots and #data.spots > 0 then
        spot, text = data.spots[ZombRand(#data.spots) + 1], "A new life somewhere else in Knox Country"
    elseif rule == "base" then
        spot, text = data.base, "Back where it all started"
    end
    return spot, text
end

-- Someone joining a world that's already going starts next to the squad.
local function applyLateJoin(player)
    local spot, mate = squadSpot(player)
    if spot then return spot, "Joining the squad: you're next to " .. mate end
    return nil, nil
end

-- Flags live on the character itself. The client saves its own character in MP, so it sets them too.
-- Called on every "hello": works out if this is a returning character, a new life or a newcomer.
local function meetCharacter(player, args)
    local user = player:getUsername()
    local md = {}
    for k, v in pairs(player:getModData()) do md[k] = v end
    for _, k in ipairs({ "knoxChar", "knoxOut", "knoxKit" }) do
        if args and args[k] then md[k] = true end
    end
    local hours = safe(function() return player:getHoursSurvived() end) or 0
    local c = castEntry(user)
    c.display = tostring(safe(function() return player:getDisplayName() end) or user)
    c.lastSeen = getTimestamp()
    if md.knoxOut then
        c.out = true
        sendOut(player, c)
        return
    end
    if md.knoxChar then
        c.alive = not (safe(function() return player:isDead() end) or false)
        return
    end
    -- a character we haven't seen before. One that has played a while is from before Knox Link tracked the cast.
    if hours > 0.25 then
        mark(player, "knoxChar")
        if c.chars == 0 then c.chars = 1 end
        c.alive = true
        return
    end
    if c.alive and c.chars > 0 then markDead(user, nil) end -- died while we weren't looking
    local returning = c.chars > 0
    if returning and outOfLives(c) then
        mark(player, "knoxOut")
        c.out = true
        print("[KnoxLink] " .. user .. " is out of lives")
        sendOut(player, c)
        return
    end
    mark(player, "knoxChar")
    c.chars = c.chars + 1
    c.alive = true
    c.out = false
    if not md.knoxKit then pcall(giveKit, player) end
    local spot, text
    if returning then
        spot, text = applyRespawn(player)
        local left = livesLeft(c)
        if left >= 0 then text = (text or "New life") .. " (" .. (left == 0 and "last life" or (left .. " lives left")) .. ")" end
    elseif data.begun and not data.open then
        spot, text = applyLateJoin(player)
    end
    data.begun = data.begun or not data.open
    if spot then
        sendServerCommand(player, MODULE, "respawn", { x = spot.x, y = spot.y, z = spot.z or 0, text = text })
        print("[KnoxLink] placed " .. user .. ": " .. tostring(text))
    elseif text then
        sendServerCommand(player, MODULE, "toast", { text = text })
    end
end

Events.OnClientCommand.Add(function(module, command, player, args)
    if module ~= MODULE or not data then return end
    local name = player:getUsername()
    if command == "hello" then
        present[name] = true
        if args and args.isHost then data.host = name end
        local ok, err = pcall(meetCharacter, player, args)
        if not ok then print("[KnoxLink] cast error: " .. tostring(err)) end
        pcall(KnoxGoals.meet, name)
        broadcast()
    elseif command == "track" then
        -- what this player did since their last report (goals and launcher stats)
        pcall(KnoxGoals.track, name, args)
    elseif command == "look" then
        -- what this survivor is wearing now; the launcher draws the cast with it
        local c = data.cast and data.cast[name]
        if c and args then c.look = args end
    elseif command == "ready" then
        ready[name] = args and args.ready == true
        broadcast()
    elseif command == "start" then
        if data.open and name == data.host then startCountdown() end
    elseif command == "cancel" then
        if name == data.host and countdownEnd then
            countdownEnd = nil
            sendServerCommand(MODULE, "cancelled", {})
        end
    end
end)

-- Status + session recap for the app ------------------------------------------

local session = { started = getTimestamp(), players = {} }

local function writeCast(day)
    local list = KnoxLinkJson.array()
    for user, c in pairs(data.cast) do
        local died = KnoxLinkJson.array()
        for _, d in ipairs(c.died or {}) do table.insert(died, d) end
        table.insert(list, { user = user, name = c.display or user, alive = c.alive == true, out = c.out == true,
            deaths = c.deaths or 0, livesLeft = livesLeft(c), kills = c.kills or 0, hours = c.hours or 0,
            firstDay = c.firstDay or 0, lastDay = c.lastDay or 0, lastSeen = c.lastSeen or 0, died = died, look = c.look })
    end
    local ended = tapeEnded()
    if ended and not data.ended then
        data.ended = true
        sendServerCommand(MODULE, "ended", {})
        print("[KnoxLink] everyone is out: THE END")
    end
    KnoxLinkJson.write("app_cast.json", { world = data.world or "", updated = getTimestamp(), day = day,
        lives = data.lives or 0, livesMode = data.livesMode or "each", ended = ended, cast = list })
end
local nextStatus = 0

local function writeStatus()
    local list = KnoxLinkJson.array()
    local players = getOnlinePlayers()
    if players then
        for i = 0, players:size() - 1 do
            local p = players:get(i)
            local user = p:getUsername()
            local kills = safe(function() return p:getZombieKills() end) or 0
            local dead = safe(function() return p:isDead() end) or false
            local rec = session.players[user]
            if not rec then
                rec = { name = user, startKills = kills, kills = 0, deaths = 0, wasDead = dead, hours = 0, firstSeen = getTimestamp() }
                session.players[user] = rec
            end
            if kills < rec.startKills then rec.startKills = kills end -- new character after a death
            rec.kills = math.max(rec.kills, kills - rec.startKills)
            if dead and not rec.wasDead then rec.deaths = rec.deaths + 1 end
            rec.wasDead = dead
            local c = data and data.cast and data.cast[user]
            if c then
                if dead then
                    if c.alive and not (safe(function() return p:getModData().knoxDead end)) then markDead(user, p) end
                elseif c.alive then
                    c.kills = (c.banked or 0) + kills
                    c.hours = safe(function() return p:getHoursSurvived() end) or c.hours
                    c.lastDay = today()
                    c.lastSeen = getTimestamp()
                end
            end
            rec.hours = safe(function() return p:getHoursSurvived() end) or rec.hours
            rec.lastSeen = getTimestamp()
            pcall(KnoxGoals.kills, user, rec.kills)
            pcall(KnoxGoals.position, user, p:getX(), p:getY(), dead)
            table.insert(list, {
                name = user,
                display = tostring(safe(function() return p:getDisplayName() end) or user),
                health = safe(function() return p:getBodyDamage():getOverallBodyHealth() end) or 0,
                kills = kills,
                hours = rec.hours,
                dead = dead,
                x = math.floor(p:getX()), y = math.floor(p:getY()), z = math.floor(p:getZ()),
            })
        end
    end
    local gt = getGameTime()
    local day = safe(function() return gt:getNightsSurvived() end) or 0
    KnoxLinkJson.write("app_status.json", {
        time = getTimestamp(),
        day = day,
        hour = safe(function() return gt:getHour() end) or 0,
        lobby = data and data.open and true or false,
        players = list,
    })
    local recap = KnoxLinkJson.array()
    pcall(KnoxGoals.tick)
    for _, rec in pairs(session.players) do
        local ok, goals, track = pcall(KnoxGoals.forPlayer, rec.name)
        table.insert(recap, { name = rec.name, kills = rec.kills, deaths = rec.deaths, hours = rec.hours, firstSeen = rec.firstSeen, lastSeen = rec.lastSeen or rec.firstSeen,
            goals = ok and goals or nil, track = ok and track or nil })
    end
    local gs = KnoxGoals.state()
    if gs then KnoxLinkJson.write("app_goals.json", gs) end
    KnoxLinkJson.write("app_recap.json", { started = session.started, updated = getTimestamp(), day = day, players = recap })
    if data and data.cast then pcall(writeCast, day) end
end

-- OnTick may not fire on every server build, so the game clock drives the status files too
local statusLogged = false
local function statusTick()
    local now = getTimestampMs()
    if now < nextStatus then return end
    nextStatus = now + STATUS_MS
    local ok, err = pcall(writeStatus)
    if not statusLogged then
        statusLogged = true
        print(ok and "[KnoxLink] status files written" or ("[KnoxLink] status files failed: " .. tostring(err)))
    end
end
Events.EveryOneMinute.Add(statusTick)

local ticks = 0
Events.OnTick.Add(function()
    local now = getTimestampMs()
    statusTick()
    if not data or not data.open then return end
    ticks = ticks + 1
    if countdownEnd and now >= countdownEnd then
        countdownEnd = nil
        data.open = false
        data.begun = true
        print("[KnoxLink] lobby closed, game on")
        sendServerCommand(MODULE, "started", { host = data.host or "" })
        return
    end
    if ticks % 120 == 0 then
        -- the app said how many players to expect: go as soon as everyone is in
        if (data.expected or 0) > 0 and #onlineNames() >= data.expected then startCountdown() end
        broadcast()
    end
end)
