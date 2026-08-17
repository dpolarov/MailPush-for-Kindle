local ConfirmBox = require("ui/widget/confirmbox")
local DataStorage = require("datastorage")
local Device = require("device")
local InfoMessage = require("ui/widget/infomessage")
local InputDialog = require("ui/widget/inputdialog")
local JSON = require("json")
local UIManager = require("ui/uimanager")
local WidgetContainer = require("ui/widget/container/widgetcontainer")
local Archiver = require("ffi/archiver")

local source = debug.getinfo(1, "S").source
local plugin_dir = source:match("^@(.+)/[^/]+$") or "."
local settings_dir = DataStorage:getSettingsDir() .. "/mailpush"
local config_path = settings_dir .. "/config.json"
local state_path = settings_dir .. "/state.json"
local update_state_path = settings_dir .. "/update_state.json"
local last_result_path = settings_dir .. "/last_result.json"
local binary_path = plugin_dir .. "/bin/mailpush"
local ca_bundle_path = plugin_dir .. "/cacert.pem"
local default_config_path = plugin_dir .. "/config.default.json"
local UPDATE_SNOOZE_SECONDS = 30 * 24 * 60 * 60
local UPDATE_MAX_BYTES = 32 * 1024 * 1024
local UPDATE_MAX_UNPACKED = 64 * 1024 * 1024
local UPDATE_MAX_FILES = 256

local MailPush = WidgetContainer:extend{ name = "mailpush", is_doc_only = false }

local function shell_quote(s) return "'" .. tostring(s):gsub("'", "'\\''") .. "'" end
local function read_all(path)
    local f = io.open(path, "rb"); if not f then return nil end
    local s = f:read("*all"); f:close(); return s
end
local function write_atomic(path, data)
    local tmp = path .. ".tmp"
    local f, err = io.open(tmp, "wb"); if not f then return false, err end
    local ok, werr = f:write(data); f:close()
    if not ok then os.remove(tmp); return false, werr end
    os.execute("chmod 600 " .. shell_quote(tmp))
    local renamed, rerr = os.rename(tmp, path)
    if not renamed then os.remove(tmp); return false, rerr end
    return true
end
local function ensure_settings()
    os.execute("mkdir -p " .. shell_quote(settings_dir)); os.execute("chmod 700 " .. shell_quote(settings_dir))
    if not read_all(config_path) then local d = read_all(default_config_path); if d then write_atomic(config_path, d) end end
end
local function decode_file(path)
    local raw = read_all(path); if not raw then return nil end
    if raw:sub(1,3) == "\239\187\191" then raw = raw:sub(4) end
    local ok, v = pcall(JSON.decode, raw); if ok and type(v) == "table" then return v end
    return nil
end
local function load_config()
    ensure_settings(); local cfg = decode_file(config_path)
    if not cfg then return nil, "Configuration file is missing or is not valid JSON." end
    return cfg
end
local function save_config(cfg)
    local ok, encoded = pcall(JSON.encode, cfg); if not ok then return false, "Cannot encode configuration." end
    local saved, err = write_atomic(config_path, encoded .. "\n")
    if not saved then return false, "Cannot save configuration: " .. tostring(err) end
    return true
end
local function basename(p) return tostring(p):match("([^/]+)$") or tostring(p) end
local function rm_rf(path) os.execute("rm -rf " .. shell_quote(path)) end
local function normalize_archive_path(path)
    local p = tostring(path or ""):gsub("\\", "/"):gsub("/+", "/")
    while p:sub(1,2) == "./" do p = p:sub(3) end
    return p
end

function MailPush:show(text, warning)
    UIManager:show(InfoMessage:new{ text = text, icon = warning and "notice-warning" or nil })
end

function MailPush:backend(command, extra_args, quiet)
    if not read_all(binary_path) then self:show("MailPush backend is missing. Reinstall the plugin.", true); return nil end
    local parts = { shell_quote(binary_path), "--config", shell_quote(config_path), "--state", shell_quote(state_path), "--ca-bundle", shell_quote(ca_bundle_path) }
    for _, a in ipairs(extra_args or {}) do table.insert(parts, a) end
    table.insert(parts, command); table.insert(parts, "2>&1")
    if not quiet then
        local text = command == "test" and "Testing IMAP connection…" or "Checking mail…"
        self:show(text)
    end
    local pipe = io.popen(table.concat(parts, " "), "r")
    if not pipe then self:show("Cannot start MailPush backend.", true); return nil end
    local output = pipe:read("*all") or ""; pipe:close(); write_atomic(last_result_path, output)
    local ok, result = pcall(JSON.decode, output)
    if not ok or type(result) ~= "table" then self:show("MailPush returned an invalid response.\n\n" .. output, true); return nil end
    return result
