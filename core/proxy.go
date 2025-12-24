package core

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/proxy"
)

// GetProxyDialer returns a dialer that respects environment variables (ALL_PROXY, etc.)
func GetProxyDialer() (proxy.Dialer, error) {
	// proxy.FromEnvironment respects ALL_PROXY and NO_PROXY
	return proxy.FromEnvironment(), nil
}

// GetProxyHTTPClient returns an HTTP client that respects proxy environment variables including SOCKS5
func GetProxyHTTPClient(timeout time.Duration) *http.Client {
	// Create a dialer from environment
	dialer := proxy.FromEnvironment()

	// Create a transport that uses the dialer
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// proxy.Dialer interface has Dial, but ContextDialer has DialContext
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx, network, addr)
			}
			return dialer.Dial(network, addr)
		},
		// Inherit default transport settings
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// However, net/http DefaultTransport also handles HTTP_PROXY/HTTPS_PROXY via ProxyFromEnvironment.
	// If we use custom DialContext, we might bypass the standard HTTP proxy logic unless we set Proxy field.
	// But if ALL_PROXY (SOCKS) is set, we want to use that.
	// If HTTP_PROXY is set, we want to use that.

	// Strategy:
	// 1. If ALL_PROXY is set, use the custom dialer (which handles SOCKS).
	// 2. If ALL_PROXY is NOT set, use standard http.ProxyFromEnvironment (which handles HTTP_PROXY).

	if os.Getenv("ALL_PROXY") != "" {
		return &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}
	}

	// Fallback to standard client which handles HTTP_PROXY automatically
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
}
