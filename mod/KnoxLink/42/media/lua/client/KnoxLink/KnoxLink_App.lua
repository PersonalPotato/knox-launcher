-- Bridge to the Knox Launcher app.
--  * exports what the app can't read on its own (traits, professions, looks, settings, mods)
--  * hosts or joins when the app asks, and fills in the character made in the app
-- All files live in Zomboid/Lua/KnoxLink/ (app_*.txt / app_*.json).
require "KnoxLink/KnoxLink_Core"
require "KnoxLink/KnoxLink_Json"
require "OptionScreens/MainScreen"
require "OptionScreens/CharacterCreationProfession"
require "OptionScreens/CharacterCreationMain"
require "OptionScreens/MapSpawnSelect"
require "OptionScreens/ConnectToServer"
require "OptionScreens/ServerSettingsScreen"

KnoxLink.App = KnoxLink.App or {}
local A = KnoxLink.App
local PRESET = "Knox Launcher"

local function arr() return { __array = true } end
A.toJson = KnoxLinkJson.encode


local function writeJson(name, data)
    local writer = getFileWriter("KnoxLink/" .. name, true, false)
    if not writer then return end
    writer:write(A.toJson(data))
    writer:close()
end
A.writeJson = writeJson

-- key=value files written by the app
function A.readKV(name)
    local lines = KnoxLink.readLines(name)
    if not lines or #lines == 0 then return nil end
    local t = {}
    for _, line in ipairs(lines) do
        local eq = string.find(line, "=", 1, true)
        if eq then t[string.sub(line, 1, eq - 1)] = string.sub(line, eq + 1) end
    end
    return t
end

local function safe(f, ...)
    local ok, r = pcall(f, ...)
    if ok then return r end
    return nil
end

local function str(v)
    if v == nil then return "" end
    return tostring(v)
end

-- Export ----------------------------------------------------------------------

local function xpTable(map)
    local out = arr()
    if not map then return out end
    local t = safe(transformIntoKahluaTable, map) or {}
    for perk, level in pairs(t) do
        local lv = type(level) == "number" and level or safe(function() return level:intValue() end) or 0
        table.insert(out, { perk = str(safe(PerkFactory.getPerkName, perk) or perk), level = lv })
    end
    return out
end

local function traitName(t)
    if t == nil then return "" end
    return str(safe(function() return t:getName() end) or t)
end

local function exportProfessions()
    local out = arr()
    local list = CharacterProfessionDefinition.getProfessions()
    for i = 0, list:size() - 1 do
        local p = list:get(i)
        local granted = arr()
        local g = p:getGrantedTraits()
        for j = 0, g:size() - 1 do
            local def = CharacterTraitDefinition.getCharacterTraitDefinition(g:get(j))
            if def then table.insert(granted, def:getType():getName()) end
        end
        table.insert(out, {
            id = p:getType():getName(),
            name = str(p:getUIName()),
            desc = str(p:getDescription()),
            cost = p:getCost(),
            granted = granted,
            xp = xpTable(p:getXpBoosts()),
        })
    end
    return out
end


local function exportTraits()
    local out = arr()
    local list = CharacterTraitDefinition.getTraits()
    for i = 0, list:size() - 1 do
        local t = list:get(i)
        local ex = arr()
        local m = t:getMutuallyExclusiveTraits()
        if m then
            for j = 0, m:size() - 1 do table.insert(ex, traitName(m:get(j))) end
        end
        local typ = t:getType()
        local granted = arr()
        local g = safe(function() return t:getGrantedTraits() end)
        if g then
            for j = 0, g:size() - 1 do
                local def = CharacterTraitDefinition.getCharacterTraitDefinition(g:get(j))
                if def then table.insert(granted, def:getType():getName()) end
            end
        end
        table.insert(out, {
            granted = granted,
            id = typ:getName(),
            name = str(t:getLabel()),
            desc = str(t:getDescription()),
            cost = t:getCost(),
            free = t:isFree() and true or false,
            exclusive = ex,
            xp = xpTable(t:getXpBoosts()),
            sleep = (typ == CharacterTrait.INSOMNIAC or typ == CharacterTrait.NEEDS_LESS_SLEEP or typ == CharacterTrait.NEEDS_MORE_SLEEP) and true or false,
        })
    end
    return out
