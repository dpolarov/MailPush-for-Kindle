local ConfirmBox = require("ui/widget/confirmbox")
local DataStorage = require("datastorage")
local InfoMessage = require("ui/widget/infomessage")
local JSON = require("json")
local NetworkMgr = require("ui/network/manager")
local UIManager = require("ui/uimanager")
local lfs = require("libs/libkoreader-lfs")
local logger = require("logger")

local Updater = {}

local REPO = "dpolarov/MailPush-for-Kindle"
local API_URL = "https://api.github.com/repos/" .. REPO .. "/releases/latest"
local ASSET_NAME = "mailpush.koplugin-bilingual.zip"
local PLUGIN_NAME = "mailpush.koplugin"
local MAX_ARCHIVE_BYTES = 32 * 1024 * 1024
local MAX_UNPACKED_BYTES = 64 * 1024 * 1024
local MAX_FILES = 256

local source = debug.getinfo(1, "S").source
local plugin_dir = source:match("^@(.+)/[^/]+$") or "."
local plugin_parent = plugin_dir:match("^(.*)/[^/]+$") or "."
local settings_dir = DataStorage:getSettingsDir() .. "/mailpush"
local update_state_path = settings_dir .. "/update_state.json"

local REQUIRED_FILES = {
    "VERSION",
    "_meta.lua",
    "main.lua",
    "updater.lua",
    "updater_download.lua", -- bridge for v1.1.x updater
    "config.default.json",
    "cacert.pem",
    "bin/mailpush",
}

local function shell_quote(s)
    return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

local function read_all(path)
    local f = io.open(path, "rb")
    if not f then return nil end
    local data = f:read("*all")
    f:close()
    return data
end

local function trim(s)
    return tostring(s or ""):match("^%s*(.-)%s*$")
end

local function normalize_version(v)
    return trim(v):gsub("^[vV]", "")
end

