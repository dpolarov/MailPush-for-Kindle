local ltn12 = require("ltn12")
local http = require("socket.http")
local socket = require("socket")
local socketutil = require("socketutil")

local M = {}

function M.download(url, path, max_bytes)
    local f, err = io.open(path, "wb")
    if not f then return false, "Cannot create update file: " .. tostring(err) end

    local total = 0
    local closed = false
    local function close_file()
        if not closed then f:close(); closed = true end
    end
    local sink = function(chunk, sink_err)
        if sink_err then
            close_file(); os.remove(path)
            return nil, sink_err
        end
        if chunk then
            total = total + #chunk
            if max_bytes and total > max_bytes then
                close_file(); os.remove(path)
                return nil, "Update archive exceeds the size limit."
            end
            local ok, werr = f:write(chunk)
            if not ok then
                close_file(); os.remove(path)
                return nil, "Cannot write update file: " .. tostring(werr)
            end
        end
        return 1
    end

    socketutil:set_timeout()
    local code, headers, status = socket.skip(1, http.request{
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
    close_file()

    if tonumber(code) ~= 200 then
        os.remove(path)
        return false, "Update download failed: " .. tostring(status or code or "network error")
    end
    local content_length = headers and tonumber(headers["content-length"])
    if content_length and max_bytes and content_length > max_bytes then
        os.remove(path)
        return false, "Update archive exceeds the size limit."
    end
    return true
end

return M