end

local function hairList(female)
    local out = arr()
    local styles = getAllHairStyles(female)
    for i = 0, styles:size() - 1 do
        local id = styles:get(i)
        local style = female and getHairStylesInstance():FindFemaleStyle(id) or getHairStylesInstance():FindMaleStyle(id)
        if style and not style:isNoChoose() then
            table.insert(out, { id = id, name = id == "" and getText("IGUI_Hair_Bald") or getText("IGUI_Hair_" .. id) })
        end
    end
    return out
end

local function clothingDefs(def)
    local out = {}
    if not def then return out end
    for location, entry in pairs(def) do
        local items = arr()
        for _, full in ipairs(entry.items or {}) do
            local item = ScriptManager.instance:FindItem(full)
            if item then table.insert(items, { id = full, name = str(item:getDisplayName()) }) end
        end
        out[location] = { name = getText("UI_ClothingType_" .. location), items = items }
    end
    return out
end

local function exportLooks()
    local looks = {}
    looks.hairMale = hairList(false)
    looks.hairFemale = hairList(true)
    looks.beards = arr()
    local beards = getAllBeardStyles()
    for i = 0, beards:size() - 1 do
        local id = beards:get(i)
        table.insert(looks.beards, { id = id, name = id == "" and getText("IGUI_Beard_None") or getText("IGUI_Beard_" .. id) })
    end
    looks.hairColors = arr()
    local desc = MainScreen.instance and MainScreen.instance.desc
    if desc then
        local colors = desc:getCommonHairColor()
        local info = ColorInfo.new()
        for i = 0, colors:size() - 1 do
            local c = colors:get(i)
            info:set(c:getRedFloat(), c:getGreenFloat(), c:getBlueFloat(), 1)
            table.insert(looks.hairColors, { r = info:getR(), g = info:getG(), b = info:getB() })
        end
    end
    looks.skinColors = {
        { r = 1, g = 0.91, b = 0.72 }, { r = 0.98, g = 0.79, b = 0.49 }, { r = 0.8, g = 0.65, b = 0.45 },
        { r = 0.54, g = 0.38, b = 0.25 }, { r = 0.36, g = 0.25, b = 0.14 },
    }
    looks.voices = arr()
    local voices = getAllVoiceStyles()
    for i = 0, voices:size() - 1 do
        local v = voices:get(i)
        if v then
            table.insert(looks.voices, { id = v:getName(), name = getText("IGUI_Voice_" .. v:getName()), body = str(safe(function() return v:getBodyTypeDefault() end)) })
        end
    end
    looks.clothing = {}
    if ClothingSelectionDefinitions then
        for key, def in pairs(ClothingSelectionDefinitions) do
            if type(def) == "table" then
                looks.clothing[key] = { Male = clothingDefs(def.Male), Female = clothingDefs(def.Female) }
            end
        end
    end
    return looks
end

local function exportSandbox()
    local pages = arr()
    local ok, tbl = pcall(ServerSettingsScreen.getSandboxSettingsTable)
    if not ok or not tbl then return pages end
    local so = getSandboxOptions()
    for _, page in ipairs(tbl) do
        local settings = arr()
        for _, s in ipairs(page.settings or {}) do
            local opt = so:getOptionByName(s.name)
            local values = arr()
            for _, v in ipairs(s.values or {}) do table.insert(values, str(v)) end
            local entry = {
                name = s.name,
                label = str(s.translatedName or s.name),
                tooltip = str(s.tooltip),
                type = str(s.type),
                values = values,
                title = s.title and getText("Sandbox_Title_" .. s.title) or nil,
            }
            if opt then
                entry.optType = str(opt:getType())
                entry.default = str(safe(function() return opt:asConfigOption():getDefaultValue() end) or safe(function() return opt:getDefaultValue() end))
                -- only number options have a range (asking others logs an error even inside pcall)
                if entry.optType == "integer" or entry.optType == "double" then
                    entry.min = safe(function() return opt:getMin() end)
                    entry.max = safe(function() return opt:getMax() end)
                end
            end
            table.insert(settings, entry)
        end
        table.insert(pages, { name = str(page.name), settings = settings })
    end
    return pages
