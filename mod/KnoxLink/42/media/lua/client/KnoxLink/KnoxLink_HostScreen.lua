-- Knox Link: the "HOST WITH MODS" screen and the launch flow.
require "ISUI/ISPanelJoypad"
require "ISUI/ISScrollingListBox"
require "OptionScreens/CoopOptionsScreen"
require "OptionScreens/ServerSettingsScreen"
require "KnoxLink/KnoxLink_Core"

KnoxLinkScreen = ISPanelJoypad:derive("KnoxLinkScreen")
local ModList = ISScrollingListBox:derive("KnoxLinkModList")

local FONT_HGT_SMALL = getTextManager():getFontHeight(UIFont.Small)
local FONT_HGT_MEDIUM = getTextManager():getFontHeight(UIFont.Medium)
local FONT_HGT_LARGE = getTextManager():getFontHeight(UIFont.Large)
local PAD = 20
local GAP = 10
local BTN_HGT = math.max(28, FONT_HGT_MEDIUM + 8)
local ROW_HGT = math.max(26, FONT_HGT_SMALL + 10)
local NEW_WORLD = "__new__"
local MEMORY_OPTIONS = { 3072, 4096, 6144, 8192 }
local PLAYER_OPTIONS = { 2, 4, 8, 16, 32 }

-- palette
local C = {
    accent = { r = 0.95, g = 0.62, b = 0.18 },
    ok = { r = 0.45, g = 0.85, b = 0.45 },
    bad = { r = 0.95, g = 0.40, b = 0.40 },
    warn = { r = 0.98, g = 0.80, b = 0.30 },
    dim = { r = 0.65, g = 0.65, b = 0.65 },
    card = { r = 0.09, g = 0.09, b = 0.10, a = 0.92 },
    line = { r = 1, g = 1, b = 1, a = 0.12 },
}
KnoxLink.COLORS = C

local function javaHelperReady()
    return KnoxLink_javaHelperReady ~= nil and KnoxLink_javaHelperReady() == true
end

local function worldExists(name)
    return checkSaveFolderExists("Multiplayer/" .. CoopServer:getServerSaveFolder(name))
end

local function listWorlds()
    local worlds = {}
    getServerSettingsManager():readAllSettings()
    for i = 1, getServerSettingsManager():getSettingsCount() do
        local name = getServerSettingsManager():getSettingsByIndex(i - 1):getName()
        if luautils.stringStarts(name, KnoxLink.SERVER_PREFIX) then table.insert(worlds, name) end
    end
    table.sort(worlds)
    return worlds
end

local function nextFreeName()
    local base = KnoxLink.SERVER_PREFIX
    if getServerSettingsManager():isValidNewName(base) then return base end
    local n = 2
    while not getServerSettingsManager():isValidNewName(base .. n) do n = n + 1 end
    return base .. n
end

local function drawCard(ui, x, y, w, h, title)
    ui:drawRect(x, y, w, h, C.card.a, C.card.r, C.card.g, C.card.b)
    ui:drawRectBorder(x, y, w, h, C.line.a, C.line.r, C.line.g, C.line.b)
    if title then
        ui:drawRect(x, y, 3, h, 0.9, C.accent.r, C.accent.g, C.accent.b)
        ui:drawText(title, x + 14, y + 8, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Small)
    end
end
KnoxLink.drawCard = drawCard

---------------------------------------------------------------------------
-- Mod list

function ModList:doDrawItem(y, item, alt)
    local d = item.item
    local h = self.itemheight
    if alt then self:drawRect(0, y, self.width, h, 0.25, 0.2, 0.2, 0.2) end
    if self.mouseoverselected == item.index then self:drawRect(0, y, self.width, h, 0.2, 1, 1, 1) end
    local col = d.color
    self:drawRect(10, y + h / 2 - 4, 8, 8, 1, col.r, col.g, col.b)
    local ty = y + (h - FONT_HGT_SMALL) / 2
    local nameCol = d.off and C.dim or { r = 0.95, g = 0.95, b = 0.95 }
    self:drawText(d.name, 28, ty, nameCol.r, nameCol.g, nameCol.b, 1, UIFont.Small)
    if d.tag then
        local tw = getTextManager():MeasureStringX(UIFont.Small, d.tag)
        local tx = self.width - tw - 26
        self:drawRect(tx - 6, y + 4, tw + 12, h - 8, 0.35, col.r * 0.5, col.g * 0.5, col.b * 0.5)
        self:drawText(d.tag, tx, ty, col.r, col.g, col.b, 1, UIFont.Small)
    end
    return y + h
