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

// invalidate drops a still-fresh success entry to stale (M8-o, the
// dial-failure feedback loop): the caller dialed every address of the
// cached set and none connected, so the set is suspect on this network
// and the next lookup should re-query the servers instead of serving
// the same set until TTL expiry. The stale window is kept — if the
// servers then fail, the old answer is still served (R5). Negative
// entries, already-expired entries and misses are untouched.
func (c *cache) invalidate(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[host]
	if !ok || e.negative != "" {
		return
	}
	now := c.now()
	if now.Before(e.expiresAt) {
		e.expiresAt = now // fresh -> stale; staleUntil unchanged
	}
}

// clearNegative drops a NEGATIVE entry for host (M8-w, the M8-r lesson
// extended to the periodic checks). Success entries are untouched — a
// good cached answer is still good, and demoting it would only tax the
// tunnel path with a re-query. The health checks call this before
// probing: their question is "what do the servers say NOW", and the 15 s
// negative memory would answer "what failed moments ago" without a
// single server being asked — the debug.14 report's degraded state
// (dns fail 16, DNS_UNREACHABLE) was exactly that, the probe name being
// the very name app flows negative-cache during a dirty-window DoH blip.
func (c *cache) clearNegative(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[host]; ok && e.negative != "" {
		delete(c.m, host)
		c.removeOrder(host)
	}
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
