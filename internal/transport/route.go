package transport

import "fmt"

type RouteSource string

const (
	RouteSourceDirect  RouteSource = "direct"
	RouteSourceGlobal  RouteSource = "global"
	RouteSourceAccount RouteSource = "account"
)

type RouteInfo struct {
	Source    RouteSource
	Scheme    ProxyScheme
	ProxyURL  string
	RemoteDNS bool
}

type route struct {
	info  RouteInfo
	proxy *ProxySpec
}

func resolveRoute(accountProxyURL, globalProxyURL string) (route, error) {
	if accountProxyURL != "" {
		spec, err := ParseProxy(accountProxyURL)
		if err != nil {
			return route{}, fmt.Errorf("account proxy: %w", err)
		}
		return route{
			info: RouteInfo{Source: RouteSourceAccount, Scheme: spec.Scheme(), ProxyURL: spec.Redacted(), RemoteDNS: spec.RemoteDNS()},
			proxy: &spec,
		}, nil
	}
	if globalProxyURL != "" {
		spec, err := ParseProxy(globalProxyURL)
		if err != nil {
			return route{}, fmt.Errorf("global proxy: %w", err)
		}
		return route{
			info: RouteInfo{Source: RouteSourceGlobal, Scheme: spec.Scheme(), ProxyURL: spec.Redacted(), RemoteDNS: spec.RemoteDNS()},
			proxy: &spec,
		}, nil
	}
	return route{info: RouteInfo{Source: RouteSourceDirect}}, nil
}

// ResolveRoute exposes only redacted route metadata.
func ResolveRoute(accountProxyURL, globalProxyURL string) (RouteInfo, error) {
	route, err := resolveRoute(accountProxyURL, globalProxyURL)
	return route.info, err
}
