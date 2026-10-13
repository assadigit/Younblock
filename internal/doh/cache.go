package doh

import (
	"net"
	"sync"
	"time"

	"github.com/assadigit/Younblock/internal/config"
)

// entry is one cached name.
type entry struct {
	ips        []net.IP
	expiresAt  time.Time
	staleUntil time.Time
	// negative is the error code of a failed lookup ("" for success entries).
	negative string
}

// cache is a bounded TTL cache with serve-stale and negative caching
// (file 01, section 6; file 03, R5). now is injectable for tests.
type cache struct {
	mu    sync.Mutex
	m     map[string]*entry
	order []string // FIFO eviction order
	max   int
	now   func() time.Time
}

func newCache(max int, now func() time.Time) *cache {
	if max < 1 {
		max = 1
	}
	if now == nil {
		now = time.Now
	}
	return &cache{m: make(map[string]*entry), max: max, now: now}
}

// lookupResult says what the cache knows about a name right now.
type lookupResult struct {
	// state is one of fresh, stale, negative, miss.
	state string
	ips   []net.IP
	// code is the negative error code when state == negative.
	code string
}

const (
	stateFresh    = "fresh"
	stateStale    = "stale"
	stateNegative = "negative"
	stateMiss     = "miss"
)

func (c *cache) lookup(host string) lookupResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[host]
	if !ok {
		return lookupResult{state: stateMiss}
	}
	now := c.now()
	if e.negative != "" {
		if now.Before(e.expiresAt) {
			return lookupResult{state: stateNegative, code: e.negative}
		}
		delete(c.m, host)
		c.removeOrder(host)
		return lookupResult{state: stateMiss}
	}
	if now.Before(e.expiresAt) {
		return lookupResult{state: stateFresh, ips: e.ips}
	}
	if now.Before(e.staleUntil) {
		return lookupResult{state: stateStale, ips: e.ips}
	}
	delete(c.m, host)
	c.removeOrder(host)
	return lookupResult{state: stateMiss}
}

func (c *cache) putSuccess(host string, ips []net.IP, ttl time.Duration) {
	if ttl < config.DNSCacheMinTTL {
		ttl = config.DNSCacheMinTTL
	}
	if ttl > config.DNSCacheMaxTTL {
		ttl = config.DNSCacheMaxTTL
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictLocked()
	e := &entry{
		ips:        append([]net.IP(nil), ips...),
		expiresAt:  now.Add(ttl),
		staleUntil: now.Add(ttl + config.DNSCacheServeStale),
	}
	c.m[host] = e
	c.pushOrder(host)
}

func (c *cache) putNegative(host string, code string) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictLocked()
	// Never overwrite a still-usable success entry with a negative one:
	// the stale answer is more useful than the failure.
	if e, ok := c.m[host]; ok && e.negative == "" && now.Before(e.staleUntil) {
		return
	}
	c.m[host] = &entry{negative: code, expiresAt: now.Add(config.DNSCacheNegativeTTL)}
	c.pushOrder(host)
}

func (c *cache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]*entry)
	c.order = nil
}

func (c *cache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

func (c *cache) pushOrder(host string) {
	for _, h := range c.order {
		if h == host {
			return
		}
	}
	c.order = append(c.order, host)
}

func (c *cache) removeOrder(host string) {
	for i, h := range c.order {
		if h == host {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *cache) evictLocked() {
	for len(c.m) >= c.max && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.m, oldest)
	}
}