end

local function exportServerOptions()
    local out = arr()
    local so = getServerOptions()
    if not so then return out end
    for i = 0, so:getNumOptions() - 1 do
        local raw = so:getOptionByIndex(i)
        local o = safe(function() return raw:asConfigOption() end) or raw
        local name = o:getName()
        local label = getText("UI_ServerOption_" .. name)
        if label == "UI_ServerOption_" .. name then label = name end
        local values = arr()
        local numeric = o:getType() == "integer" or o:getType() == "double"
        if o:getType() == "enum" then
            for k = 1, o:getNumValues() do table.insert(values, str(o:getValueTranslationByIndex(k))) end
        end
        table.insert(out, {
            name = name,
            label = label,
            type = str(o:getType()),
            default = str(safe(function() return o:getDefaultValue() end)),
            tooltip = str(safe(function() return o:getTooltip() end)),
            values = values,
            min = numeric and safe(function() return o:getMin() end) or nil,
            max = numeric and safe(function() return o:getMax() end) or nil,
        })
    end
    return out
end

local function exportSpawns()
    local out = arr()
    local settings = ServerSettings.new("EZHostProbe")
    if not settings or not settings:isValid() then return out end
    settings:resetToDefault()
    for i = 1, settings:getNumSpawnRegions() do
        local name, file = settings:getSpawnRegionName(i - 1), settings:getSpawnRegionFile(i - 1)
        local pts = arr()
        local ok, profs = pcall(function() return settings:loadSpawnPointsFile(file) end)
        if ok and profs then
            local list = profs.unemployed
            if not list then for _, v in pairs(profs) do list = v break end end
            for _, p in ipairs(list or {}) do
                local x, y = p.posX, p.posY
                if p.worldX then x = p.worldX * 300 + p.posX; y = p.worldY * 300 + p.posY end
                if x and y then table.insert(pts, { x = math.floor(x), y = math.floor(y), z = p.posZ or 0 }) end
            end
        end
        table.insert(out, { name = str(name), file = str(file), points = pts })
    end
    return out
end

local function exportMods()
    local out = arr()
    local mods = KnoxLink.mods or KnoxLink.scanMods()
    local broken = KnoxLink.broken or KnoxLink.findBroken(mods)
    local active, activeIndex = ActiveMods.getById("default"):getMods(), {}
    for i = 0, active:size() - 1 do activeIndex[active:get(i)] = i + 1 end
    for id, m in pairs(mods) do
        local posters = arr()
        local n = safe(function() return m.info:getPosterCount() end) or 0
        for i = 0, n - 1 do table.insert(posters, str(safe(function() return m.info:getPoster(i) end))) end
        local req = arr()
        for _, r in ipairs(m.require) do table.insert(req, r) end
        local inc = arr()
        for _, r in ipairs(m.incompatible) do table.insert(inc, r) end
        table.insert(out, {
            id = id,
            name = str(m.name),
            workshopID = m.workshopID or "",
            dir = str(m.dir),
            description = str(safe(function() return m.info:getDescription() end)),
            posters = posters,
            require = req,
            incompatible = inc,
            broken = broken[id] or "",
            removed = (m.workshopID and KnoxLink.isRemoved(m.workshopID)) and true or false,
            on = activeIndex[id] ~= nil,
            order = activeIndex[id] or 0,
        })
    end
    return out
end

local function exportWorlds()
    local out = arr()
    getServerSettingsManager():readAllSettings()
    for i = 1, getServerSettingsManager():getSettingsCount() do
        local name = getServerSettingsManager():getSettingsByIndex(i - 1):getName()
        if name ~= "EZHostProbe" then
            table.insert(out, {
                name = name,
                exists = checkSaveFolderExists("Multiplayer/" .. CoopServer:getServerSaveFolder(name)) and true or false,
            })
        end
    end
    return out
end

