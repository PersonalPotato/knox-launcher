-- Knox Link lobby, client side.
-- As soon as your character is in the world you land on the lobby screen: you can't move,
-- zombies can't see or hurt you, and you can see who's ready and who's still making a character.
-- The host presses START, a 5 second countdown runs, and everyone starts at the same moment.
require "ISUI/ISPanel"

local MODULE = "KnoxLink"
local FONT_HGT_SMALL = getTextManager():getFontHeight(UIFont.Small)
local FONT_HGT_MEDIUM = getTextManager():getFontHeight(UIFont.Medium)
local FONT_HGT_LARGE = getTextManager():getFontHeight(UIFont.Large)
local FONT_HGT_TITLE = getTextManager():getFontHeight(UIFont.Title)
local ROW = math.max(30, FONT_HGT_MEDIUM + 10)

local C = {
    accent = { r = 0.95, g = 0.62, b = 0.18 },
    ok = { r = 0.45, g = 0.85, b = 0.45 },
    warn = { r = 0.98, g = 0.80, b = 0.30 },
    dim = { r = 0.6, g = 0.6, b = 0.6 },
}

local TIPS = {
    "Press Enter to chat with everyone while you wait.",
    "Everyone starts in the same spot, so stick together.",
    "The host can change mods from the main menu. They switch on for next time automatically.",
    "Can't find a friend? Hit INVITE FRIENDS and pick them from your Steam list.",
    "Nobody can be hurt or seen by zombies until the countdown ends.",
}

local Lobby = {
    open = false, isHost = false, host = "", town = "",
    players = {}, readySet = {}, connected = {},
    protected = {}, myReady = false, countdownEnd = nil, screen = nil,
}

local function quote(name) return "\"" .. tostring(name):gsub("\"", "") .. "\"" end

local function setProtected(name, on)
    local flag = on and "-true" or "-false"
    SendCommandToServer("/godmodeplayer " .. quote(name) .. " " .. flag)
    SendCommandToServer("/invisibleplayer " .. quote(name) .. " " .. flag)
end

local function freeze(on)
    local p = getPlayer()
    if p then p:setBlockMovement(on) end
end

---------------------------------------------------------------------------
-- Friends list (Steam invites)

KnoxLinkInvitePanel = ISPanel:derive("KnoxLinkInvitePanel")

function KnoxLinkInvitePanel:new(x, y, w, h)
    local o = ISPanel:new(x, y, w, h)
    setmetatable(o, self)
    self.__index = self
    o.backgroundColor = { r = 0.06, g = 0.06, b = 0.07, a = 0.97 }
    o.borderColor = { r = 1, g = 1, b = 1, a = 0.15 }
    o.invited = {}
    return o
end

function KnoxLinkInvitePanel:createChildren()
    self.list = ISScrollingListBox:new(8, FONT_HGT_MEDIUM + 16, self.width - 16, self.height - FONT_HGT_MEDIUM - 24)
    self.list:initialise()
    self.list:instantiate()
    self.list.itemheight = ROW
    self.list.drawBorder = false
    self.list.backgroundColor = { r = 0, g = 0, b = 0, a = 0 }
    local panel = self
    self.list.doDrawItem = function(list, y, item, alt)
        local f = item.item
        if list.mouseoverselected == item.index then list:drawRect(0, y, list.width, ROW, 0.2, 1, 1, 1) end
        local online = f:getState() ~= "Offline"
        local c = online and C.ok or C.dim
        list:drawRect(8, y + ROW / 2 - 4, 8, 8, 1, c.r, c.g, c.b)
        list:drawText(f:getName(), 24, y + (ROW - FONT_HGT_SMALL) / 2, 0.95, 0.95, 0.95, 1, UIFont.Small)
        local tag = panel.invited[f:getSteamID()] and "INVITED" or "INVITE"
        local tw = getTextManager():MeasureStringX(UIFont.Small, tag)
        local tc = panel.invited[f:getSteamID()] and C.ok or C.accent
        list:drawText(tag, list.width - tw - 22, y + (ROW - FONT_HGT_SMALL) / 2, tc.r, tc.g, tc.b, 1, UIFont.Small)
        return y + ROW
    end
    self.list:setOnMouseDownFunction(self, self.onFriend)
    self:addChild(self.list)
    self:fill()
