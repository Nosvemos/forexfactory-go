package forexfactory

import (
	"net/http"
	"time"
)

// Option is a function type used to apply configurations to a Client.
type Option func(*Client)

// WithHTTPClient returns an Option that configures the client to use a custom *http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithUserAgent returns an Option that configures the Client to send a custom User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithProxy returns an Option that configures the client to use a specific proxy server.
// The proxyURL parameter should be in the format: http://[user:pass@]host:port or socks5://[user:pass@]host:port.
func WithProxy(proxyURL string) Option {
	return func(c *Client) {
		if proxyURL != "" {
			c.proxyURL = proxyURL
		}
	}
}

// WithRateLimit returns an Option that configures the rate limiter to cap requests.
// requestsPerSecond determines the maximum number of outgoing requests allowed per second.
func WithRateLimit(requestsPerSecond int) Option {
	return func(c *Client) {
		if requestsPerSecond > 0 {
			c.rateLimit = requestsPerSecond
		}
	}
}

// WithTimeLocation returns an Option that shifts parsed event datetimes
// into the specified timezone. If not set, UTC or Forex Factory server default is used.
func WithTimeLocation(loc *time.Location) Option {
	return func(c *Client) {
		if loc != nil {
			c.timeLoc = loc
		}
	}
}

// WithHeader returns an Option that sets a custom HTTP header for all requests.
// This is extremely useful for passing custom Cookies (e.g. cf_clearance) to bypass Cloudflare.
func WithHeader(key, value string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[key] = value
	}
}

// WithConcurrency returns an Option that configures the concurrent downloading workers
// when fetching a range of dates.
func WithConcurrency(workers int) Option {
	return func(c *Client) {
		if workers > 0 {
			c.concurrency = workers
		}
	}
}

// WithProgressCallback returns an Option that sets a custom callback function
// to receive progress updates during range downloads (ticks current week index / total weeks).
func WithProgressCallback(callback func(current, total int)) Option {
	return func(c *Client) {
		c.progressCallback = callback
	}
}

// WithImpacts returns an Option that configures the Client to only parse and return
// events matching the specified impact levels (e.g. ImpactHigh, ImpactMedium).
func WithImpacts(impacts ...Impact) Option {
	return func(c *Client) {
		if len(impacts) > 0 {
			c.impactFilter = make(map[Impact]bool)
			for _, imp := range impacts {
				c.impactFilter[imp] = true
			}
		}
	}
}

