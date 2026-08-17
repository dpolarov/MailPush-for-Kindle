local ltn12 = require("ltn12")
local http = require("socket.http")
local socket = require("socket")
local socketutil = require("socketutil")

local M = {}

function M.download(url, path, max_bytes)
    local f, err = io.open(path, "wb")
    if not f then return false, "Cannot create update file: " .. tostring(err) end

    local total = 0
    local sink = function(chunk, sink_err)
        if chunk then
            total = total + #chunk
            if max_bytes and total > max_bytes then
                f:close()
                os.remove(path)
                return nil, "Update archive exceeds the size limit."
            end
            local ok, werr = f:write(chunk)
            if not ok then
                f:close()
                os.remove(path)
                return nil, "Cannot write update file: " .. tostring(werr)
            end
        end
        return 1
    end

    socketutil:set_timeout()
    local _, code, headers, status = socket.skip(1, http.request{
        url = url,
        method = "GET",
        headers = {
            ["User-Agent"] = "MailPush-for-Kindle updater",
            ["Accept"] = "application/octet-stream",
            ["Connection"] = "close",
        },
        sink = sink,
        redirect = true,
    })
    socketutil:reset_timeout()
    f:close()

    if tonumber(code) ~= 200 then
        os.remove(path)
        return false, "Update download failed: " .. tostring(status or code or "network error")
    end
    if headers and headers["content-length"] and max_bytes and tonumber(headers["content-length"]) and tonumber(headers["content-length"]) > max_bytes then
        os.remove(path)
        return false, "Update archive exceeds the size limit."
    end
    return true
end

return M
