-- Knox Link: shared helpers and the automatic mod manager.
KnoxLink = KnoxLink or {}
KnoxLink.MOD_ID = "KnoxLink"
KnoxLink.SERVER_PREFIX = "KnoxLink"

local DIR = "KnoxLink/"
local PINNED = { "ZombieBuddy", "zdk", "ZModUnbork" }

function KnoxLink.log(s)
    print("[KnoxLink] " .. tostring(s))
end

-- File helpers (Zomboid/Lua/KnoxLink/...)

function KnoxLink.readLines(name)
    local lines = {}
    local reader = getFileReader(DIR .. name, false)
    if not reader then return nil end
    while true do
        local line = reader:readLine()
        if not line then break end
        table.insert(lines, line)
    end
    reader:close()
    return lines
end

function KnoxLink.writeLines(name, lines)
    local writer = getFileWriter(DIR .. name, true, false)
    if not writer then return end
    for _, line in ipairs(lines) do
        writer:write(line .. "\r\n")
    end
    writer:close()
end

local function trim(s)
    return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function cleanId(s)
    return (trim(s):gsub("^\\+", ""))
end

local function javaList(arr)
    local t = {}
    if arr then
        for i = 0, arr:size() - 1 do
            local v = cleanId(arr:get(i))
            if v ~= "" then table.insert(t, v) end
        end
    end
    return t
end

local function split(s, sep)
    local out, start = {}, 1
    while true do
        local i = string.find(s, sep, start, true)
        if not i then
            table.insert(out, string.sub(s, start))
            return out
        end
        table.insert(out, string.sub(s, start, i - 1))
        start = i + #sep
    end
end
KnoxLink.split = split

-- Scan every installed mod (Workshop + Zomboid/mods)

function KnoxLink.scanMods()
    local mods, order = {}, {}
    for _, directory in ipairs(getModDirectoryTable()) do
        local ok, info = pcall(getModInfo, directory)
        if ok and info and info:getId() and not mods[info:getId()] then
            local id = info:getId()
            local wid = info:getWorkshopID()
            wid = wid and trim(wid) or ""
            if wid == "" or wid == "0" or not isValidSteamID(wid) then wid = nil end
            local m = {
                id = id,
                info = info,
                name = info:getName() or id,
                workshopID = wid,
                dir = directory,
                require = javaList(info:getRequire()),
                incompatible = javaList(info:getIncompatible()),
                loadAfter = javaList(info:getLoadAfter()),
                loadBefore = javaList(info:getLoadBefore()),
                versionOk = info:isAvailableSelf(),
            }
            mods[id] = m
            table.insert(order, id)
        end
    end
    return mods, order
end

local function versionText(info)
    local vmin, vmax = info:getVersionMin(), info:getVersionMax()
    local range = ""
    if vmin and vmax then
        range = tostring(vmin) .. " to " .. tostring(vmax)
    elseif vmin then
        range = tostring(vmin) .. " or newer"
    elseif vmax then
        range = tostring(vmax) .. " or older"
    end
    return "made for game version " .. range .. ", you have " .. tostring(getCore():getGameVersion())
end

-- Returns { [id] = reason } for mods that cannot load.
function KnoxLink.findBroken(mods)
    local broken = {}
    for id, m in pairs(mods) do
        if not m.versionOk then broken[id] = versionText(m.info) end
    end
    local changed = true
    while changed do
        changed = false
        for id, m in pairs(mods) do
            if not broken[id] then
                for _, req in ipairs(m.require) do
                    if not mods[req] then
                        broken[id] = "needs \"" .. req .. "\", which isn't installed"
                    elseif broken[req] then
                        broken[id] = "needs \"" .. mods[req].name .. "\", which is broken"
                    end
                    if broken[id] then changed = true break end
                end
            end
        end
    end
    return broken
end

-- Dependency-aware load order. Keeps the current order wherever the rules allow.
function KnoxLink.sortMods(ids, mods)
    local index, active = {}, {}
    for i, id in ipairs(ids) do index[id] = i; active[id] = true end
    for p, id in ipairs(PINNED) do
        if active[id] then index[id] = -100 + p end
    end
    local before = {} -- before[a][b] = true means a must load before b
    local indeg = {}
    for _, id in ipairs(ids) do before[id] = {}; indeg[id] = 0 end
    local pinned = {}
    for _, id in ipairs(PINNED) do pinned[id] = true end
    local function edge(a, b)
        if a == b or not active[a] or not active[b] or before[a][b] then return end
        if pinned[b] and not pinned[a] then return end -- frameworks like ZombieBuddy always load first
        before[a][b] = true
        indeg[b] = indeg[b] + 1
    end
    for _, id in ipairs(ids) do
        local m = mods[id]
        if m then
            for _, r in ipairs(m.require) do edge(r, id) end
            for _, a in ipairs(m.loadAfter) do edge(a, id) end
            for _, b in ipairs(m.loadBefore) do edge(id, b) end
        end
    end
    local result, done = {}, {}
    while #result < #ids do
        local pick = nil
        for _, id in ipairs(ids) do
            if not done[id] and indeg[id] == 0 and (pick == nil or index[id] < index[pick]) then pick = id end
        end
        if not pick then
            -- dependency loop: fall back to the current order for whatever is left
            for _, id in ipairs(ids) do
                if not done[id] and (pick == nil or index[id] < index[pick]) then pick = id end
            end
        end
        done[pick] = true
        table.insert(result, pick)
        for b, _ in pairs(before[pick]) do indeg[b] = indeg[b] - 1 end
    end
    return result
