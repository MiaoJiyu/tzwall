package cache

import (
	"sync"
	"time"
)

type memItem struct {
	val    string
	expire time.Time
}

type memCache struct {
	mu   sync.RWMutex
	kvs  map[string]memItem
	sets map[string]map[string]time.Time
	stop chan struct{}
}

func newMemory() *memCache {
	m := &memCache{
		kvs:  make(map[string]memItem),
		sets: make(map[string]map[string]time.Time),
		stop: make(chan struct{}),
	}
	go m.sweep()
	return m
}

func (m *memCache) sweep() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			now := time.Now()
			m.mu.Lock()
			for k, v := range m.kvs {
				if !v.expire.IsZero() && now.After(v.expire) {
					delete(m.kvs, k)
				}
			}
			for k, set := range m.sets {
				for member, exp := range set {
					if !exp.IsZero() && now.After(exp) {
						delete(set, member)
					}
				}
				if len(set) == 0 {
					delete(m.sets, k)
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *memCache) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if it, ok := m.kvs[key]; ok {
		if it.expire.IsZero() || time.Now().Before(it.expire) {
			return it.val, true
		}
	}
	return "", false
}

func (m *memCache) Set(key string, val string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.kvs[key] = memItem{val: val, expire: exp}
}

func (m *memCache) Incr(key string, ttl time.Duration) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	it, ok := m.kvs[key]
	if !ok || (!it.expire.IsZero() && now.After(it.expire)) {
		exp := time.Time{}
		if ttl > 0 {
			exp = now.Add(ttl)
		}
		m.kvs[key] = memItem{val: "1", expire: exp}
		return 1
	}
	n := 0
	for i := 0; i < len(it.val); i++ {
		n = n*10 + int(it.val[i]-'0')
	}
	n++
	m.kvs[key] = memItem{val: itoa(n), expire: it.expire}
	return int64(n)
}

func (m *memCache) Del(key string) {
	m.mu.Lock()
	delete(m.kvs, key)
	m.mu.Unlock()
}

func (m *memCache) SAdd(key string, member string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	set, ok := m.sets[key]
	if !ok {
		set = make(map[string]time.Time)
		m.sets[key] = set
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = now.Add(ttl)
	}
	set[member] = exp
}

func (m *memCache) SIsMember(key string, member string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if set, ok := m.sets[key]; ok {
		if exp, exists := set[member]; exists {
			if exp.IsZero() || time.Now().Before(exp) {
				return true
			}
		}
	}
	return false
}

func (m *memCache) Close() error {
	close(m.stop)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
