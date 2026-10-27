-- Knox Link tracking, client side. Counts what this player does (walked, drove, bandaged a friend, read a
-- book...) and sends it to the server every ten in-game minutes. The server turns it into Knox goals and
-- the launcher's stats. Looks only: nothing here changes the game. Every hook is wrapped, so a game update
-- that renames something just stops that one count.
local MODULE = "KnoxLink"
local SEND_MINS = 10

local delta = {}
local last = nil -- last position, health and bites
local mins = 0
local startLevels = nil
local onceThisSession = {}

local function add(k, n)
    delta[k] = (delta[k] or 0) + (n or 1)
end
local function once(k)
    if onceThisSession[k] then return end
    onceThisSession[k] = true
    delta[k] = 1
end

local function me(c)
    local p = getPlayer()
    return p and c == p
end

-- hook: run fn after a timed action's method, when the method and class exist
local function hook(names, method, fn)
    for _, name in ipairs(names) do
        local cls = _G[name]
        if cls and type(cls[method]) == "function" and not cls["knox_" .. method] then
            local orig = cls[method]
            cls["knox_" .. method] = orig
            cls[method] = function(self, ...)
                local r = orig(self, ...)
                pcall(fn, self)
                return r
            end
        end
    end
end

local function roomName(p)
    local ok, name = pcall(function()
        local sq = p:getCurrentSquare()
        local room = sq and sq:getRoom()
        return room and room:getName() or ""
    end)
    return ok and string.lower(tostring(name or "")) or ""
end

local function bites(p)
    local ok, n = pcall(function()
        local parts = p:getBodyDamage():getBodyParts()
        local c = 0
        for i = 0, parts:size() - 1 do
            if parts:get(i):bitten() then c = c + 1 end
        end
        return c
    end)
    return ok and n or 0
end

local function perkLevels(p)
    local out = {}
    pcall(function()
        local list = PerkFactory.PerkList
        for i = 0, list:size() - 1 do
            local perk = list:get(i)
            out[tostring(perk)] = p:getPerkLevel(perk)
        end
    end)
    return out
end

local function hasGun(p)
    local ok, it = pcall(function()
        return p:getInventory():getFirstEvalRecurse(function(item)
            return instanceof(item, "HandWeapon") and item:isRanged()
        end)
    end)
    return ok and it ~= nil
end

local function everyMinute()
    if not isClient() then return end
    local p = getPlayer()
    if not p or p:isDead() then last = nil return end
    local x, y = p:getX(), p:getY()
    local health = 100
    pcall(function() health = p:getBodyDamage():getOverallBodyHealth() end)
    local b = bites(p)
    if last then
        local dx, dy = x - last.x, y - last.y
        local d = math.sqrt(dx * dx + dy * dy)
        local car = p:getVehicle()
        if d < 400 then -- a teleport is not a trip
            if car and car:getDriver() == p then
                add("drive", d)
                add("driveMins", 1)
            elseif not car and d < 40 then
                add("walk", d)
            end
        end
        if health < last.health then add("damage", last.health - health) end
        if b > last.bites then add("bites", b - last.bites) end
    end
    last = { x = x, y = y, health = health, bites = b }
    if p:isAsleep() then add("sleepMins", 1) end
    local room = roomName(p)
    if room ~= "" then
        local hour = getGameTime():getHour()
        if string.find(room, "church", 1, true) and (hour >= 22 or hour < 5) then once("church") end
        if string.find(room, "military", 1, true) or string.find(room, "army", 1, true) then once("military") end
    end
    mins = mins + 1
    if mins % SEND_MINS ~= 0 then return end
    if not startLevels then startLevels = perkLevels(p) end
    for k, lvl in pairs(perkLevels(p)) do
        if lvl >= 3 and lvl > (startLevels[k] or 0) then once("skill3") end
    end
    if hasGun(p) then once("gun") end
    local any = false
    for _, v in pairs(delta) do any = any or v > 0 end
    if any then
        sendClientCommand(p, MODULE, "track", delta)
        delta = {}
    end
end
Events.EveryOneMinute.Add(function() pcall(everyMinute) end)

-- kills with a blunt weapon
Events.OnZombieDead.Add(function(zombie)
    pcall(function()
        local p = getPlayer()
        if not p or zombie:getAttackedBy() ~= p then return end
        local w = p:getPrimaryHandItem()
        if w and instanceof(w, "HandWeapon") then
            local cats = w:getCategories()
            if cats and (cats:contains("Blunt") or cats:contains("SmallBlunt")) then add("blunt", 1) end
        end
    end)
end)

Events.OnGameStart.Add(function()
    local p = getPlayer()
    if p then startLevels = perkLevels(p) end
    -- bandaging someone else
    hook({ "ISApplyBandage" }, "perform", function(self)
        if me(self.character) and self.otherPlayer and self.otherPlayer ~= self.character and self.doIt ~= false then add("bandOthers", 1) end
    end)
    hook({ "ISActivateGenerator" }, "perform", function(self)
        if me(self.character) and self.activate then add("gen", 1) end
    end)
    -- adding to a dish counts as cooking a meal
    hook({ "ISAddItemInRecipe" }, "perform", function(self)
        if me(self.character) then add("cooked", 1) end
    end)
    hook({ "ISReadABook" }, "perform", function(self)
        if not me(self.character) or not self.item then return end
        local pages = self.item:getNumberOfPages()
        if not pages or pages <= 0 or self.character:getAlreadyReadPages(self.item:getFullType()) >= pages then add("books", 1) end
    end)
    hook({ "ISSeedAction", "ISSeedActionNew", "ISPlantSeedAction" }, "perform", function(self)
        if me(self.character) then add("planted", 1) end
    end)
    hook({ "ISHarvestPlantAction" }, "perform", function(self)
        if me(self.character) then add("harvested", 1) end
    end)
    hook({ "ISPutOutFire", "ISExtinguishAction", "ISPutOutFireAction" }, "perform", function(self)
        if me(self.character) then add("fireOut", 1) end
    end)
    hook({ "ISHandcraftAction", "ISCraftAction", "ISBuildAction" }, "perform", function(self)
        if me(self.character) then add("crafted", 1) end
    end)
    hook({ "ISRefuelFromGasPump", "ISAddGasolineToVehicle" }, "perform", function(self)
        if not me(self.character) or not self.part then return end
        if self.part:getContainerContentAmount() >= self.part:getContainerCapacity() - 0.5 then add("tank", 1) end
    end)
end)