end

-- Workshop availability (removed / hidden items), cached so it also works offline

KnoxLink.workshopStatus = KnoxLink.workshopStatus or {} -- [workshopID] = "ok" | "removed"

function KnoxLink.loadWorkshopStatus()
    local lines = KnoxLink.readLines("workshop_status.txt") or {}
    for _, line in ipairs(lines) do
        local p = split(line, "|")
        if p[1] and p[3] then KnoxLink.workshopStatus[p[1]] = p[3] end
    end
end

function KnoxLink.saveWorkshopStatus()
    local lines = {}
    for wid, st in pairs(KnoxLink.workshopStatus) do table.insert(lines, wid .. "|" .. st) end
    KnoxLink.writeLines("workshop_status.txt", lines)
end

function KnoxLink.isRemoved(wid)
    return wid ~= nil and KnoxLink.workshopStatus ~= nil and KnoxLink.workshopStatus[wid] == "removed"
end

function KnoxLink.checkWorkshop(mods, callback)
    if not getSteamModeActive() then if callback then callback(false) end return end
    local ids, seen = ArrayList.new(), {}
    for _, m in pairs(mods) do
        if m.workshopID and not seen[m.workshopID] then
            seen[m.workshopID] = true
            ids:add(m.workshopID)
        end
    end
    if ids:isEmpty() then if callback then callback(true) end return end
    -- the game calls back with (arg, status, info); the arg is required.
    -- The answer can arrive after a Lua reload wiped this copy of KnoxLink,
    -- so a stale callback just does nothing (the new copy asks again).
    local me = KnoxLink
    querySteamWorkshopItemDetails(ids, function(_, status, info)
        if KnoxLink == nil or KnoxLink ~= me or type(KnoxLink.workshopStatus) ~= "table" then return end
        if status ~= "Completed" then
            KnoxLink.log("Workshop check failed (offline?), using last known status")
            if callback then callback(false) end
            return
        end
        local found = {}
        for i = 0, info:size() - 1 do
            local d = info:get(i)
            local wid = d:getIDString()
            found[wid] = true
            local title = d:getTitle()
            if d:getTimeCreated() == 0 or title == nil or title == "" then
                KnoxLink.workshopStatus[wid] = "removed"
            else
                KnoxLink.workshopStatus[wid] = "ok"
            end
        end
        for wid, _ in pairs(seen) do
            if not found[wid] then KnoxLink.workshopStatus[wid] = "removed" end
        end
        KnoxLink.saveWorkshopStatus()
        if callback then callback(true) end
    end, {})
end

-- The automatic mod manager. Runs on every trip to the main menu:
--  * mods it has never seen are turned on (unless broken)
--  * broken mods are turned off
--  * mods you turned off yourself in the Mods menu stay off
--  * load order is fixed from each mod's require / loadModAfter / loadModBefore
-- Returns true when the active mod list changed (the game must reload Lua).

