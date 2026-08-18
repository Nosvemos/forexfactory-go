package forexcalendar

import (
	"net/http"
	"strings"
	"time"
)

// Option is a function type used to configure a Client.
type Option func(*Client)

// WithHTTPClient configures the client to use a custom *http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithProxy configures a proxy URL (http:// or socks5://).
func WithProxy(proxyURL string) Option {
	return func(c *Client) {
		if proxyURL != "" {
			c.proxyURL = proxyURL
		}
	}
}

// WithRateLimit sets the maximum requests allowed per second.
func WithRateLimit(requestsPerSecond int) Option {
	return func(c *Client) {
		if requestsPerSecond > 0 {
			c.rateLimit = requestsPerSecond
		}
	}
}

// WithConcurrency sets the number of parallel download workers.
func WithConcurrency(workers int) Option {
	return func(c *Client) {
		if workers > 0 {
			c.concurrency = workers
		}
	}
}

// WithTimeLocation shifts event datetimes into the specified timezone.
func WithTimeLocation(loc *time.Location) Option {
	return func(c *Client) {
		if loc != nil {
			c.timeLoc = loc
		}
	}
}

// WithMaxRetries sets the maximum retry attempts for failed HTTP calls.
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		if retries > 0 {
			c.maxRetries = retries
		}
	}
}

// WithImpactFilter filters events to only include the specified impact levels.
func WithImpactFilter(impacts ...Impact) Option {
	return func(c *Client) {
		if len(impacts) > 0 {
			if c.impactFilter == nil {
				c.impactFilter = make(map[Impact]bool)
			}
			for _, imp := range impacts {
				c.impactFilter[imp] = true
			}
		}
	}
}

// WithCurrencyFilter filters events to only include specific currencies (e.g. "USD", "EUR").
func WithCurrencyFilter(currencies ...string) Option {
	return func(c *Client) {
		if len(currencies) > 0 {
			if c.currencyFilter == nil {
				c.currencyFilter = make(map[string]bool)
			}
			for _, curr := range currencies {
				c.currencyFilter[strings.ToUpper(strings.TrimSpace(curr))] = true
			}
		}
	}
}

// WithCountryFilter filters events to only include specific countries (e.g. "US", "EU", "GB").
func WithCountryFilter(countries ...string) Option {
	return func(c *Client) {
		if len(countries) > 0 {
			if c.countryFilter == nil {
				c.countryFilter = make(map[string]bool)
			}
			for _, cnt := range countries {
				c.countryFilter[strings.ToUpper(strings.TrimSpace(cnt))] = true
			}
		}
	}
}

// WithProgressCallback sets a callback invoked after each batch of downloads completes.
func WithProgressCallback(cb func(current, total int)) Option {
	return func(c *Client) {
		c.progressCallback = cb
	}
}