end

local function hint_for(text)
    local s = string.lower(tostring(text or ""))
    if s:find("authentication failed",1,true) then return "Hint: Check the username and use an app password if your mail provider requires one." end
    if s:find("certificate",1,true) or s:find("x509",1,true) then return "Hint: Check the Kindle date/time and CA settings." end
    if s:find("cannot connect",1,true) or s:find("connection refused",1,true) then return "Hint: Check Wi-Fi, host, port, and whether IMAP is enabled." end
    if s:find("timeout",1,true) then return "Hint: Check Wi-Fi quality and the server address." end
    if s:find("outside configured root",1,true) or s:find("symbolic link",1,true) then return "Hint: Keep the save path inside Allowed root directory." end
    if s:find("size limit",1,true) or s:find("too large",1,true) then return "Hint: A safety size limit blocked this item." end
    return nil
end
function MailPush:show_result(result)
    if not result then return end
    local lines = { result.message or (result.ok and "Done." or "Operation failed.") }
    local files = result.downloaded or {}
    if #files > 0 then table.insert(lines,""); table.insert(lines,"Downloaded files:"); for i=1,math.min(#files,12) do table.insert(lines,"• "..basename(files[i])) end; if #files>12 then table.insert(lines,string.format("…and %d more.",#files-12)) end end
    local errors = result.errors or {}
    if #errors > 0 then table.insert(lines,""); table.insert(lines,"Errors:"); for i=1,math.min(#errors,8) do table.insert(lines,"• "..tostring(errors[i])) end end
    if not result.ok then local h=hint_for((result.message or "").." "..table.concat(errors," ")); if h then table.insert(lines,""); table.insert(lines,h) end end
    self:show(table.concat(lines,"\n"), not result.ok)
end

function MailPush:update_snoozed()
    local st = decode_file(update_state_path) or {}
    return tonumber(st.snooze_until or 0) > os.time()
end
function MailPush:snooze_updates()
    write_atomic(update_state_path, JSON.encode({ snooze_until = os.time() + UPDATE_SNOOZE_SECONDS }) .. "\n")
end

function MailPush:validate_update_archive(path)
    local arc = Archiver.Reader:new()
    if not arc:open(path) then return false, "Cannot open downloaded update archive." end
    local count, total = 0, 0
    local ok, err = true, nil
    for entry in arc:iterate() do
        count = count + 1
        local p = normalize_archive_path(entry.path)
        local size = tonumber(entry.size or 0) or 0
        total = total + size
        if count > UPDATE_MAX_FILES then ok, err = false, "Update archive contains too many files."; break end
        if total > UPDATE_MAX_UNPACKED then ok, err = false, "Update archive is too large after unpacking."; break end
        if p:sub(1,1) == "/" or p == ".." or p:sub(1,3) == "../" or p:find("/../", 1, true) then
            ok, err = false, "Update archive contains an unsafe path."; break
        end
        if p ~= "mailpush.koplugin" and p:sub(1,20) ~= "mailpush.koplugin/" then
            ok, err = false, "Update archive has an unexpected layout."; break
        end
        if entry.mode ~= "file" and entry.mode ~= "directory" then
            ok, err = false, "Update archive contains an unsupported file type."; break
        end
    end
    arc:close()
    if count == 0 then return false, "Update archive is empty." end
    return ok, err
end