end

function KnoxLinkInvitePanel:fill()
    self.list:clear()
    local friends = getFriendsList()
    if not friends then return end
    table.sort(friends, function(a, b)
        local ao, bo = a:getState() ~= "Offline", b:getState() ~= "Offline"
        if ao ~= bo then return ao end
        return a:getName() < b:getName()
    end)
    for _, f in ipairs(friends) do self.list:addItem(f:getName(), f) end
end

function KnoxLinkInvitePanel:onFriend(f)
    if not f then return end
    inviteFriend(f:getSteamID())
    self.invited[f:getSteamID()] = true
end

function KnoxLinkInvitePanel:prerender()
    ISPanel.prerender(self)
    self:drawText("INVITE FRIENDS", 12, 8, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Medium)
end

---------------------------------------------------------------------------
-- Lobby screen

KnoxLinkLobbyScreen = ISPanel:derive("KnoxLinkLobbyScreen")

function KnoxLinkLobbyScreen:new()
    local sw, sh = getCore():getScreenWidth(), getCore():getScreenHeight()
    local o = ISPanel:new(0, 0, sw, sh)
    setmetatable(o, self)
    self.__index = self
    o.backgroundColor = { r = 0, g = 0, b = 0, a = 0.82 }
    o.borderColor = { r = 0, g = 0, b = 0, a = 0 }
    o.cardW = math.min(620, sw - 40)
    o.cardH = math.min(560, sh - 120)
    o.cardX = (sw - o.cardW) / 2
    o.cardY = (sh - o.cardH) / 2
    o.tip = ZombRand(#TIPS) + 1
    o.tipTime = getTimestampMs()
    o.alpha = 0
    return o
end

function KnoxLinkLobbyScreen:createChildren()
    local bw = (self.cardW - 48) / 3
    local by = self.cardY + self.cardH - 18 - 32
    local function button(title, i, fn)
        local b = ISButton:new(self.cardX + 16 + (i - 1) * (bw + 8), by, bw, 32, title, self, fn)
        b:initialise()
        b.font = UIFont.Medium
        self:addChild(b)
        return b
    end
    self.readyButton = button("READY", 1, self.onReady)
    self.inviteButton = button("INVITE FRIENDS", 2, self.onInvite)
    self.startButton = button("START", 3, self.onStart)
    self.startButton:enableAcceptColor()

    self.invitePanel = KnoxLinkInvitePanel:new(self.cardX + self.cardW + 12, self.cardY, 300, self.cardH)
    self.invitePanel:initialise()
    if self.invitePanel.x + self.invitePanel.width > self.width then
        self.invitePanel:setX(self.width - self.invitePanel.width - 8)
    end
    self.invitePanel:setVisible(false)
    self:addChild(self.invitePanel)
end

function KnoxLinkLobbyScreen:onReady()
    Lobby.myReady = not Lobby.myReady
    sendClientCommand(getPlayer(), MODULE, "ready", { ready = Lobby.myReady })
end

function KnoxLinkLobbyScreen:onInvite()
    self.invitePanel:setVisible(not self.invitePanel:isVisible())
    if self.invitePanel:isVisible() then self.invitePanel:fill() end
end

function KnoxLinkLobbyScreen:onStart()
    if not Lobby.isHost then return end
    if Lobby.countdownEnd then
        sendClientCommand(getPlayer(), MODULE, "cancel", {})
    else
        sendClientCommand(getPlayer(), MODULE, "start", {})
    end
end

function KnoxLinkLobbyScreen:update()
    ISPanel.update(self)
    self.alpha = math.min(1, self.alpha + 0.05)
    local readyCount = 0
    for _, name in ipairs(Lobby.players) do if Lobby.readySet[name] then readyCount = readyCount + 1 end end
    self.readyCount = readyCount
    self.readyButton:setVisible(not Lobby.isHost)
    self.readyButton:setTitle(Lobby.myReady and "NOT READY" or "READY")
    if Lobby.myReady then self.readyButton:enableCancelColor() else self.readyButton:enableAcceptColor() end
    self.startButton:setVisible(Lobby.isHost)
    if Lobby.countdownEnd then
        self.startButton:setTitle("CANCEL")
        self.startButton:enableCancelColor()
    else
        self.startButton:setTitle("START (" .. readyCount .. "/" .. #Lobby.players .. " ready)")
        self.startButton:enableAcceptColor()
    end
    if getTimestampMs() - self.tipTime > 8000 then
        self.tip = self.tip % #TIPS + 1
        self.tipTime = getTimestampMs()
    end
    if ISChat and ISChat.instance and ISChat.instance:isVisible() then ISChat.instance:bringToTop() end
end

function KnoxLinkLobbyScreen:prerender()
    ISPanel.prerender(self)
    local x, y, w, h = self.cardX, self.cardY, self.cardW, self.cardH
    self:drawRect(x, y, w, h, 0.95, 0.07, 0.07, 0.08)
    self:drawRectBorder(x, y, w, h, 0.25, 1, 1, 1)
    self:drawRect(x, y, w, 4, 1, C.accent.r, C.accent.g, C.accent.b)

    local cy = y + 18
    self:drawTextCentre("LOBBY", x + w / 2, cy, 1, 1, 1, 1, UIFont.Title)
    cy = cy + FONT_HGT_TITLE + 2
    local sub = Lobby.town ~= "" and ("Everyone starts together in " .. Lobby.town) or "Everyone starts at the same moment"
    self:drawTextCentre(sub, x + w / 2, cy, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Medium)
    cy = cy + FONT_HGT_MEDIUM + 16
    self:drawRect(x + 16, cy, w - 32, 1, 0.15, 1, 1, 1)
    cy = cy + 10

    -- players: in the world, then still making a character
    local rows = {}
    for _, name in ipairs(Lobby.players) do
        local isHost = name == Lobby.host
        table.insert(rows, { name = name, host = isHost, ready = Lobby.readySet[name] })
    end
    local here = {}
    for _, name in ipairs(Lobby.players) do here[name] = true end
    for name, _ in pairs(Lobby.connected) do
        if not here[name] then table.insert(rows, { name = name, creating = true }) end
    end
    local maxRows = math.floor((y + h - 18 - 32 - FONT_HGT_SMALL * 2 - 20 - cy) / ROW)
    for i, r in ipairs(rows) do
        if i > maxRows then
            self:drawText("+ " .. (#rows - maxRows) .. " more", x + 28, cy, C.dim.r, C.dim.g, C.dim.b, 1, UIFont.Small)
            break
        end
        if i % 2 == 0 then self:drawRect(x + 16, cy, w - 32, ROW, 0.25, 0.2, 0.2, 0.2) end
        local ty = cy + (ROW - FONT_HGT_MEDIUM) / 2
        local status, sc
        if r.creating then status, sc = "making a character...", C.warn
        elseif r.ready then status, sc = "READY", C.ok
        else status, sc = "not ready", C.dim end
        self:drawRect(x + 28, cy + ROW / 2 - 5, 10, 10, 1, sc.r, sc.g, sc.b)
        self:drawText(r.name, x + 48, ty, 0.95, 0.95, 0.95, 1, UIFont.Medium)
        if r.host then
            local nw = getTextManager():MeasureStringX(UIFont.Medium, r.name)
            self:drawText("HOST", x + 56 + nw, cy + (ROW - FONT_HGT_SMALL) / 2, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Small)
        end
        local stw = getTextManager():MeasureStringX(UIFont.Small, status)
        self:drawText(status, x + w - 28 - stw, cy + (ROW - FONT_HGT_SMALL) / 2, sc.r, sc.g, sc.b, 1, UIFont.Small)
        cy = cy + ROW
    end

    -- footer
    local fy = y + h - 18 - 32 - FONT_HGT_SMALL * 2 - 14
    local hint = Lobby.isHost and "Press START when everyone is in. A 5 second countdown follows."
        or "Hit READY when you're set. The host starts the game."
    self:drawTextCentre(hint, x + w / 2, fy, 0.85, 0.85, 0.85, 1, UIFont.Small)
    self:drawTextCentre("Tip: " .. TIPS[self.tip], x + w / 2, fy + FONT_HGT_SMALL + 4, C.dim.r, C.dim.g, C.dim.b, 1, UIFont.Small)
end

function KnoxLinkLobbyScreen:render()
    ISPanel.render(self)
    if Lobby.countdownEnd then
        local left = math.max(0, Lobby.countdownEnd - getTimestampMs())
        local secs = math.ceil(left / 1000)
        local text = secs > 0 and tostring(secs) or "GO!"
        local pulse = 1 - (left % 1000) / 1000
        self:drawRect(0, self.height / 2 - 70, self.width, 140, 0.6 * (1 - pulse * 0.5), 0, 0, 0)
        self:drawTextCentre(text, self.width / 2, self.height / 2 - FONT_HGT_TITLE / 2, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Title)
        self:drawTextCentre("Get ready...", self.width / 2, self.height / 2 + FONT_HGT_TITLE / 2 + 4, 1, 1, 1, 0.9, UIFont.Medium)
    end
end

---------------------------------------------------------------------------
-- "GO!" toast after the lobby closes

KnoxLinkToast = ISUIElement:derive("KnoxLinkToast")
function KnoxLinkToast:new(text)
    local w = getTextManager():MeasureStringX(UIFont.Large, text) + 60
    local o = ISUIElement:new((getCore():getScreenWidth() - w) / 2, 90, w, FONT_HGT_LARGE + 24)
    setmetatable(o, self)
    self.__index = self
    o.text = text
    o.born = getTimestampMs()
    return o
end
function KnoxLinkToast:render()
    local age = getTimestampMs() - self.born
    local a = age < 3000 and 1 or math.max(0, 1 - (age - 3000) / 1000)
    if a <= 0 then self:removeFromUIManager() return end
    self:drawRect(0, 0, self.width, self.height, 0.8 * a, 0.07, 0.07, 0.08)
    self:drawRect(0, 0, self.width, 3, a, C.accent.r, C.accent.g, C.accent.b)
    self:drawTextCentre(self.text, self.width / 2, 12, 1, 1, 1, a, UIFont.Large)
end
local function toast(text)
    local t = KnoxLinkToast:new(text)
    t:initialise()
    t:addToUIManager()
end

---------------------------------------------------------------------------

---------------------------------------------------------------------------
-- Out of lives (or the whole squad is: THE END)

KnoxLinkOutScreen = ISPanel:derive("KnoxLinkOutScreen")
function KnoxLinkOutScreen:new()
    local o = ISPanel:new(0, 0, getCore():getScreenWidth(), getCore():getScreenHeight())
    setmetatable(o, self)
    self.__index = self
    o.backgroundColor = { r = 0, g = 0, b = 0, a = 0.88 }
    o.borderColor = { r = 0, g = 0, b = 0, a = 0 }
    o.moveWithMouse = false
    return o
end
function KnoxLinkOutScreen:createChildren()
    local w, h = 220, ROW + 6
    self.leave = ISButton:new((self.width - w) / 2, self.height / 2 + 110, w, h, "Leave the game", self, function()
        getCore():exitToMenu()
    end)
    self.leave:initialise()
    self:addChild(self.leave)
end
function KnoxLinkOutScreen:render()
    ISPanel.render(self)
    local title = Lobby.ended and "THE END" or "OUT OF LIVES"
    local cy = self.height / 2 - 80
    self:drawTextCentre(title, self.width / 2, cy, C.accent.r, C.accent.g, C.accent.b, 1, UIFont.Title)
    cy = cy + FONT_HGT_TITLE + 14
    local line = Lobby.ended and "Nobody made it. Start the next part from the Knox Launcher."
        or (tostring(Lobby.outName or "You") .. " is gone for good. The others keep going without you.")
    self:drawTextCentre(line, self.width / 2, cy, 1, 1, 1, 0.9, UIFont.Medium)
    if not Lobby.ended then
        self:drawTextCentre("You can stay and wait here, or leave.", self.width / 2, cy + FONT_HGT_MEDIUM + 8, C.dim.r, C.dim.g, C.dim.b, 1, UIFont.Small)
    end
end

local function showOut()
    if not Lobby.outScreen then
        Lobby.outScreen = KnoxLinkOutScreen:new()
        Lobby.outScreen:initialise()
        Lobby.outScreen:addToUIManager()
    end
    local p = getPlayer()
    if p then p:setBlockMovement(true) end
end

local function showScreen()
    if not Lobby.screen then
        Lobby.screen = KnoxLinkLobbyScreen:new()
        Lobby.screen:initialise()
        Lobby.screen:addToUIManager()
        freeze(true)
    end
end

local function hideScreen()
    if Lobby.screen then
        Lobby.screen:setVisible(false)
        Lobby.screen:removeFromUIManager()
        Lobby.screen = nil
    end
    freeze(false)
end

local function onServerCommand(module, command, args)
    if module ~= MODULE then return end
    if command == "state" then
        Lobby.open = args.open == true
        Lobby.host = args.host or ""
        Lobby.town = args.town or ""
        Lobby.players, Lobby.readySet = {}, {}
        for _, name in pairs(args.players or {}) do table.insert(Lobby.players, name) end
        table.sort(Lobby.players)
        for _, name in pairs(args.ready or {}) do Lobby.readySet[name] = true end
        if not Lobby.open then hideScreen() return end
        if Lobby.isHost then
            for _, name in ipairs(Lobby.players) do
                if not Lobby.protected[name] then
                    Lobby.protected[name] = true
                    setProtected(name, true)
                end
            end
        end
        showScreen()
    elseif command == "countdown" then
        Lobby.countdownEnd = getTimestampMs() + (args.ms or 5000)
        getSoundManager():playUISound("UIActivateMainMenuItem")
    elseif command == "cancelled" then
        Lobby.countdownEnd = nil
    elseif command == "started" then
        Lobby.open = false
        Lobby.countdownEnd = nil
        hideScreen()
        if Lobby.isHost then
            for name, _ in pairs(Lobby.protected) do setProtected(name, false) end
            Lobby.protected = {}
        end
        toast("GO! Good luck out there.")
        getSoundManager():playUISound("UIActivatePlayButton")
    elseif command == "mark" then
        -- flags on this character (new life seen, kit given, out of lives); saved with the character
        local p = getPlayer()
        if p and args and args.key then p:getModData()[args.key] = true end
    elseif command == "out" then
        Lobby.out = true
        Lobby.outName = args and args.name
        Lobby.ended = args and args.ended == true
        showOut()
    elseif command == "ended" then
        Lobby.ended = true
        if Lobby.out then showOut() end
    elseif command == "protect" then
        -- host only: hide a player who is out of lives
        if Lobby.isHost and args and args.name then setProtected(args.name, true) end
    elseif command == "toast" then
        if args and args.text then toast(args.text) end
    elseif command == "goal" then
        -- a Knox goal was finished (squad goals for everyone, personal ones for that player)
        local me = getPlayer() and getPlayer():getUsername() or ""
        if args and args.name and (args.who == "" or args.who == me) then
            toast("Goal: " .. args.name .. "  +" .. tostring(args.xp or 0) .. " Knox XP")
            getSoundManager():playUISound("UIActivateMainMenuItem")
        end
    elseif command == "respawn" then
        -- the host's respawn rule moved this new character (back to base, or next to the squad)
        local p = getPlayer()
        if p and args and args.x then
            local x, y, z = args.x + 0.5, args.y + 0.5, args.z or 0
            if not pcall(function() p:teleportTo(x, y, z) end) then
                p:setX(x) p:setY(y) p:setZ(z)
                p:setLastX(x) p:setLastY(y) p:setLastZ(z)
            end
            if args.text then toast(args.text) end
        end
    end
end
Events.OnServerCommand.Add(onServerCommand)

-- Who is connected but still making a character (from the scoreboard).
Events.OnScoreboardUpdate.Add(function(usernames)
    if not Lobby.open then return end
    Lobby.connected = {}
    for i = 0, usernames:size() - 1 do Lobby.connected[usernames:get(i)] = true end
end)

-- What this character looks like right now, in the launcher's Character shape (for tape covers).
local function colorOf(c)
    if not c then return nil end
    local ok, t = pcall(function() return { r = c:getRedFloat(), g = c:getGreenFloat(), b = c:getBlueFloat() } end)
    return ok and t or nil
end
local function lookOf(p)
    local look = { female = p:isFemale(), clothing = {}, clothingTint = {}, clothingTex = {} }
    pcall(function()
        local desc = p:getDescriptor()
        look.forename, look.surname = desc:getForename(), desc:getSurname()
        -- only plain values can go to the server: a Java object would fail the whole message
        local prof = desc:getProfession()
        look.profession = type(prof) == "string" and prof or tostring(prof)
    end)
    pcall(function()
        local hv = p:getHumanVisual()
        look.hair = hv:getHairModel()
        look.beard = hv:getBeardModel()
        look.hairColor = colorOf(hv:getHairColor())
        look.skinIndex = hv:getSkinTextureIndex()
    end)
    pcall(function()
        local worn = p:getWornItems()
        for i = 0, worn:size() - 1 do
            local w = worn:get(i)
            local item, loc = w:getItem(), tostring(w:getLocation())
            if item and loc then
                look.clothing[loc] = item:getFullType()
                local v = item:getVisual()
                if v then
                    local tint = colorOf(v:getTint())
                    if tint then look.clothingTint[loc] = tint end
                    local tex = v:getTextureChoice()
                    if tex and tex >= 0 then look.clothingTex[loc] = tex else
                        local base = v:getBaseTexture()
                        if base and base >= 0 then look.clothingTex[loc] = base end
                    end
                end
            end
        end
    end)
    return look
end

local LOOK_TICKS = 60 * 60 -- about once a minute
local tick = -1
Events.OnGameStart.Add(function()
    if isClient() then tick = 0 end
end)
-- a new character after a death says hello again, so the server can apply the respawn rule
Events.OnCreatePlayer.Add(function()
    if isClient() and tick > 30 then tick = 0 end
end)
Events.OnTick.Add(function()
    if tick < 0 then return end
    tick = tick + 1
    if tick == 30 then
        local p = getPlayer()
        if not p then tick = 0 return end
        Lobby.isHost = isCoopHost()
        local md = p:getModData()
        sendClientCommand(p, MODULE, "hello", { isHost = Lobby.isHost,
            knoxChar = md.knoxChar == true, knoxOut = md.knoxOut == true, knoxKit = md.knoxKit == true })
    end
    if tick > 30 and tick % LOOK_TICKS == 0 then
        local p = getPlayer()
        if p and not p:isDead() then
            local ok, look = pcall(lookOf, p)
            if ok then sendClientCommand(p, MODULE, "look", look) end
        end
    end
    if tick > 30 and Lobby.out then
        local p = getPlayer()
        if p then p:setBlockMovement(true) end
    end
    if tick > 30 and Lobby.open then
        if tick % 180 == 0 then scoreboardUpdate() end
        freeze(true)
    end
end)