end

---------------------------------------------------------------------------

function KnoxLinkScreen:new(x, y, w, h)
    local o = ISPanelJoypad:new(x, y, w, h)
    setmetatable(o, self)
    self.__index = self
    o.backgroundColor = { r = 0.02, g = 0.02, b = 0.03, a = 0.94 }
    o.borderColor = { r = 0, g = 0, b = 0, a = 0 }
    o.anchorRight = true
    o.anchorBottom = true
    o.stats = { ready = 0, off = 0, removed = 0 }
    return o
end

function KnoxLinkScreen:addRowLabel(text, y)
    local l = ISLabel:new(self.rightX + 14, y, BTN_HGT, text, 0.85, 0.85, 0.85, 1, UIFont.Small, true)
    self:addChild(l)
    return l
end

function KnoxLinkScreen:createChildren()
    local w, h = self.width, self.height
    local contentW = math.min(1280, w - PAD * 2)
    local x0 = (w - contentW) / 2
    self.x0, self.contentW = x0, contentW

    self.headerY = PAD
    self.cardsY = self.headerY + FONT_HGT_LARGE + FONT_HGT_SMALL + 18
    self.cardsH = FONT_HGT_LARGE + FONT_HGT_SMALL + 22
    local bodyY = self.cardsY + self.cardsH + GAP + 4
    local bottomY = h - PAD - BTN_HGT
    local bodyH = bottomY - GAP - bodyY

    local rightW = math.min(440, math.floor(contentW * 0.38))
    local leftW = contentW - rightW - GAP
    self.leftX, self.leftW = x0, leftW
    self.rightX, self.rightW = x0 + leftW + GAP, rightW
    self.bodyY, self.bodyH = bodyY, bodyH

    -- left: mod list card
    self.modList = ModList:new(x0 + 6, bodyY + FONT_HGT_SMALL + 16, leftW - 12, bodyH - FONT_HGT_SMALL - 22)
    self.modList:initialise()
    self.modList:instantiate()
    self.modList.itemheight = ROW_HGT
    self.modList.drawBorder = false
    self.modList.backgroundColor = { r = 0, g = 0, b = 0, a = 0 }
    self.modList.font = UIFont.Small
    self:addChild(self.modList)

    -- right: settings card
    local fx = self.rightX + 130
    local fw = rightW - 130 - 14
    local y = bodyY + FONT_HGT_SMALL + 18

    self:addRowLabel("World", y)
    self.worldCombo = ISComboBox:new(fx, y, fw, BTN_HGT, self, self.onWorldChanged)
    self.worldCombo:initialise()
    self:addChild(self.worldCombo)
    y = y + BTN_HGT + GAP

    self.spawnLabel = self:addRowLabel("Spawn town", y)
    self.spawnCombo = ISComboBox:new(fx, y, fw, BTN_HGT, self, nil)
    self.spawnCombo:initialise()
    self:addChild(self.spawnCombo)
    y = y + BTN_HGT + GAP

    self:addRowLabel("Options", y)
    self.ticks = ISTickBox:new(fx, y + 4, fw, FONT_HGT_SMALL + 6, "", self, nil)
    self.ticks:initialise()
    self:addChild(self.ticks)
    self.ticks:addOption("Lobby before the game starts")
    self.ticks:addOption("Everyone spawns together")
    self.ticks:addOption("PvP")
    self.ticks:addOption("Pause when nobody is online")
    y = y + self.ticks:getHeight() + GAP + 4

    self:addRowLabel("Max players", y)
    self.playersCombo = ISComboBox:new(fx, y, 110, BTN_HGT, self, nil)
    self.playersCombo:initialise()
    for _, n in ipairs(PLAYER_OPTIONS) do self.playersCombo:addOptionWithData(tostring(n), n) end
    self:addChild(self.playersCombo)
    y = y + BTN_HGT + GAP

    self:addRowLabel("Password", y)
    self.passwordEntry = ISTextEntryBox:new("", fx, y, fw, BTN_HGT)
    self.passwordEntry:initialise()
    self.passwordEntry:instantiate()
    self.passwordEntry.tooltip = "Optional. Steam friends still need it to join."
    self:addChild(self.passwordEntry)
    y = y + BTN_HGT + GAP

    self:addRowLabel("Server memory", y)
    self.memCombo = ISComboBox:new(fx, y, 110, BTN_HGT, self, nil)
    self.memCombo:initialise()
    for _, mb in ipairs(MEMORY_OPTIONS) do self.memCombo:addOptionWithData(tostring(mb / 1024) .. " GB", mb) end
    self:addChild(self.memCombo)
    y = y + BTN_HGT + GAP * 2
    self.notesY = y

    -- bottom bar
    local function button(title, bx, bw, fn, accept)
        local b = ISButton:new(bx, bottomY, bw, BTN_HGT, title, self, fn)
        b:initialise()
        b.font = UIFont.Medium
        if accept then b:enableAcceptColor() end
        self:addChild(b)
        return b
    end
    self.backButton = button("BACK", x0, 120, self.onBack)
    self.modsButton = button("MODS MENU", x0 + 120 + GAP, 150, self.onModsMenu)
    self.copyButton = button("COPY MOD LIST", x0 + 270 + GAP * 2, 180, self.onCopyList)
    self.copyButton.tooltip = "Copies the mod list (with Workshop links) so you can paste it to friends."
    self.startButton = button("START", x0 + contentW - 240, 240, self.onStart, true)
