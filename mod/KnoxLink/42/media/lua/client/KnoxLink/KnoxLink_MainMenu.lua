-- Knox Link: main menu entry and the automatic mod sync on reaching the main menu.
require "OptionScreens/MainScreen"
require "KnoxLink/KnoxLink_Core"

local MENU_TEXT = "HOST WITH MODS"

local function addMenuItem(self)
    if self.inGame or not self.onlineCoopOption or self.knoxLinkOption then return end
    local anchor = self.onlineCoopOption
    local labelHgt = anchor:getHeight()
    local y = anchor:getBottom()
    for _, child in pairs(self.bottomPanel:getChildren()) do
        if child ~= anchor and child.Type == "ISLabel" and child:getY() >= y then
            child:setY(child:getY() + labelHgt)
        end
    end
    local label = ISLabel:new(anchor:getX(), y, labelHgt, MENU_TEXT, 1, 1, 1, 1, UIFont.Large, true)
    label.internal = "KNOXLINK"
    label:initialise()
    label.onMouseDown = MainScreen.onMenuItemMouseDownMainMenu
    label.fade = UITransition.new()
    label.fade:setFadeIn(false)
    label.prerender = MainScreen.prerenderBottomPanelLabel
    label:setWidth(anchor:getWidth())
    self.bottomPanel:addChild(label)
    label:setVisible(anchor:isVisible())
    self.knoxLinkOption = label
    self.bottomPanel:setHeight(self.bottomPanel:getHeight() + labelHgt)
end

local original_instantiate = MainScreen.instantiate
function MainScreen:instantiate()
    original_instantiate(self)
    addMenuItem(self)
end

local original_startNormal = MainScreen.startNormalMainScreen
function MainScreen:startNormalMainScreen()
    original_startNormal(self)
    if self.knoxLinkOption then self.knoxLinkOption:setVisible(true) end
end

local original_onGainJoypadFocus = MainScreen.onGainJoypadFocus
function MainScreen:onGainJoypadFocus(joypadData)
    original_onGainJoypadFocus(self, joypadData)
    if self.knoxLinkOption and self.knoxLinkOption:isVisible() and self.joypadButtonsY then
        for k, v in ipairs(self.joypadButtonsY) do
            if v[1] == self.onlineCoopOption then
                table.insert(self.joypadButtonsY, k + 1, { self.knoxLinkOption })
                break
            end
        end
    end
end

local original_onMenuItem = MainScreen.onMenuItemMouseDownMainMenu
MainScreen.onMenuItemMouseDownMainMenu = function(item, x, y)
    if item.internal == "KNOXLINK" then
        getSoundManager():playUISound("UIActivateMainMenuItem")
        KnoxLink.openHostScreen()
        return
    end
    return original_onMenuItem(item, x, y)
end

-- Automatic mod sync, a moment after the main menu appears.

local ticks = 0
local function onFETick()
    if not MainScreen.instance or MainScreen.instance.inGame then return end
    ticks = ticks + 1
    if ticks < 30 then return end
    Events.OnFETick.Remove(onFETick)

    KnoxLink.loadWorkshopStatus()
    if not getCore():getOptionModsEnabled() then
        KnoxLink.log("mods are disabled in the game options, skipping auto mod sync")
        return
    end
    -- Knox Launcher already picked the mods for this launch
    if KnoxLink.App and KnoxLink.App.pendingLaunch() then
        KnoxLink.mods = KnoxLink.scanMods()
        KnoxLink.broken = KnoxLink.findBroken(KnoxLink.mods)
        KnoxLink.App.onMenuReady()
        return
    end
    local ok, changed, _, key = pcall(KnoxLink.syncMods)
    if not ok then
        KnoxLink.log("mod sync failed: " .. tostring(changed))
        return
    end
    KnoxLink.checkWorkshop(KnoxLink.mods)
    local guard = KnoxLink.readLines("reload_guard.txt")
    if changed then
        -- guard against a reload loop if the game rewrites the list differently than we asked
        if guard and guard[1] == key then
            KnoxLink.log("mod list did not stick after a reload, not reloading again")
            return
        end
        KnoxLink.writeLines("reload_guard.txt", { key })
        KnoxLink.writeLines("show_report.txt", { "1" })
        MainScreen.instance.bottomPanel:setVisible(false)
        getCore():ResetLua("default", "modsChanged")
        return
    end
    if guard and guard[1] ~= "" then KnoxLink.writeLines("reload_guard.txt", { "" }) end
    local show = KnoxLink.readLines("show_report.txt")
    if show and show[1] == "1" then
        KnoxLink.writeLines("show_report.txt", { "0" })
        KnoxLink.showReportPopup()
    end
    if KnoxLink.App then KnoxLink.App.onMenuReady() end
end
Events.OnFETick.Add(onFETick)

function KnoxLink.showReportPopup()
    local lines = KnoxLink.readLines("report.txt") or {}
    local on, off = {}, {}
    for _, line in ipairs(lines) do
        local p = KnoxLink.split(line, "|")
        if p[1] == "on" then table.insert(on, tostring(p[3])) end
        if p[1] == "off" or p[1] == "skip" then table.insert(off, " <LINE> - " .. tostring(p[3]) .. ": " .. tostring(p[4])) end
    end
    if #on == 0 and #off == 0 then return end
    local text = "<SIZE:medium> Knox Link updated your mods <LINE> <SIZE:small> <LINE> "
    if #on > 0 then
        text = text .. "Turned on " .. #on .. " mod" .. (#on == 1 and "" or "s") .. ": " .. table.concat(on, ", ") .. " <LINE> <LINE> "
    end
    if #off > 0 then
        text = text .. "Turned off " .. #off .. " broken mod" .. (#off == 1 and "" or "s") .. ":" .. table.concat(off, "") .. " <LINE> "
    end
    KnoxLink.showMessage(text)
end

function KnoxLink.showMessage(richText)
    local w = math.min(700, getCore():getScreenWidth() - 40)
    local h = math.min(500, getCore():getScreenHeight() - 40)
    local modal = ISModalRichText:new((getCore():getScreenWidth() - w) / 2, (getCore():getScreenHeight() - h) / 2, w, h, richText, false)
    modal:initialise()
    modal:addToUIManager()
    modal:setAlwaysOnTop(true)
end
