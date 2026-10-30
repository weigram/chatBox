package main

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// How a chat's pages reach the network, as Chat.Proxy holds it: empty for
// the system's proxy, proxyDirect to connect directly whatever the system
// says, or a proxy URL of its own.
const proxyDirect = "direct"

// proxyModes are the choices of the editor's proxy field, in order.
var proxyModes = []string{"跟随系统", "直连", "自定义"}

// proxyModeOf names the mode a chat's proxy setting is in.
func proxyModeOf(spec string) string {
	switch strings.TrimSpace(spec) {
	case "":
		return proxyModes[0]
	case proxyDirect:
		return proxyModes[1]
	default:
		return proxyModes[2]
	}
}

// proxySpecOf makes the setting the mode and the URL mean.
func proxySpecOf(mode, address string) string {
	switch mode {
	case proxyModes[1]:
		return proxyDirect
	case proxyModes[2]:
		return strings.TrimSpace(address)
	default:
		return ""
	}
}

// validateProxy checks the proxy the editor holds: an HTTP or HTTPS proxy,
// or a SOCKS5 one with its port.
func validateProxy(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == proxyDirect {
		return nil
	}
	u, err := url.Parse(spec)
	if err != nil {
		return errors.New("代理地址无效：" + err.Error())
	}
	socks := false
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	case "socks", "socks5", "socks5h":
		socks = true
	default:
		return errors.New("代理只支持 http、https、socks5，例如 http://127.0.0.1:7890")
	}
	if u.Hostname() == "" {
		return errors.New("代理地址缺少主机名")
	}
	if u.Port() == "" {
		if socks {
			return errors.New("socks5 代理要写端口，例如 socks5://127.0.0.1:1080")
		}
		return nil
	}
	if port, err := strconv.Atoi(u.Port()); err != nil || port < 1 || port > 65535 {
		return errors.New("代理端口无效")
	}
	return nil
}

// parseBypass reads the hosts the editor's text holds, one per line, with
// commas and spaces between them too.
func parseBypass(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t' || r == ';'
	})
	var out []string
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// describeProxyFully names a chat's proxy at length, for a tooltip.
func describeProxyFully(spec string) string {
	switch strings.TrimSpace(spec) {
	case "":
		return "跟随系统代理"
	case proxyDirect:
		return "直连，不用系统代理"
	default:
		return "代理 " + strings.TrimSpace(spec)
	}
}