local function parse_version(v)
    local out = {}
    v = normalize_version(v)
    v = v:match("^[^+-]+") or v
    for part in v:gmatch("([^.]+)") do
        local n = tonumber(part)
        if not n then return nil end
        out[#out + 1] = n
    end
    if #out < 2 then return nil end
    return out
end

local function is_newer(latest, current)
    local a, b = parse_version(latest), parse_version(current)
    if not a or not b then return false end
    for i = 1, math.max(#a, #b) do
        local x, y = a[i] or 0, b[i] or 0
        if x ~= y then return x > y end
    end
    return false
end

local function installed_version()
    local v = read_all(plugin_dir .. "/VERSION")
    if v and trim(v) ~= "" then
        return trim(v)
    end

    -- Compatibility with v1.1.x packages, which stored the version only in
    -- the Go backend. This path is local-only; no updater networking uses Go.
    local binary = plugin_dir .. "/bin/mailpush"
    if read_all(binary) then
        local pipe = io.popen(shell_quote(binary) .. " version 2>/dev/null", "r")
        if pipe then
            local raw = pipe:read("*all") or ""
            pipe:close()
            local ok, decoded = pcall(JSON.decode, raw)
            if ok and type(decoded) == "table" and decoded.message then
                return trim(decoded.message)
            end
        end
    end
    return "unknown"
end

local function network_is_online()
    if NetworkMgr.isOnline then return NetworkMgr:isOnline() end
    if NetworkMgr.isConnected then return NetworkMgr:isConnected() end
    if NetworkMgr.isWifiOn then return NetworkMgr:isWifiOn() end
    return true
end

local function run_when_online(callback)
    if network_is_online() then return false end
    if NetworkMgr.runWhenOnline then
        NetworkMgr:runWhenOnline(callback)
        return true
    end
    return false
end

local function http_get_json(url, user_agent)
    local ok_require, http, ltn12, socket, socketutil = pcall(function()
        return require("socket/http"), require("ltn12"), require("socket"), require("socketutil")
    end)
    if not ok_require then
        return nil, "KOReader networking modules are unavailable."
    end

    local body = {}
    local ok_request, code, headers, status = pcall(function()
        socketutil:set_timeout(socketutil.LARGE_BLOCK_TIMEOUT, socketutil.LARGE_TOTAL_TIMEOUT)
        local c, h, s = socket.skip(1, http.request{
            url = url,
            method = "GET",
            headers = {
                ["User-Agent"] = user_agent,
                ["Accept"] = "application/vnd.github+json",
            },
            sink = ltn12.sink.table(body),
            redirect = true,
        })
        socketutil:reset_timeout()
        return c, h, s
    end)
    pcall(function() socketutil:reset_timeout() end)

    if not ok_request then
        return nil, "GitHub request failed: " .. tostring(code)
    end
    if tonumber(code) ~= 200 then
        return nil, "GitHub returned HTTP " .. tostring(code or status or "unknown") .. "."
    end

    local ok_json, decoded = pcall(JSON.decode, table.concat(body))
    if not ok_json or type(decoded) ~= "table" then
        return nil, "GitHub returned an invalid response."
    end
    return decoded
end

local function fetch_latest_release(current)
    local release, err = http_get_json(API_URL, "KOReader-MailPush/" .. tostring(current))
    if not release then return nil, err end
    if release.draft or release.prerelease then
        return nil, "The latest GitHub release is not a stable release."
    end
    if not release.tag_name then
        return nil, "GitHub release has no version tag."
    end

    local asset
    for _, item in ipairs(release.assets or {}) do
        if item.name == ASSET_NAME then
            asset = item
            break
        end
    end
    if not asset or not asset.browser_download_url then
        return nil, "Release " .. tostring(release.tag_name) .. " does not contain " .. ASSET_NAME .. "."
    end
    if not tostring(asset.browser_download_url):match("^https://") then
        return nil, "Update download URL is not HTTPS."
    end

    local size = tonumber(asset.size)
    if size and size > MAX_ARCHIVE_BYTES then
        return nil, "Update archive is larger than the safety limit."
    end

    return {
        version = tostring(release.tag_name),
        url = tostring(asset.browser_download_url),
        size = size,
        digest = asset.digest,
    }
end

local function download_file(url, dest, expected_size, user_agent)
    local ok_require, http, ltn12, socket, socketutil = pcall(function()
        return require("socket/http"), require("ltn12"), require("socket"), require("socketutil")
    end)
    if not ok_require then
        return false, "KOReader networking modules are unavailable."
    end

    local file, open_err = io.open(dest, "wb")
    if not file then return false, "Cannot create update file: " .. tostring(open_err) end

    local ok_request, code, headers, status = pcall(function()
        socketutil:set_timeout(socketutil.FILE_BLOCK_TIMEOUT, socketutil.FILE_TOTAL_TIMEOUT)
        local c, h, s = socket.skip(1, http.request{
            url = url,
            method = "GET",
            headers = {
                ["User-Agent"] = user_agent,
                ["Accept"] = "application/octet-stream",
            },
            sink = ltn12.sink.file(file),
            redirect = true,
        })
        socketutil:reset_timeout()
        return c, h, s
    end)
    pcall(function() socketutil:reset_timeout() end)
    pcall(function() file:close() end)

    if not ok_request or tonumber(code) ~= 200 then
        os.remove(dest)
        if not ok_request then
            return false, "Update download failed: " .. tostring(code)
        end
        return false, "Update download returned HTTP " .. tostring(code or status or "unknown") .. "."
    end

    local attr = lfs.attributes(dest)
    local actual_size = attr and tonumber(attr.size) or 0
    if actual_size <= 0 then
        os.remove(dest)
        return false, "Downloaded update is empty."
    end
    if actual_size > MAX_ARCHIVE_BYTES then
        os.remove(dest)
        return false, "Downloaded update exceeds the safety limit."
    end
    if expected_size and actual_size ~= expected_size then
        os.remove(dest)
        return false, string.format("Downloaded update is incomplete (%d of %d bytes).", actual_size, expected_size)
    end
    return true
end

local function rm_rf(path)
    os.execute("rm -rf " .. shell_quote(path))
end

local function mkdir_p(path)
    local ret = os.execute("mkdir -p " .. shell_quote(path))
    return ret == true or ret == 0
end

local function normalize_archive_path(path)
    local p = tostring(path or ""):gsub("\\", "/"):gsub("/+", "/")
    while p:sub(1, 2) == "./" do p = p:sub(3) end
    return p
end

local function safe_relative_path(path)
    if path == "" or path:sub(1, 1) == "/" then return false end
    if path == ".." or path:sub(1, 3) == "../" or path:find("/../", 1, true) then return false end
    if path:find("%z") then return false end
    return true
end

local function unpack_to_staging(zip_path, staged_plugin)
    local ok_archiver, Archiver = pcall(require, "ffi/archiver")
    if not (ok_archiver and Archiver and Archiver.Reader) then
        return false, "KOReader archive extractor is unavailable."
    end
    if not mkdir_p(staged_plugin) then
        return false, "Cannot create update staging directory."
    end

    local arc = Archiver.Reader:new()
    if not arc:open(zip_path) then
        local err = arc.err
        arc:close()
        return false, "Cannot open update archive: " .. tostring(err or "unknown error")
    end

    local root = PLUGIN_NAME .. "/"
    local count, total = 0, 0
    local seen_file = false
    local failure
    for entry in arc:iterate() do
        count = count + 1
        local p = normalize_archive_path(entry.path)
        local size = tonumber(entry.size or 0) or 0
        total = total + size

        if count > MAX_FILES then
            failure = "Update archive contains too many files."
            break
        end
        if total > MAX_UNPACKED_BYTES then
            failure = "Update archive is too large after unpacking."
            break
        end
        if entry.mode ~= "file" and entry.mode ~= "directory" then
            failure = "Update archive contains an unsupported file type: " .. p
            break
        end
        if p ~= PLUGIN_NAME and p ~= root and p:sub(1, #root) ~= root then
            failure = "Update archive has an unexpected layout: " .. p
            break
        end

        local rel = p:sub(#root + 1)
        if rel ~= "" then
            if not safe_relative_path(rel) then
                failure = "Update archive contains an unsafe path: " .. rel
                break
            end
            local dest = staged_plugin .. "/" .. rel
            if not arc:extractToPath(entry.path, dest) then
                failure = "Cannot extract " .. rel .. ": " .. tostring(arc.err or "unknown error")
                break
            end
            if entry.mode == "file" then seen_file = true end
        end
    end
    arc:close()

    if failure then return false, failure end
    if not seen_file then return false, "Update archive is empty." end
    return true
end

local function verify_staged_plugin(staged_plugin, expected_version)
    for _, rel in ipairs(REQUIRED_FILES) do
        local attr = lfs.attributes(staged_plugin .. "/" .. rel)
        if not attr or attr.mode ~= "file" then
            return false, "Update archive is missing required file: " .. rel
        end
    end

    local staged_version = normalize_version(read_all(staged_plugin .. "/VERSION"))
    local wanted_version = normalize_version(expected_version)
    if staged_version == "" or staged_version ~= wanted_version then
        return false, "Update package version does not match the GitHub release."
    end

    local binary = staged_plugin .. "/bin/mailpush"
    os.execute("chmod 755 " .. shell_quote(binary))
    local pipe = io.popen(shell_quote(binary) .. " version 2>&1", "r")
    if not pipe then return false, "Cannot run the updated MailPush backend." end
    local raw = pipe:read("*all") or ""
    pipe:close()
    local ok_json, result = pcall(JSON.decode, raw)
    if not ok_json or type(result) ~= "table" or normalize_version(result.message) ~= wanted_version then
        return false, "Updated backend self-test failed: " .. trim(raw)
    end
    return true
end

local function clear_snooze()
    os.remove(update_state_path)
end

function Updater.install(plugin, release)
    if run_when_online(function() Updater.install(plugin, release) end) then return end
    if not network_is_online() then
        plugin:show("No network connection. Please connect to Wi-Fi first.", true)
        return
    end

    UIManager:show(InfoMessage:new{ text = "Downloading MailPush " .. tostring(release.version) .. "…", timeout = 2 })
    UIManager:forceRePaint()

    UIManager:scheduleIn(0.1, function()
        local work = plugin_parent .. "/.mailpush-update"
        local zip_path = work .. "/update.zip"
        local staged_plugin = work .. "/staged/" .. PLUGIN_NAME
        local backup = plugin_dir .. ".previous"

        rm_rf(work)
        if not mkdir_p(work .. "/staged") then
            plugin:show("Cannot create the update staging directory.", true)
            return
        end

        local downloaded, download_err = download_file(
            release.url,
            zip_path,
            release.size,
            "KOReader-MailPush/" .. tostring(installed_version()))
        if not downloaded then
            rm_rf(work)
            plugin:show(download_err or "Update download failed.", true)
            return
        end

        local unpacked, unpack_err = unpack_to_staging(zip_path, staged_plugin)
        if not unpacked then
            rm_rf(work)
            plugin:show(unpack_err or "Update extraction failed.", true)
            return
        end

        local verified, verify_err = verify_staged_plugin(staged_plugin, release.version)
        if not verified then
            rm_rf(work)
            plugin:show(verify_err or "Update verification failed.", true)
            return
        end

        -- Only now touch the working plugin. Both directories are under the
        -- same plugins filesystem, so os.rename is atomic on Kindle/Kobo.
        rm_rf(backup)
        local moved_old, move_old_err = os.rename(plugin_dir, backup)
        if not moved_old then
            rm_rf(work)
            plugin:show("Cannot create plugin backup: " .. tostring(move_old_err), true)
            return
        end

        local moved_new, move_new_err = os.rename(staged_plugin, plugin_dir)
        if not moved_new then
            os.rename(backup, plugin_dir)
            rm_rf(work)
            plugin:show("Cannot install update; previous version was restored: " .. tostring(move_new_err), true)
            return
        end

        rm_rf(work)
        clear_snooze()
        logger.warn("MailPush updater: installed", release.version, "backup left at", backup)
        UIManager:show(ConfirmBox:new{
            text = "MailPush " .. tostring(release.version) .. " was installed successfully.\n\nRestart KOReader now to load the new version?",
            ok_text = "Restart",
            cancel_text = "Later",
            ok_callback = function() UIManager:restartKOReader() end,
        })
    end)
end

function Updater.check(plugin, force)
    if not force and plugin.update_snoozed and plugin:update_snoozed() then return end

    if run_when_online(function() Updater.check(plugin, force) end) then return end
    if not network_is_online() then
        if force then plugin:show("No network connection. Please connect to Wi-Fi first.", true) end
        return
    end

    local current = installed_version()
    if force then
        UIManager:show(InfoMessage:new{ text = "Checking for MailPush updates…", timeout = 1 })
        UIManager:forceRePaint()
    end

    UIManager:scheduleIn(0.1, function()
        local release, err = fetch_latest_release(current)
        if not release then
            logger.warn("MailPush updater: update check failed:", err)
            if force then plugin:show(err or "Could not check for updates.", true) end
            return
        end

        if not is_newer(release.version, current) then
            if force then plugin:show("MailPush is up to date (" .. tostring(current) .. ").") end
            return
        end

        UIManager:show(ConfirmBox:new{
            text = "A new MailPush version is available.\n\nInstalled: " .. tostring(current) ..
                "\nAvailable: " .. tostring(release.version) .. "\n\nDownload and install it now?",
            ok_text = "Update",
            cancel_text = "Not now",
            ok_callback = function() Updater.install(plugin, release) end,
            cancel_callback = function()
                if plugin.snooze_updates then plugin:snooze_updates() end
                plugin:show("Update reminder postponed for 30 days.")
            end,
        })
    end)
end

Updater._test = {
    normalize_version = normalize_version,
    parse_version = parse_version,
    is_newer = is_newer,
    normalize_archive_path = normalize_archive_path,
    safe_relative_path = safe_relative_path,
}

return Updater