local function exportItems()
    local out = arr()
    local all = getScriptManager():getAllItems()
    for i = 0, all:size() - 1 do
        local it = all:get(i)
        local hidden = safe(function() return it:getObsolete() end) or safe(function() return it:isHidden() end)
        if not hidden then
            table.insert(out, {
                id = str(it:getFullName()),
                name = str(safe(function() return it:getDisplayName() end)),
                cat = str(safe(function() return it:getDisplayCategory() end)),
            })
        end
    end
    return out
end

function A.exportGameData()
    local data = {
        version = 1,
        time = getTimestamp(),
        game = str(getCore():getGameVersion()),
        steamID = str(safe(getCurrentUserSteamID)),
        userName = str(safe(getCurrentUserProfileName)),
        javaHelper = (KnoxLink_javaHelperReady ~= nil and KnoxLink_javaHelperReady() == true),
    }
    local parts = {
        professions = exportProfessions, traits = exportTraits, looks = exportLooks,
        sandbox = exportSandbox, server = exportServerOptions, spawns = exportSpawns,
        mods = exportMods, worlds = exportWorlds, items = exportItems,
    }
    for key, f in pairs(parts) do
        local ok, r = pcall(f)
        if ok then data[key] = r else
            KnoxLink.log("app export: " .. key .. " failed: " .. tostring(r))
            data[key] = arr()
        end
    end
    writeJson("app_gamedata.json", data)
    KnoxLink.log("app export written")
end

-- Launch requests from the app ------------------------------------------------

local function setServerOption(settings, name, value)
    local opt = settings:getServerOptions():getOptionByName(name)
    if opt then pcall(function() opt:parse(tostring(value)) end) end
end

local function setOption(options, name, value)
    local opt = options:getOptionByName(name)
    if not opt then return end
    local ok = pcall(function() opt:asConfigOption():parse(tostring(value)) end)
    if not ok then pcall(function() opt:parse(tostring(value)) end) end
end

local function setSandboxOption(settings, name, value)
    setOption(settings:getSandboxOptions(), name, value)
end

function A.pendingLaunch()
    local L = A.readKV("app_launch.txt")
    if not L or not L.mode then return nil end
    if (tonumber(L.time) or 0) < getTimestamp() - 900 then return nil end -- older than 15 minutes
    return L
end

function A.host(L)
    local name = L.world or "KnoxLink"
    local settings = ServerSettings.new(name)
    if not settings:isValid() then
        KnoxLink.showMessage("<SIZE:medium> Can't create the server settings <LINE> <SIZE:small> " .. tostring(settings:getErrorMsg()))
        return
    end
    local isNew = getServerSettingsManager():isValidNewName(name)
    if isNew then
        settings:resetToDefault()
        setServerOption(settings, "PublicName", "Knox Launcher")
        setServerOption(settings, "Public", "false")
    else
        settings:loadFiles()
    end
    KnoxLink.applyModsToSettings(settings, isNew)

    local H = KnoxLink.readLines("app_host.txt") or {}
    for _, line in ipairs(H) do
        local eq = string.find(line, "=", 1, true)
        if eq then
            local key, value = string.sub(line, 1, eq - 1), string.sub(line, eq + 1)
            if luautils.stringStarts(key, "ini.") then
                setServerOption(settings, string.sub(key, 5), value)
            elseif luautils.stringStarts(key, "sandbox.") then
                setSandboxOption(settings, string.sub(key, 9), value)
            end
        end
    end
    if L.spawn and L.spawn ~= "" then setServerOption(settings, "SpawnPoint", L.spawn) end
    settings:saveFiles()

    -- for the server: lobby flag, town, how many players to wait for, starting kit, respawn rule,
    -- the start building and the houses a "random" respawn picks from
    KnoxLink.writeLines("lobby.txt", { L.lobby or "1", L.town or "", L.expected or "0", L.loadout or "",
        L.respawn or "base", L.spawn or "", L.respawnSpots or "" })

    A.wantCharacter(L.character)
    KnoxLink.launch(name, tonumber(L.memory) or 4096)
end

