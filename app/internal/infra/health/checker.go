package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Health struct {
	Failing bool
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

	last, ok := c.lastCheck[url]
	if ok && time.Since(last) < 5*time.Second {
		return !c.cache[url].Failing
	}

	resp, err := http.Get(url + "/payments/service-health")
	if err != nil || resp.StatusCode != 200 {
		c.cache[url] = Health{Failing: true}
	} else {
		var h Health
		json.NewDecoder(resp.Body).Decode(&h)
		c.cache[url] = h
	}
	c.lastCheck[url] = time.Now()
	return !c.cache[url].Failing
}