function MailPush:install_update(info)
    local downloader_path = plugin_dir .. "/updater_download.lua"
    local loader, load_err = loadfile(downloader_path)
    if not loader then self:show("Updater downloader is missing or invalid: " .. tostring(load_err), true); return end
    local ok_mod, downloader = pcall(loader)
    if not ok_mod or type(downloader) ~= "table" or type(downloader.download) ~= "function" then
        self:show("Updater downloader could not be loaded.", true); return
    end

    local update_zip = settings_dir .. "/mailpush-update.zip"
    local staging = settings_dir .. "/mailpush-update-stage"
    local backup = plugin_dir .. ".previous"
    os.remove(update_zip); rm_rf(staging)
    self:show("Downloading MailPush " .. tostring(info.latest_version) .. "…")
    local downloaded, download_err = downloader.download(info.asset_url, update_zip, UPDATE_MAX_BYTES)
    if not downloaded then self:show(download_err or "Update download failed.", true); return end

    local safe, safe_err = self:validate_update_archive(update_zip)
    if not safe then os.remove(update_zip); self:show(safe_err or "Update archive validation failed.", true); return end

    rm_rf(staging)
    os.execute("mkdir -p " .. shell_quote(staging))
    local unpacked, unpack_err = Device:unpackArchive(update_zip, staging, false)
    if not unpacked then rm_rf(staging); os.remove(update_zip); self:show("Cannot unpack update: " .. tostring(unpack_err), true); return end

    local staged_plugin = staging .. "/mailpush.koplugin"
    local required = { "_meta.lua", "main.lua", "config.default.json", "cacert.pem", "bin/mailpush", "updater_download.lua" }
    for _, rel in ipairs(required) do
        if not read_all(staged_plugin .. "/" .. rel) then
            rm_rf(staging); os.remove(update_zip); self:show("Update archive is missing required file: " .. rel, true); return
        end
    end
    os.execute("chmod 755 " .. shell_quote(staged_plugin .. "/bin/mailpush"))

    rm_rf(backup)
    local moved_old, move_old_err = os.rename(plugin_dir, backup)
    if not moved_old then rm_rf(staging); os.remove(update_zip); self:show("Cannot create plugin backup: " .. tostring(move_old_err), true); return end
    local moved_new, move_new_err = os.rename(staged_plugin, plugin_dir)
    if not moved_new then
        os.rename(backup, plugin_dir)
        rm_rf(staging); os.remove(update_zip)
        self:show("Cannot install update; previous version was restored: " .. tostring(move_new_err), true); return
    end

    rm_rf(staging); os.remove(update_zip); os.remove(update_state_path)
    UIManager:show(ConfirmBox:new{
        text = "MailPush " .. tostring(info.latest_version) .. " was installed successfully.\n\nRestart KOReader now to load the new version?",
        ok_text = "Restart",
        cancel_text = "Later",
        ok_callback = function() UIManager:restartKOReader() end,
    })
end
function MailPush:check_update(force)
    if not force and self:update_snoozed() then return end
    local info = self:backend("check-update", nil, true)
    if not info then return end
    if info.available then
        UIManager:show(ConfirmBox:new{
            text = "A new MailPush version is available.\n\nInstalled: " .. tostring(info.current_version) .. "\nAvailable: " .. tostring(info.latest_version) .. "\n\nDownload and install it now?",
            ok_text = "Update",
            cancel_text = "Not now",
            ok_callback = function() self:install_update(info) end,
            cancel_callback = function() self:snooze_updates(); self:show("Update reminder postponed for 30 days.") end,
        })
    elseif force then self:show("MailPush is up to date (" .. tostring(info.current_version or "unknown") .. ").") end
end

function MailPush:fetch(quiet)
    local cfg, err = load_config(); if not cfg then self:show(err,true); return end
    local result = self:backend("fetch",nil,quiet); self:show_result(result)
    UIManager:nextTick(function() self:check_update(false) end)
end
function MailPush:test_connection() self:show_result(self:backend("test",nil,false)) end
function MailPush:edit_field(key,title,input_type)
    local cfg,err=load_config(); if not cfg then self:show(err,true); return end
    local dialog
    dialog=InputDialog:new{title=title,input=tostring(cfg[key] or ""),input_type=input_type or "text",buttons={{{text="Cancel",id="close",callback=function() UIManager:close(dialog) end},{text="Save",is_enter_default=true,callback=function()
        local value=dialog:getInputText(); if input_type=="number" then value=tonumber(value); if not value then self:show("Please enter a valid number.",true); return end end
        cfg[key]=value; local ok,serr=save_config(cfg); if not ok then self:show(serr,true); return end; UIManager:close(dialog)
    end}}}}
    UIManager:show(dialog); dialog:onShowKeyboard()