-- Solo: a normal single-player Sandbox game, no server. app_solo.txt has the world settings
-- (sandbox.Name=value), the start building and the loadout, which go on once the character exists.
local function readSolo()
    local S = { sandbox = {} }
    for _, line in ipairs(KnoxLink.readLines("app_solo.txt") or {}) do
        local eq = string.find(line, "=", 1, true)
        if eq then
            local key, value = string.sub(line, 1, eq - 1), string.sub(line, eq + 1)
            if luautils.stringStarts(key, "sandbox.") then S.sandbox[string.sub(key, 9)] = value
            else S[key] = value end
        end
    end
    return S
end

function A.solo(L)
    local ms = MainScreen.instance
    if not ms then return end
    local S = readSolo()
    ActiveMods.getById("currentGame"):copyFrom(ActiveMods.getById("default"))
    getWorld():setGameMode("Sandbox")
    ms:setDefaultSandboxVars()
    local options = getSandboxOptions()
    for name, value in pairs(S.sandbox) do setOption(options, name, value) end
    pcall(function()
        options:set("WaterShutModifier", options:randomWaterShut(options:getOptionByName("WaterShut"):getValue()))
        options:set("ElecShutModifier", options:randomElectricityShut(options:getOptionByName("ElecShut"):getValue()))
    end)
    options:toLua()
    getWorld():setMap("DEFAULT")
    ms.createWorld = true
    local spawn = MapSpawnSelect.instance
    if spawn then
        pcall(function() spawn:saveGenParams() end)
        local region = spawn:useDefaultSpawnRegion()
        if region then
            setSpawnRegion(region.name)
            getCore():setSelectedMap(tostring(region.name))
        end
    end
    getWorld():setWorld(sanitizeWorldName(L.world or "Solo"))
    ms.bottomPanel:setVisible(false)
    A.wantCharacter(L.character)
    ms.charCreationProfession.previousScreen = "NewGameScreen"
    ms.charCreationProfession:setVisible(true)
end

function A.load(L)
    if not MainScreen.instance or not L.gameMode or not L.save then return end
    MainScreen.instance.bottomPanel:setVisible(false)
    MainScreen.continueLatestSave(L.gameMode, L.save)
end

-- New solo character: move to the start building and hand out the loadout, once.
Events.OnNewGame.Add(function(player)
    if isClient() or isServer() then return end
    local S = readSolo()
    if not S.world or S.world == "" or sanitizeWorldName(S.world) ~= getWorld():getWorld() then return end
    KnoxLink.writeLines("app_solo.txt", {})
    player = player or getPlayer()
    if not player then return end
    local spot = KnoxLink.split(S.spawn or "", ",")
    local x, y, z = tonumber(spot[1]), tonumber(spot[2]), tonumber(spot[3]) or 0
    if x and y and (x ~= 0 or y ~= 0) then
        x, y = x + 0.5, y + 0.5
        if not pcall(function() player:teleportTo(x, y, z) end) then
            player:setX(x) player:setY(y) player:setZ(z)
            player:setLastX(x) player:setLastY(y) player:setLastZ(z)
        end
    end
    local inv = player:getInventory()
    for _, entry in ipairs(KnoxLink.split(S.kit or "", ";")) do
        local colon = string.find(entry, ":", 1, true)
        local itemType, count = entry, 1
        if colon then
            itemType = string.sub(entry, 1, colon - 1)
            count = tonumber(string.sub(entry, colon + 1)) or 1
        end
        if itemType ~= "" then
            for _ = 1, math.min(count, 50) do pcall(function() inv:AddItem(itemType) end) end
        end
    end
    KnoxLink.log("solo start: " .. tostring(S.spawn) .. " kit " .. tostring(S.kit))
end)

-- Joining: keep trying until the host's server is up.
A.join = nil

local function status(text)
    local ms = MainScreen.instance
    if not ms then return end
    if not ms.ezStatus then
        local w = 520
        ms.ezStatus = ISLabel:new((ms.width - w) / 2, ms.height - 120, 30, "", 1, 0.82, 0.55, 1, UIFont.Medium, true)
        ms.ezStatus:initialise()
        ms:addChild(ms.ezStatus)
    end
    ms.ezStatus:setName(text or "")
    ms.ezStatus:setX((ms.width - ms.ezStatus:getWidth()) / 2)
    ms.ezStatus:setVisible(text ~= nil)
    if text then ms.ezStatus:bringToTop() end