function KnoxLink.syncMods()
    local mods, scanOrder = KnoxLink.scanMods()
    local broken = KnoxLink.findBroken(mods)
    KnoxLink.mods, KnoxLink.broken = mods, broken

    local knownLines = KnoxLink.readLines("known_mods.txt")
    local known = {}
    for _, id in ipairs(knownLines or {}) do known[trim(id)] = true end

    local activeMods = ActiveMods.getById("default")
    local current = {}
    local list = activeMods:getMods()
    for i = 0, list:size() - 1 do table.insert(current, list:get(i)) end

    local report = {}
    local want, wantSet = {}, {}
    local function add(id)
        if not wantSet[id] then wantSet[id] = true; table.insert(want, id) end
    end

    for _, id in ipairs(current) do
        if mods[id] then
            if broken[id] then
                table.insert(report, "off|" .. id .. "|" .. mods[id].name .. "|" .. broken[id])
                wantSet[id] = "broken"
            else
                add(id)
            end
        end
        -- ids of mods that are no longer installed simply drop out
    end
    for _, id in ipairs(scanOrder) do
        if not known[id] and not wantSet[id] then
            if broken[id] then
                table.insert(report, "skip|" .. id .. "|" .. mods[id].name .. "|" .. broken[id])
            else
                add(id)
                if id ~= KnoxLink.MOD_ID then
                    table.insert(report, "on|" .. id .. "|" .. mods[id].name .. "|new mod")
                end
            end
        end
    end
    add(KnoxLink.MOD_ID)

    -- required mods of anything active must be active too
    local i = 1
    while i <= #want do
        local m = mods[want[i]]
        if m then
            for _, r in ipairs(m.require) do
                if mods[r] and not broken[r] and not wantSet[r] then
                    add(r)
                    table.insert(report, "on|" .. r .. "|" .. mods[r].name .. "|needed by " .. m.name)
                end
            end
        end
        i = i + 1
    end

    -- incompatible pairs: the mod that was on first wins
    local removed = {}
    for _, id in ipairs(want) do
        if not removed[id] and mods[id] then
            for _, other in ipairs(mods[id].incompatible) do
                if wantSet[other] and not removed[other] then
                    removed[other] = true
                    table.insert(report, "off|" .. other .. "|" .. (mods[other] and mods[other].name or other) .. "|conflicts with " .. mods[id].name)
                end
            end
        end
    end
    -- anything that needs a removed mod goes too
    local changed = true
    while changed do
        changed = false
        for _, id in ipairs(want) do
            if not removed[id] and mods[id] then
                for _, r in ipairs(mods[id].require) do
                    if removed[r] then
                        removed[id] = true
                        changed = true
                        table.insert(report, "off|" .. id .. "|" .. mods[id].name .. "|needs " .. (mods[r] and mods[r].name or r) .. ", which was turned off")
                        break
                    end
                end
            end
        end
    end
    local filtered = {}
    for _, entry in ipairs(report) do
        local p = split(entry, "|")
        if not (p[1] == "on" and removed[p[2]]) then table.insert(filtered, entry) end
    end
    report = filtered
    local final = {}
    for _, id in ipairs(want) do
        if not removed[id] and wantSet[id] == true then table.insert(final, id) end
    end
    final = KnoxLink.sortMods(final, mods)

    -- remember every installed mod so a mod you switch off stays off
    local allIds = {}
    for _, id in ipairs(scanOrder) do table.insert(allIds, id) end
    KnoxLink.writeLines("known_mods.txt", allIds)

    local differs = #final ~= #current
    if not differs then
        for k = 1, #final do
            if final[k] ~= current[k] then differs = true break end
        end
    end
    if differs then
        list:clear()
        for _, id in ipairs(final) do list:add(id) end
        saveModsFile()
        local copy = ActiveMods.getById("currentGame")
        if copy then copy:copyFrom(activeMods) end
    end
    if #report > 0 then
        KnoxLink.writeLines("report.txt", report)
    end
    return differs, report, table.concat(final, ";")
end

-- What the hosted server should load, based on the mods that are on right now.
function KnoxLink.serverModPlan(useLocalFiles)
    local mods = KnoxLink.mods or KnoxLink.scanMods()
    local active = ActiveMods.getById("default"):getMods()
    local plan = { mods = {}, workshop = {}, removed = {}, localOnly = {}, dropped = {} }
    local seenWid, droppedSet = {}, {}
    for i = 0, active:size() - 1 do
        local id = active:get(i)
        local m = mods[id]
        if m then
            if m.workshopID and KnoxLink.isRemoved(m.workshopID) then
                if useLocalFiles then
                    table.insert(plan.removed, m)
                else
                    droppedSet[id] = true
                    table.insert(plan.dropped, m)
                end
            elseif not m.workshopID then
                table.insert(plan.localOnly, m)
            end
            if not droppedSet[id] then
                -- without the Java helper, a mod that needs a dropped mod can't load either
                for _, r in ipairs(m.require) do
                    if droppedSet[r] then droppedSet[id] = true; table.insert(plan.dropped, m) break end
                end
            end
            if not droppedSet[id] then
                table.insert(plan.mods, id)
                if m.workshopID and not KnoxLink.isRemoved(m.workshopID) and not seenWid[m.workshopID] then
                    seenWid[m.workshopID] = true
                    table.insert(plan.workshop, m.workshopID)
                end
            end
        end
    end
    return plan
end

-- Remembered host screen choices (Zomboid/Lua/KnoxLink/prefs.txt, key=value lines)

function KnoxLink.loadPrefs()
    local prefs = {}
    for _, line in ipairs(KnoxLink.readLines("prefs.txt") or {}) do
        local eq = string.find(line, "=", 1, true)
        if eq then prefs[string.sub(line, 1, eq - 1)] = string.sub(line, eq + 1) end
    end
    return prefs
end

function KnoxLink.savePrefs(prefs)
    local lines = {}
    for k, v in pairs(prefs) do table.insert(lines, k .. "=" .. tostring(v)) end
    table.sort(lines)
    KnoxLink.writeLines("prefs.txt", lines)
end
