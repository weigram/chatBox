//go:build darwin

package darwin

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

// Proxies per session: a WKWebsiteDataStore takes an array of
// Network.framework proxy configurations (macOS 14 and later), which the
// requests of the pages using that store go through, whatever the system's
// own proxy says. Each session has a store of its own, so each session
// routes as its app asks.
//
// Network.framework is loaded, and its functions looked up, the first time
// an app sets a proxy, so that apps that never do pay nothing for it.

var (
	networkOnce sync.Once
	networkErr  error

	fnEndpointCreateHost  func(hostname, port string) uintptr
	fnProxyHTTPConnect    func(endpoint, tlsOptions uintptr) uintptr
	fnProxySOCKSv5        func(endpoint uintptr) uintptr
	fnProxySetCredentials func(config uintptr, username, password string)
	fnProxyAddExcluded    func(config uintptr, domain string)
)

// loadNetwork opens Network.framework and finds the functions of its proxy
// configurations, reporting why a system without them cannot use this.
func loadNetwork() error {
	networkOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/Network.framework/Network",
			purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			networkErr = fmt.Errorf("mygo: cannot load Network.framework: %w", err)
			return
		}
		fns := []struct {
			name string
			fn   any
		}{
			{"nw_endpoint_create_host", &fnEndpointCreateHost},
			{"nw_proxy_config_create_http_connect", &fnProxyHTTPConnect},
			{"nw_proxy_config_create_socksv5", &fnProxySOCKSv5},
			{"nw_proxy_config_set_username_and_password", &fnProxySetCredentials},
			{"nw_proxy_config_add_excluded_domain", &fnProxyAddExcluded},
		}
		for _, f := range fns {
			addr, err := purego.Dlsym(lib, f.name)
			if err != nil {
				networkErr = fmt.Errorf("mygo: Network.framework has no %s: %w", f.name, err)
				return
			}
			purego.RegisterFunc(f.fn, addr)
		}
	})
	return networkErr
}

// SetProxy decides how the requests of the store's pages reach the
// network, as Session.SetProxy asks: the system's proxy for an empty spec,
// a direct connection for "direct", and the proxy of any other spec.
func (s *dataStore) SetProxy(spec string, bypass []string) error {
	if s.closed {
		return errors.New("mygo: the session is closed")
	}
	if !sendBool(s.store, "respondsToSelector:", uintptr(sel("setProxyConfigurations:"))) {
		return errors.New("mygo: a proxy per session needs macOS 14 or later")
	}
	if err := loadNetwork(); err != nil {
		return err
	}

	switch strings.TrimSpace(spec) {
	case "":
		// No configurations at all: the store takes the system's proxy
		// again, as it did before any of this.
		withPool(func() {
			send(s.store, "setProxyConfigurations:", 0)
		})
		return nil
	case "direct":
		// An empty array connects directly: the system's own proxy does
		// not apply to this session's pages.
		withPool(func() {
			send(s.store, "setProxyConfigurations:", uintptr(send(class("NSArray"), "array")))
		})
		return nil
	}

	config, err := proxyConfig(spec, bypass)
	if err != nil {
		return err
	}
	withPool(func() {
		// The array, which the store copies, holds the configuration: it
		// keeps it past the release below.
		list := send(class("NSArray"), "arrayWithObject:", uintptr(config))
		send(s.store, "setProxyConfigurations:", uintptr(list))
	})
	release(config)
	return nil
}

// proxyConfig makes the Network.framework configuration of a proxy URL:
// an HTTP CONNECT proxy for http and https, a SOCKSv5 one for socks5.
func proxyConfig(spec string, bypass []string) (id, error) {
	u, err := url.Parse(spec)
	if err != nil {
		return 0, fmt.Errorf("mygo: proxy %q: %w", spec, err)
	}
	host, port := u.Hostname(), u.Port()
	if host == "" {
		return 0, fmt.Errorf("mygo: proxy %q has no host", spec)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		scheme = "http"
	}
	switch scheme {
	case "http", "https", "socks", "socks5", "socks5h":
	default:
		return 0, fmt.Errorf("mygo: proxy %q: %q is not one of http, https, socks5", spec, u.Scheme)
	}
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return 0, fmt.Errorf("mygo: proxy %q needs its port", spec)
		}
	}

	endpoint := fnEndpointCreateHost(host, port)
	if endpoint == 0 {
		return 0, fmt.Errorf("mygo: proxy %q: cannot make its endpoint", spec)
	}
	// The configuration takes the endpoint over.
	defer release(id(endpoint))

	var config uintptr
	if scheme == "http" || scheme == "https" {
		// proxy_tls_options stays nil: the connection to an HTTP proxy is
		// clear, as HTTP CONNECT is.
		config = fnProxyHTTPConnect(endpoint, 0)
	} else {
		config = fnProxySOCKSv5(endpoint)
	}
	if config == 0 {
		return 0, fmt.Errorf("mygo: proxy %q: cannot make its configuration", spec)
	}
	if u.User != nil {
		password, _ := u.User.Password()
		fnProxySetCredentials(config, u.User.Username(), password)
	}
	for _, domain := range bypass {
		if domain = strings.TrimSpace(domain); domain != "" {
			fnProxyAddExcluded(config, domain)
		}
	}
	return id(config), nil
}