end
A.status = status

function A.tryJoin()
    local J = A.join
    if not J then return end
    J.tries = J.tries + 1
    J.waiting = nil
    status("Joining " .. J.hostName .. " (try " .. J.tries .. ")")
    ConnectToServer.instance:connectCoop(MainScreen.instance.bottomPanel, J.host)
end

function A.joinGame(L)
    A.join = { host = L.host, hostName = L.hostName or "host", tries = 0, max = tonumber(L.tries) or 45 }
    A.wantCharacter(L.character)
    A.tryJoin()
end

local function onConnectFailed(message)
    local J = A.join
    if not J or J.waiting then return end
    if J.tries >= J.max then
        A.join = nil
        status(nil)
        A.clearCharacter()
        return
    end
    J.waiting = 8 * 30 -- ticks
    status("Waiting for " .. J.hostName .. "'s server to start... (" .. tostring(message or "not up yet") .. ")")
end
Events.OnConnectFailed.Add(onConnectFailed)
Events.OnConnected.Add(function()
    if A.join then A.join = nil; status(nil) end
end)

-- Character: the app saves it as the "Knox Launcher" preset in the game's own preset files
-- (saved_builds.txt = job + traits, saved_outfits.txt = looks). When the creator opens, Knox Link
-- loads that preset and clicks through it behind a cover, so the player never sees the screens.
-- Skills come with the job and traits, the same way the game's own creator gives them.
-- Joining or hosting reloads Lua with the server's mods, which wipes memory, so the request is
-- kept in app_character.txt (preset name + time) until the character exists.

local CHAR_FILE = "app_character.txt"
local CHAR_MAX_AGE = 1800 -- seconds; a join can sit on "waiting for the server" for a while

function A.wantCharacter(name)
    if not name or name == "" then return end
    KnoxLink.writeLines(CHAR_FILE, { name, tostring(getTimestamp()) })
end

local function wantedCharacter()
    local lines = KnoxLink.readLines(CHAR_FILE)
    if not lines or not lines[1] or lines[1] == "" then return nil end
    if (tonumber(lines[2]) or 0) < getTimestamp() - CHAR_MAX_AGE then return nil end
    return lines[1]
end

function A.clearCharacter()
    KnoxLink.writeLines(CHAR_FILE, {})
end
local clearCharacter = A.clearCharacter

local function hasPreset(file, name)
    local reader = getFileReader(file, false)
    if not reader then return false end
    local found = false
    while true do
        local line = reader:readLine()
        if not line then break end
        if luautils.stringStarts(line, name .. ":") then found = true end
    end
    reader:close()
    return found
end

-- Black screen with one line while the creator clicks itself through.
local function cover(show)
    if not show then
        if A.cover then
            A.cover:setVisible(false)
            A.cover:removeFromUIManager()
            A.cover = nil
        end
        return
    end
    if not A.cover then
        local core = getCore()
        local panel = ISPanel:new(0, 0, core:getScreenWidth(), core:getScreenHeight())
        panel.backgroundColor = { r = 0, g = 0, b = 0, a = 1 }
        panel.borderColor = { r = 0, g = 0, b = 0, a = 0 }
        panel:initialise()
        panel.render = function(self)
            self:drawTextCentre("Loading your survivor...", self.width / 2, self.height / 2, 0.82, 0.55, 1, 1, UIFont.Medium)
        end
        panel:addToUIManager()
        panel:setAlwaysOnTop(true)
        A.cover = panel
    end
    A.cover:setVisible(true)
    A.cover:bringToTop()
end

A.charStep = nil

local function stopCharacter(why)
    if A.charStep and why then KnoxLink.log("character: " .. why) end
    A.charStep = nil
    cover(false)
end

-- The creator screens that belong together: the main menu ones, or the in-game respawn ones.
local function creatorScreens(prof)
    local coop = CoopCharacterCreation and CoopCharacterCreation.instance
    if coop and coop.charCreationProfession == prof then return prof, coop.charCreationMain, true end
    local ms = MainScreen.instance
    return prof, ms and ms.charCreationMain, false