end
function MailPush:toggle(key)
    local cfg,err=load_config(); if not cfg then self:show(err,true); return end
    cfg[key]=not cfg[key]; local ok,serr=save_config(cfg); if not ok then self:show(serr,true) end
end
function MailPush:is_enabled(key) local cfg=load_config(); return cfg and cfg[key]==true or false end
function MailPush:reset_history()
    UIManager:show(ConfirmBox:new{text="Forget the processed-message history?\n\nPreviously downloaded messages may be downloaded again.",ok_text="Reset",ok_callback=function() os.remove(state_path); self:show("Processed-message history was reset.") end})
end
function MailPush:show_paths() self:show("Configuration:\n"..config_path.."\n\nLast backend result:\n"..last_result_path) end
function MailPush:init()
    ensure_settings()
    -- A successful update intentionally leaves the old plugin directory as a
    -- rollback copy until the new plugin has loaded at least once.
    rm_rf(plugin_dir .. ".previous")
    self.ui.menu:registerToMainMenu(self)
    local cfg=load_config(); if cfg and cfg.fetch_on_start then UIManager:nextTick(function() self:fetch(true) end) end
end
function MailPush:addToMainMenu(menu_items)
    menu_items.mailpush={text="MailPush",sorting_hint="network",sub_item_table={
        {text="Fetch mail now",callback=function() self:fetch(false) end,separator=true},
        {text="Check for updates",callback=function() self:check_update(true) end},
        {text="Test connection",callback=function() self:test_connection() end},
        {text="IMAP host",keep_menu_open=true,callback=function() self:edit_field("host","IMAP host") end},
        {text="IMAP port",keep_menu_open=true,callback=function() self:edit_field("port","IMAP port","number") end},
        {text="Username",keep_menu_open=true,callback=function() self:edit_field("user","Username") end},
        {text="Password",keep_menu_open=true,callback=function() self:edit_field("password","Password","password") end},
        {text="Mailbox",keep_menu_open=true,callback=function() self:edit_field("mailbox","Mailbox") end},
        {text="Download directory",keep_menu_open=true,callback=function() self:edit_field("download_dir","Download directory") end},
        {text="Allowed root directory",keep_menu_open=true,callback=function() self:edit_field("root","Allowed root directory") end},
        {text="Custom CA file",keep_menu_open=true,callback=function() self:edit_field("ca_file","Custom CA file (optional)") end,separator=true},
        {text="Fetch unread messages only",checked_func=function() return self:is_enabled("fetch_unread_only") end,callback=function() self:toggle("fetch_unread_only") end},
        {text="Mark successfully processed mail as read",checked_func=function() return self:is_enabled("mark_seen") end,callback=function() self:toggle("mark_seen") end},
        {text="Fetch once when KOReader starts",checked_func=function() return self:is_enabled("fetch_on_start") end,callback=function() self:toggle("fetch_on_start") end},
        {text="Automatically unpack archives",checked_func=function() return self:is_enabled("auto_unpack") end,callback=function() self:toggle("auto_unpack") end,separator=true},
        {text="Maximum message age (days)",keep_menu_open=true,callback=function() self:edit_field("max_age_days","Maximum message age (days)","number") end},
        {text="Maximum messages per check",keep_menu_open=true,callback=function() self:edit_field("max_messages","Maximum messages per check","number") end},
        {text="Maximum file size (bytes)",keep_menu_open=true,callback=function() self:edit_field("max_file_bytes","Maximum file size (bytes)","number") end},
        {text="Maximum message size (bytes)",keep_menu_open=true,callback=function() self:edit_field("max_message_bytes","Maximum message size (bytes)","number") end},
        {text="Maximum unpacked archive size (bytes)",keep_menu_open=true,callback=function() self:edit_field("max_archive_bytes","Maximum unpacked archive size (bytes)","number") end},
        {text="Maximum files in archive",keep_menu_open=true,callback=function() self:edit_field("max_archive_files","Maximum files in archive","number") end,separator=true},
        {text="Reset processed-message history",callback=function() self:reset_history() end},
        {text="Show configuration paths",callback=function() self:show_paths() end},
    }}
end
return MailPush
