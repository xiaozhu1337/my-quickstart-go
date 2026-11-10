package mcpcheck

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"my-quickstart-go/internal/library"
)

func httpTransport(config library.Server) (mcp.Transport, func() error, error) {
	u, err := url.Parse(config.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, nil, errors.New("无效的 HTTP 地址")
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = networkProxy
	base.DisableKeepAlives = true
	client := &http.Client{Transport: &headerTransport{base, config.Headers}, Timeout: Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &mcp.StreamableClientTransport{Endpoint: config.URL, HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true, MaxEventSize: 4 << 20}, func() error { base.CloseIdleConnections(); return nil }, nil
}

func networkProxy(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, nil
		}
		return url.Parse("http://192.168.1.99:7890")
	}
	if strings.EqualFold(host, "localhost") || !strings.Contains(host, ".") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return nil, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(req.Context(), host)
	if err == nil {
		for _, addr := range addresses {
			if addr.IP.IsPrivate() || addr.IP.IsLoopback() || addr.IP.IsLinkLocalUnicast() {
				return nil, nil
			}
		}
	}
	return url.Parse("http://192.168.1.99:7890")
}

type headerTransport struct {
	base    *http.Transport
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	for key, value := range t.headers {
		r.Header.Set(key, value)
	}
	resp, err := t.base.RoundTrip(r)
	if err == nil {
		resp.Body = &limitedBody{ReadCloser: resp.Body, remaining: 4 << 20}
	}
	return resp, err
}

type limitedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, errors.New("MCP 响应超过 4 MiB")
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}