end

local function startCharacter(prof, name, respawn)
    local _, main, coop = creatorScreens(prof)
    if not main then return end
    if not hasPreset("saved_builds.txt", name) then
        KnoxLink.log("character: no \"" .. name .. "\" preset in saved_builds.txt, leaving the creator open")
        return
    end
    A.charStep = { stage = "build", wait = 2, ticks = 0, name = name, prof = prof, main = main, coop = coop, respawn = respawn }
    cover(true)
end

-- In multiplayer the server's spawn list can have more than one region. The launcher already
-- picked the start building (SpawnPoint), so take the default and go straight to the creator.
local oldSpawnVisible = MapSpawnSelect.setVisible
function MapSpawnSelect:setVisible(visible, joypadData)
    oldSpawnVisible(self, visible, joypadData)
    if visible and self == MapSpawnSelect.instance and not A.charStep and wantedCharacter()
        and MainScreen.instance and not MainScreen.instance.inGame then
        A.charStep = { stage = "spawn", wait = 2, ticks = 0, spawn = self }
        cover(true)
    end
end

local oldProfVisible = CharacterCreationProfession.setVisible
function CharacterCreationProfession:setVisible(visible, joypadData)
    oldProfVisible(self, visible, joypadData)
    if not visible then return end
    if A.charStep and A.charStep.stage ~= "spawn" then return end
    A.charStep = nil
    local name = wantedCharacter()
    if name then
        startCharacter(self, name, false)
    elseif isClient() and CoopCharacterCreation and CoopCharacterCreation.instance
        and CoopCharacterCreation.instance.charCreationProfession == self then
        -- respawning on a Knox server: the next life is the launcher character again
        startCharacter(self, PRESET, true)
    else
        cover(false)
    end
end

local function selectedTraits(prof)
    local have = {}
    for _, v in ipairs(prof.listboxTraitSelected and prof.listboxTraitSelected.items or {}) do
        pcall(function() have[v.item:getType():getName()] = true end)
    end
    return have
end

