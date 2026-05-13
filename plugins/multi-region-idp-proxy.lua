--
-- multi-region-idp-proxy plugin
--
-- Migrates an nginx configuration-snippet that rewrites request cookies for
-- multi-region IDP/SP flows and optionally proxies to a region-specific
-- upstream from the region_url cookie.
--
-- Configuration:
--   proxy_scheme: "https"
--   proxy_port: 443
--   allowed_region_hosts: ["10.0.0.12", "idp.example.local"] -- optional
--

local core = require("apisix.core")
local ngx = ngx

local plugin_name = "multi-region-idp-proxy"

local schema = {
    type = "object",
    properties = {
        proxy_scheme = {
            type = "string",
            enum = {"http", "https"},
            default = "https",
        },
        proxy_port = {
            type = "integer",
            minimum = 1,
            maximum = 65535,
            default = 443,
        },
        allowed_region_hosts = {
            type = "array",
            items = {
                type = "string",
                minLength = 1,
            },
        },
    },
}

local _M = {
    version = 0.1,
    priority = 4100,
    name = plugin_name,
    schema = schema,
}

function _M.check_schema(conf)
    return core.schema.check(schema, conf)
end

local function cookie(name)
    return ngx.var["cookie_" .. name] or ""
end

local function set_request_header(name, value)
    if value and value ~= "" then
        ngx.req.set_header(name, value)
    end
end

local function join_cookie(parts)
    local out = {}
    for _, item in ipairs(parts) do
        if item.value and item.value ~= "" then
            out[#out + 1] = item.name .. "=" .. item.value
        end
    end
    return table.concat(out, ";")
end

local function normalize_host(value)
    if not value or value == "" then
        return nil
    end

    value = value:gsub("^https?://", "")
    value = value:gsub("/.*$", "")
    value = value:gsub("%s+", "")

    if value == "" then
        return nil
    end
    return value
end

local function host_allowed(conf, host)
    if not conf.allowed_region_hosts or #conf.allowed_region_hosts == 0 then
        return true
    end

    for _, allowed in ipairs(conf.allowed_region_hosts) do
        if host == allowed then
            return true
        end
    end
    return false
end

local function set_dynamic_upstream(conf, host)
    if not host_allowed(conf, host) then
        core.log.warn("multi-region-idp-proxy: blocked region host: ", host)
        return
    end

    local node = host
    if not host:find(":", 1, true) and conf.proxy_port then
        node = host .. ":" .. conf.proxy_port
    end

    ngx.ctx.upstream = {
        type = "roundrobin",
        scheme = conf.proxy_scheme or "https",
        pass_host = "node",
        nodes = {
            [node] = 1,
        },
    }
end

local function handle_from_idp(ctx)
    local sp_cookie = join_cookie({
        {name = "sessionid", value = cookie("sp_sessionid")},
        {name = "escookie", value = cookie("sp_escookie")},
        {name = "csrftoken", value = cookie("sp_csrftoken")},
        {name = "ems_dashboard_api_language", value = cookie("sp_ems_dashboard_api_language")},
    })

    set_request_header("Cookie", sp_cookie)
    set_request_header("X-Csrftoken", cookie("sp_csrftoken"))
    ctx.multi_region_clear_set_cookie = true
end

local function handle_region_url(conf)
    local host = normalize_host(cookie("region_url"))
    if not host then
        return
    end

    local sp_cookie = join_cookie({
        {name = "sessionid", value = cookie("sessionid")},
        {name = "escookie", value = cookie("escookie")},
        {name = "csrftoken", value = cookie("csrftoken")},
        {name = "sp_sessionid", value = cookie("sp_sessionid")},
        {name = "sp_escookie", value = cookie("sp_escookie")},
        {name = "sp_csrftoken", value = cookie("sp_csrftoken")},
    })

    set_request_header("Cookie", sp_cookie)
    set_dynamic_upstream(conf, host)
end

function _M.rewrite(conf, ctx)
    local raw_cookie = ngx.var.http_cookie or ""

    if raw_cookie:find("region_label=fromidp", 1, true) then
        handle_from_idp(ctx)
        return
    end

    if raw_cookie:find("region_url=", 1, true) then
        handle_region_url(conf)
    end
end

local function should_drop_set_cookie(value)
    if not value then
        return false
    end

    local lower = value:lower()
    return lower:find("^sessionid=") ~= nil or lower:find("^csrftoken=") ~= nil
end

function _M.header_filter(conf, ctx)
    if not ctx.multi_region_clear_set_cookie then
        return
    end

    local set_cookie = ngx.header["Set-Cookie"]
    if not set_cookie then
        return
    end

    if type(set_cookie) == "table" then
        local kept = {}
        for _, value in ipairs(set_cookie) do
            if not should_drop_set_cookie(value) then
                kept[#kept + 1] = value
            end
        end

        if #kept == 0 then
            ngx.header["Set-Cookie"] = nil
        else
            ngx.header["Set-Cookie"] = kept
        end
        return
    end

    if should_drop_set_cookie(set_cookie) then
        ngx.header["Set-Cookie"] = nil
    end
end

return _M
