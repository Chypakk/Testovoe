package cache

import (
    "context"
    "sync"
    "time"
)

type inMemoryCache struct {
    data map[string]cacheItem
    mu   sync.RWMutex
}

type cacheItem struct {
    value     string
    expiresAt time.Time
}

func NewInMemoryCache() Cache {
    return &inMemoryCache{
        data: make(map[string]cacheItem),
    }
}

func (c *inMemoryCache) Get(ctx context.Context, key string) (string, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, exists := c.data[key]
    if !exists || time.Now().After(item.expiresAt) {
        return "", &ErrNotFound{Key: key}
    }
    
    return item.value, nil
}

func (c *inMemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.data[key] = cacheItem{
        value:     value,
        expiresAt: time.Now().Add(ttl),
    }
    
    return nil
}

func (c *inMemoryCache) Delete(ctx context.Context, key string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    delete(c.data, key)
    return nil
}