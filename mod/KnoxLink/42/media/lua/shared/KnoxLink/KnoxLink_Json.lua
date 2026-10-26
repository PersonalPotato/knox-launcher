-- Tiny JSON writer shared by the client bridge and the server (the Knox Launcher reads these files).
KnoxLinkJson = KnoxLinkJson or {}

local function jsonString(s)
    s = tostring(s or "")
    s = s:gsub("\\", "\\\\"):gsub("\"", "\\\""):gsub("\n", "\\n"):gsub("\r", "\\r"):gsub("\t", "\\t")
    s = s:gsub("[%z\1-\31]", "")
    return "\"" .. s .. "\""
end

local function isArray(t)
    if t.__array then return true end
    local n = 0
    for k, _ in pairs(t) do
        if type(k) ~= "number" then return false end
        n = n + 1
    end
    return n > 0 and n == #t
end

local function encode(v, out)
    local tv = type(v)
    if tv == "table" then
        if isArray(v) then
            table.insert(out, "[")
            for i = 1, #v do
                if i > 1 then table.insert(out, ",") end
                encode(v[i], out)
            end
            table.insert(out, "]")
        else
            table.insert(out, "{")
            local first = true
            for k, val in pairs(v) do
                if k ~= "__array" then
                    if not first then table.insert(out, ",") end
                    first = false
                    table.insert(out, jsonString(k))
                    table.insert(out, ":")
                    encode(val, out)
                end
            end
            table.insert(out, "}")
        end
    elseif tv == "number" then
        if v ~= v or v == math.huge or v == -math.huge then table.insert(out, "0")
        else table.insert(out, tostring(v)) end
    elseif tv == "boolean" then
        table.insert(out, v and "true" or "false")
    elseif v == nil then
        table.insert(out, "null")
    else
        table.insert(out, jsonString(v))
    end
end

function KnoxLinkJson.encode(v)
    local out = {}
    encode(v, out)
    return table.concat(out)
end

function KnoxLinkJson.array()
    return { __array = true }
end

function KnoxLinkJson.write(name, data)
    local writer = getFileWriter("KnoxLink/" .. name, true, false)
    if not writer then return end
    writer:write(KnoxLinkJson.encode(data))
    writer:close()
end

function KnoxLinkJson.split(s, sep)
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