-- Log what didn't make it, so a bad preset shows up in console.txt.
local function checkBuild(S)
    local reader = getFileReader("saved_builds.txt", false)
    local build
    while reader do
        local line = reader:readLine()
        if not line then break end
        if luautils.stringStarts(line, S.name .. ":") then build = string.sub(line, #S.name + 2) end
    end
    if reader then reader:close() end
    local parts = luautils.split(build or "", ";")
    local job = "?"
    pcall(function() job = S.prof.profession:getType():getName() end)
    local have, missing = selectedTraits(S.prof), {}
    for i = 2, #parts do
        if parts[i] ~= "" and not have[parts[i]] then table.insert(missing, parts[i]) end
    end
    local text = "job " .. tostring(job) .. (parts[1] and job ~= parts[1] and (" (wanted " .. parts[1] .. ")") or "")
    if #missing > 0 then text = text .. ", traits not added: " .. table.concat(missing, ", ") end
    return text
end

local function stepCharacter()
    local S = A.charStep
    if not S then return end
    S.ticks = S.ticks + 1
    if S.ticks > 600 then return stopCharacter("gave up, creator left open") end
    if S.wait > 0 then S.wait = S.wait - 1 return end
    if S.stage == "spawn" then
        if not S.spawn:isVisible() then return stopCharacter("spawn screen closed") end
        -- clickNext opens the creator, whose setVisible starts the character steps
        S.spawn:clickNext()
        if A.charStep == S then stopCharacter("no creator after the spawn screen") end
    elseif S.stage == "build" then
        if not S.prof:isVisible() then return stopCharacter("creator closed") end
        CharacterCreationProfession.loadBuild(S.prof, { options = { S.name }, selected = 1 })
        S.prof:checkXPBoost()
        S.result = checkBuild(S)
        S.prof:onOptionMouseDown({ internal = "NEXT" })
        S.stage, S.wait = "outfit", 2
    elseif S.stage == "outfit" then
        -- the respawn creator opens CharacterCreationMain.instance, which may not be the one we expected
        local other = CharacterCreationMain.instance
        if not S.main:isVisible() and other and other:isVisible() then S.main = other end
        if not S.main:isVisible() then return stopCharacter("looks screen didn't open") end
        if hasPreset("saved_outfits.txt", S.name) then
            S.main:loadOutfit({ options = { S.name }, selected = 1 })
        end
        -- creator mods' own sliders (Size Matters height, body shape...), set from the launcher
        for field, value in pairs(A.readKV("app_extras.txt") or {}) do
            local control, n = S.main[field], tonumber(value)
            if control and n and control.setCurrentValue then pcall(control.setCurrentValue, control, n, true) end
        end
        S.stage, S.wait = "play", 2
    elseif S.stage == "play" then
        if not S.main:isVisible() then return stopCharacter("looks screen closed") end
        local who = tostring(S.main.forenameEntry:getText()) .. " " .. tostring(S.main.surnameEntry:getText())
        KnoxLink.log("character: " .. (S.respawn and "respawn as " or "loaded ") .. who .. ", " .. tostring(S.result))
        A.charStep = nil
        if not S.respawn then clearCharacter() end
        S.main:onOptionMouseDown({ internal = "NEXT" })
        cover(false)
    end
end

local function safeStep()
    if not A.charStep then return end
    local ok, err = pcall(stepCharacter)
    if not ok then stopCharacter("failed (" .. tostring(err) .. "), creator left open") end
end
A.stepCharacter = safeStep

-- A returning survivor skips the creator: the request is done once a game is running.
Events.OnGameStart.Add(function()
    if not A.charStep then clearCharacter() end
end)

-- Main menu tick: called by KnoxLink_MainMenu after the mod sync settled.
function A.onMenuReady()
    local L = A.pendingLaunch()
    pcall(A.exportGameData)
    if L then
        KnoxLink.writeLines("app_launch.txt", {})
        KnoxLink.log("app launch: " .. tostring(L.mode))
        if L.mode == "host" then A.host(L)
        elseif L.mode == "join" then A.joinGame(L)
        elseif L.mode == "solo" then A.solo(L)
        elseif L.mode == "load" then A.load(L) end
    end
end

Events.OnFETick.Add(function()
    local J = A.join
    if J and J.waiting then
        J.waiting = J.waiting - 1
        if J.waiting <= 0 then
            if ConnectToServer.instance:isVisible() then ConnectToServer.instance:onBackButton() end
            A.tryJoin()
        end
    end
    A.stepCharacter()
end)
-- respawning in game: the front-end tick doesn't run there
Events.OnRenderTick.Add(function() A.stepCharacter() end)

-- In game, the host runs commands the app's server panel asks for (kick, ban, message).
-- app_commands.txt lines: id|command|argument
local lastCommand = nil
local commandTicks = 0
Events.OnTick.Add(function()
    if not isClient() or not isCoopHost() then return end
    commandTicks = commandTicks + 1
    if commandTicks < 60 then return end
    commandTicks = 0
    local lines = KnoxLink.readLines("app_commands.txt")
    if not lines then lastCommand = lastCommand or 0 return end
    -- the first read only learns where the list is, so old commands never replay
    local first = lastCommand == nil
    for _, line in ipairs(lines) do
        local p = KnoxLink.split(line, "|")
        local id = tonumber(p[1])
        if id and (lastCommand == nil or id > lastCommand) then
            if not first then
                local cmd, arg = p[2], (p[3] or ""):gsub("\"", "'")
                if cmd == "kick" then SendCommandToServer("/kickuser \"" .. arg .. "\"")
                elseif cmd == "ban" then SendCommandToServer("/banuser \"" .. arg .. "\"")
                elseif cmd == "msg" then SendCommandToServer("/servermsg \"" .. arg .. "\"")
                elseif cmd == "save" then SendCommandToServer("/save") end
                KnoxLink.log("app command: " .. tostring(cmd) .. " " .. arg)
            end
            lastCommand = id
        end
    end
    if lastCommand == nil then lastCommand = 0 end
end)