end

function KnoxLinkScreen:prerender()
    ISPanelJoypad.prerender(self)
    local x0, cw = self.x0, self.contentW

    -- header
    self:drawText("HOST WITH MODS", x0, self.headerY, 1, 1, 1, 1, UIFont.Large)
    self:drawText("Knox Link  |  " .. (self.subtitle or ""), x0, self.headerY + FONT_HGT_LARGE + 2, C.dim.r, C.dim.g, C.dim.b, 1, UIFont.Small)
    self:drawRect(x0, self.cardsY - 8, cw, 1, 0.25, 1, 1, 1)

    -- stat cards
    local cards = {
        { tostring(self.stats.ready), "mods ready", C.ok },
        { tostring(self.stats.off), "turned off (broken)", self.stats.off > 0 and C.bad or C.dim },
        { tostring(self.stats.removed), "gone from the Workshop", self.stats.removed > 0 and C.warn or C.dim },
        { self.javaOk and "ON" or "OFF", self.javaOk and "no re-downloads" or "server downloads mods", self.javaOk and C.ok or C.warn },
    }
    local cwEach = (cw - GAP * 3) / 4
    for i, c in ipairs(cards) do
        local cx = x0 + (i - 1) * (cwEach + GAP)
        drawCard(self, cx, self.cardsY, cwEach, self.cardsH)
        self:drawRect(cx, self.cardsY, cwEach, 3, 0.9, c[3].r, c[3].g, c[3].b)
        self:drawText(c[1], cx + 14, self.cardsY + 8, c[3].r, c[3].g, c[3].b, 1, UIFont.Large)
        self:drawText(c[2], cx + 14, self.cardsY + 10 + FONT_HGT_LARGE, 0.8, 0.8, 0.8, 1, UIFont.Small)
    end

    -- body cards
    drawCard(self, self.leftX, self.bodyY, self.leftW, self.bodyH, "MODS (" .. #self.modList.items .. ")")
    drawCard(self, self.rightX, self.bodyY, self.rightW, self.bodyH, "SERVER")
end

function KnoxLinkScreen:render()
    ISPanelJoypad.render(self)
    -- notes under the settings
    local y = self.notesY
    local maxY = self.bodyY + self.bodyH - FONT_HGT_SMALL - 6
    for _, n in ipairs(self.notes or {}) do
        if y > maxY then break end
        local lines = self:wrap(n.text, self.rightW - 28)
        for _, line in ipairs(lines) do
            if y > maxY then break end
            self:drawText(line, self.rightX + 14, y, n.color.r, n.color.g, n.color.b, 1, UIFont.Small)
            y = y + FONT_HGT_SMALL + 1
        end
        y = y + 6
    end
end

function KnoxLinkScreen:wrap(text, width)
    local lines, current = {}, ""
    for word in string.gmatch(text, "%S+") do
        local test = current == "" and word or (current .. " " .. word)
        if getTextManager():MeasureStringX(UIFont.Small, test) > width and current ~= "" then
            table.insert(lines, current)
            current = word
        else
            current = test
        end
    end
    if current ~= "" then table.insert(lines, current) end
    return lines
end

function KnoxLinkScreen:refresh()
    KnoxLink.mods = KnoxLink.scanMods()
    KnoxLink.broken = KnoxLink.findBroken(KnoxLink.mods)
    self.javaOk = javaHelperReady()
    self.prefs = KnoxLink.loadPrefs()

    -- worlds
    self.worldCombo:clear()
    local selected
    for _, name in ipairs(listWorlds()) do
        local exists = worldExists(name)
        self.worldCombo:addOptionWithData(name .. (exists and "" or "  (not started)"), name)
        if exists and (selected == nil or name == self.prefs.world) then selected = #self.worldCombo.options end
    end
    self.worldCombo:addOptionWithData("+ New world", NEW_WORLD)
    self.worldCombo.selected = selected or #self.worldCombo.options

    -- spawn towns (vanilla regions + map mods)
    self.spawnCombo:clear()
    self.spawnCombo:addOptionWithData("Random town", "random")
    local defaults = ServerSettings.new("__knoxlink_probe__")
    defaults:resetToDefault()
    self.regions = {}
    for i = 1, defaults:getNumSpawnRegions() do
        local name, file = defaults:getSpawnRegionName(i - 1), defaults:getSpawnRegionFile(i - 1)
        table.insert(self.regions, { name = name, file = file })
        self.spawnCombo:addOptionWithData(name, name)
    end
    self.spawnCombo:addOptionWithData("Let everyone pick", "pick")
    self.spawnCombo:selectData(self.prefs.spawn or "random")

    local function pref(key, default) local v = self.prefs[key]; if v == nil then return default end return v == "1" end
    self.ticks:setSelected(1, pref("lobby", true))
    self.ticks:setSelected(2, pref("together", true))
    self.ticks:setSelected(3, pref("pvp", false))
    self.ticks:setSelected(4, pref("pause", true))
    self.playersCombo:selectData(tonumber(self.prefs.players) or 8)
    self.memCombo:selectData(tonumber(self.prefs.memory) or 4096)
    self.passwordEntry:setText(self.prefs.password or "")

    self:onWorldChanged()
    self:updateInfo()
end

function KnoxLinkScreen:selectedWorld()
    return self.worldCombo:getOptionData(self.worldCombo.selected)
end

function KnoxLinkScreen:isNewWorld()
    local name = self:selectedWorld()
    return name == NEW_WORLD or not worldExists(name)
end

function KnoxLinkScreen:onWorldChanged()
    local isNew = self:isNewWorld()
    self.spawnCombo:setVisible(isNew)
    self.spawnLabel:setVisible(isNew)
    self.startButton:setTitle(isNew and "SET UP WORLD  >" or "START  >")
    self.subtitle = isNew and "new world: you'll get the normal world settings next" or ("continue " .. tostring(self:selectedWorld()))
end

function KnoxLinkScreen:updateInfo()
    local plan = KnoxLink.serverModPlan(self.javaOk)
    local removedSet, droppedSet = {}, {}
    for _, m in ipairs(plan.removed) do removedSet[m.id] = true end
    for _, m in ipairs(plan.dropped) do droppedSet[m.id] = true end

    self.modList:clear()
    local rows = {}
    for _, id in ipairs(plan.mods) do
        local m = KnoxLink.mods[id]
        if m and id ~= KnoxLink.MOD_ID then
            local row = { name = m.name, color = C.ok, tag = m.workshopID and "Workshop" or "Local" }
            if removedSet[id] then row.color, row.tag = C.warn, "Removed from Workshop" end
            if not m.workshopID then row.color = { r = 0.55, g = 0.75, b = 1 } end
            table.insert(rows, row)
        end
    end
    for _, m in ipairs(plan.dropped) do
        table.insert(rows, { name = m.name, color = C.bad, tag = "Can't load on server", off = true })
    end
    local offCount = 0
    for id, reason in pairs(KnoxLink.broken or {}) do
        local m = KnoxLink.mods[id]
        offCount = offCount + 1
        table.insert(rows, { name = (m and m.name or id) .. "  -  " .. reason, color = C.bad, tag = "Off", off = true })
    end
    for _, row in ipairs(rows) do self.modList:addItem(row.name, row) end

    self.stats.ready = #plan.mods - 1
    self.stats.off = offCount
    self.stats.removed = #plan.removed + #plan.dropped
    self.plan = plan

    self.notes = {}
    local function note(text, color) table.insert(self.notes, { text = text, color = color or C.dim }) end
    if self.stats.removed > 0 then
        note("Friends can't download mods that are gone from the Workshop. Hit COPY MOD LIST and send them the folders listed under \"copy these by hand\".", C.warn)
    end
    note("New Workshop mods switch on by themselves next time you open the game. Turn mods off in the Mods menu and they stay off.")
    note("Friends join from your name in the Steam friends list, or use Invite in the lobby.")
end

function KnoxLinkScreen:savePrefs()
    local p = self.prefs or {}
    local name = self:selectedWorld()
    p.world = name ~= NEW_WORLD and name or ""
    p.spawn = self.spawnCombo:getOptionData(self.spawnCombo.selected) or "random"
    p.lobby = self.ticks:isSelected(1) and "1" or "0"
    p.together = self.ticks:isSelected(2) and "1" or "0"
    p.pvp = self.ticks:isSelected(3) and "1" or "0"
    p.pause = self.ticks:isSelected(4) and "1" or "0"
    p.players = tostring(self.playersCombo:getOptionData(self.playersCombo.selected) or 8)
    p.memory = tostring(self.memCombo:getOptionData(self.memCombo.selected) or 4096)
    p.password = self.passwordEntry:getText() or ""
    KnoxLink.savePrefs(p)
    self.prefs = p
end

function KnoxLinkScreen:onBack()
    self:savePrefs()
    self:setVisible(false)
    MainScreen.instance.bottomPanel:setVisible(true)
end

function KnoxLinkScreen:onModsMenu()
    self:savePrefs()
    self:setVisible(false)
    MainScreen.onMenuItemMouseDownMainMenu(MainScreen.instance.modsOption, 0, 0)
end

function KnoxLinkScreen:onCopyList()
    local plan = self.plan or KnoxLink.serverModPlan(self.javaOk)
    local auto, manual = {}, {}
    local seenWid = {}
    for _, id in ipairs(plan.mods) do
        local m = KnoxLink.mods[id]
        if m then
            if m.workshopID and not KnoxLink.isRemoved(m.workshopID) then
                if not seenWid[m.workshopID] then
                    seenWid[m.workshopID] = true
                    table.insert(auto, m.name .. "  https://steamcommunity.com/sharedfiles/filedetails/?id=" .. m.workshopID)
                end
            else
                table.insert(manual, m.name .. "  (folder: " .. tostring(m.dir) .. ")")
            end
        end
    end
    local text = "Mods on my Project Zomboid server\n\nDownload automatically when you join (" .. #auto .. "):\n" .. table.concat(auto, "\n")
    if #manual > 0 then
        text = text .. "\n\nCopy these by hand into your Zomboid\\mods folder (" .. #manual .. "):\n" .. table.concat(manual, "\n")
    end
    Clipboard.setClipboard(text)
    self.copyButton:setTitle("COPIED!")
    self.copyFlash = getTimestampMs()
end

function KnoxLinkScreen:update()
    ISPanelJoypad.update(self)
    if self.copyFlash and getTimestampMs() - self.copyFlash > 2000 then
        self.copyFlash = nil
        self.copyButton:setTitle("COPY MOD LIST")
    end
end

---------------------------------------------------------------------------
-- Writing the server settings

local function setOption(settings, name, value)
    local opt = settings:getServerOptions():getOptionByName(name)
    if opt then opt:parse(tostring(value)) end
end

function KnoxLink.applyModsToSettings(settings, isNew)
    local plan = KnoxLink.serverModPlan(javaHelperReady())
    setOption(settings, "Mods", table.concat(plan.mods, ";"))
    setOption(settings, "WorkshopItems", table.concat(plan.workshop, ";"))

    local mapOpt = settings:getServerOptions():getOptionByName("Map")
    local maps, seen = {}, {}
    for _, id in ipairs(plan.mods) do
        local folders = getMapFoldersForMod(id)
        if folders then
            for j = 0, folders:size() - 1 do
                local folder = folders:get(j)
                if not seen[folder] then
                    seen[folder] = true
                    table.insert(maps, folder)
                    if isNew and spawnpointsExistsForMod(id, folder) then
                        settings:addSpawnRegion(folder, "media/maps/" .. folder .. "/spawnpoints.lua")
                    end
                end
            end
        end
    end
    if mapOpt then
        for _, folder in ipairs(KnoxLink.split(mapOpt:getValueAsString() or "", ";")) do
            if folder ~= "" and not seen[folder] then
                seen[folder] = true
                table.insert(maps, folder)
            end
        end
        if #maps == 0 then table.insert(maps, "Muldraugh, KY") end
        mapOpt:parse(table.concat(maps, ";"))
    end
    return plan
end

-- Pick one spawn point so every new character appears in the same spot.
function KnoxLink.pickSpawnPoint(settings, choice)
    local regions = {}
    for i = 1, settings:getNumSpawnRegions() do
        table.insert(regions, { name = settings:getSpawnRegionName(i - 1), file = settings:getSpawnRegionFile(i - 1) })
    end
    if choice ~= "random" then
        local filtered = {}
        for _, r in ipairs(regions) do if r.name == choice then table.insert(filtered, r) end end
        if #filtered > 0 then regions = filtered end
    end
    for _ = 1, 10 do
        if #regions == 0 then return nil end
        local idx = ZombRand(#regions) + 1
        local r = regions[idx]
        local ok, professions = pcall(function() return settings:loadSpawnPointsFile(r.file) end)
        if ok and professions then
            local points = professions.unemployed
            if not points then for _, v in pairs(professions) do points = v break end end
            if points and #points > 0 then
                local p = points[ZombRand(#points) + 1]
                local px, py = p.posX, p.posY
                if p.worldX then px = p.worldX * 300 + p.posX; py = p.worldY * 300 + p.posY end
                if px and py then return math.floor(px), math.floor(py), p.posZ or 0, r.name end
            end
        end
        table.remove(regions, idx)
    end
    return nil
end

function KnoxLinkScreen:onStart()
    self:savePrefs()
    local name = self:selectedWorld()
    local isNew = self:isNewWorld()
    if name == NEW_WORLD then name = nextFreeName() end

    local settings = ServerSettings.new(name)
    if not settings:isValid() then
        KnoxLink.showMessage("<SIZE:medium> Can't create the server settings <LINE> <SIZE:small> " .. tostring(settings:getErrorMsg()))
        return
    end
    if getServerSettingsManager():isValidNewName(name) then
        settings:resetToDefault()
        setOption(settings, "PublicName", "Knox Link")
        setOption(settings, "Public", "false")
    else
        settings:loadFiles()
    end
    KnoxLink.applyModsToSettings(settings, isNew)

    local p = self.prefs
    setOption(settings, "PVP", p.pvp == "1" and "true" or "false")
    setOption(settings, "PauseEmpty", p.pause == "1" and "true" or "false")
    setOption(settings, "MaxPlayers", p.players)
    setOption(settings, "Password", p.password)

    local town = ""
    if isNew then
        if p.together == "1" and p.spawn ~= "pick" then
            local x, y, z, regionName = KnoxLink.pickSpawnPoint(settings, p.spawn)
            if x then
                setOption(settings, "SpawnPoint", x .. "," .. y .. "," .. z)
                town = regionName
            end
        else
            setOption(settings, "SpawnPoint", "0,0,0")
        end
    end
    settings:saveFiles()

    -- lobby flag for the server: open|town
    KnoxLink.writeLines("lobby.txt", { (p.lobby == "1") and "1" or "0", town })

    self:setVisible(false)
    local mem = tonumber(p.memory) or 4096
    if isNew then
        KnoxLink.openWorldSetup(settings, function() KnoxLink.launch(name, mem) end)
    else
        KnoxLink.launch(name, mem)
    end
end

-- Regular world setup: the game's own server settings editor, opened straight on this world.
function KnoxLink.openWorldSetup(settings, onSaved)
    local sss = ServerSettingsScreen.instance
    local page = sss.pageEdit
    MainScreen.instance.bottomPanel:setVisible(false)
    sss.prevScreen = nil
    sss:aboutToShow()
    sss.pageStart:setVisible(false)
    page.settings = settings
    settings:loadFiles()
    page:aboutToShow()
    page:setVisible(true)
    sss:setVisible(true)

    local saveBtn, cancelBtn = page.buttonAccept, page.buttonCancel
    local oldSave, oldCancel = saveBtn.onclick, cancelBtn.onclick
    local oldTitle = saveBtn.title
    saveBtn:setTitle("START SERVER")
    local function restore()
        saveBtn.onclick, cancelBtn.onclick = oldSave, oldCancel
        saveBtn:setTitle(oldTitle)
        page:setVisible(false)
        sss:setVisible(false)
    end
    saveBtn.onclick = function()
        page:settingsFromUI()
        page.settings:saveFiles()
        restore()
        onSaved()
    end
    cancelBtn.onclick = function()
        restore()
        KnoxLink.openHostScreen()
    end
end

-- Start the hosted server through the game's own co-op host screen (it shows progress and connects you).
function KnoxLink.launch(name, memory)
    local coop = MainScreen.instance.onlineCoopScreen
    MainScreen.instance.bottomPanel:setVisible(false)
    coop:aboutToShow()
    CoopConnection.servername = name
    CoopConnection.memory = memory
    local combo = coop.settingsComboBox
    for i, opt in ipairs(combo.options) do
        local text = type(opt) == "table" and opt.text or opt
        if text == name then combo.selected = i end
    end
    if coop.memoryToIndex[memory] then coop.memoryComboBox.selected = coop.memoryToIndex[memory] end
    coop:checkWorldExists()
    coop:checkPlayerExists()
    coop:checkWorldVersion()
    coop:setVisible(true)
    coop:onStartButtonDown()
end

function KnoxLink.openHostScreen()
    local ms = MainScreen.instance
    if not ms.knoxLinkScreen then
        ms.knoxLinkScreen = KnoxLinkScreen:new(0, 0, ms.width, ms.height)
        ms.knoxLinkScreen:initialise()
        ms:addChild(ms.knoxLinkScreen)
    end
    ms.bottomPanel:setVisible(false)
    ms.knoxLinkScreen:setVisible(true)
    ms.knoxLinkScreen:refresh()
    KnoxLink.checkWorkshop(KnoxLink.mods, function()
        if ms.knoxLinkScreen:isVisible() then ms.knoxLinkScreen:updateInfo() end
    end)
end
