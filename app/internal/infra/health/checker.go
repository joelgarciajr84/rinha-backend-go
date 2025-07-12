package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Health struct {
	Failing bool `json:"failing"`
}

type Checker struct {
	cache      map[string]Health
	lastCheck  map[string]time.Time
	checkMutex sync.Mutex
}

func NewChecker() *Checker {
	return &Checker{
		cache:     make(map[string]Health),
		lastCheck: make(map[string]time.Time),
	}
}

func (c *Checker) IsHealthy(url string) bool {
	c.checkMutex.Lock()
	defer c.checkMutex.Unlock()

	now := time.Now()
	last, ok := c.lastCheck[url]

	if ok && now.Sub(last) < 5*time.Second {
		return !c.cache[url].Failing
	}

	resp, err := http.Get(url + "/payments/service-health")
	if err != nil || resp.StatusCode == 429 || resp.StatusCode >= 500 {
		c.cache[url] = Health{Failing: true}
	} else {
		var h Health
		if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
			c.cache[url] = Health{Failing: true}
		} else {
			c.cache[url] = h
		}
	}
	c.lastCheck[url] = now

	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	return !c.cache[url].Failing
}
