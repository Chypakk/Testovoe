package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testovoe/internal/models"
	"testovoe/internal/repository"
	"time"
)

type cacheItem struct {
    value     string
    expiresAt time.Time
}

type taskCacheInMemory struct {
	data map[string]cacheItem
	mu   sync.RWMutex
	ttl  time.Duration
}

func NewTaskCacheInMemory() TaskCache {
	return &taskCacheInMemory{
		data: make(map[string]cacheItem),
		ttl:  5 * time.Minute,
	}
}

func (c *taskCacheInMemory) cacheKey(filter repository.TaskFilter) string {
	raw := fmt.Sprintf("t:%d|s:%s|a:%d|l:%d|o:%d",
		filter.TeamID, filter.Status, filter.AssigneeID, filter.Limit, filter.Offset)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("tasks:team:%d:hash:%s", filter.TeamID, hex.EncodeToString(hash[:]))
}

func (c *taskCacheInMemory) GetTasks(ctx context.Context, filter repository.TaskFilter) ([]models.Task, bool, error) {
	key := c.cacheKey(filter)

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.data[key]
	if !exists {
		return nil, false, nil
	}

	// Проверяем TTL
	if time.Now().After(item.expiresAt) {
		return nil, false, nil
	}

	var tasks []models.Task
	if err := json.Unmarshal([]byte(item.value), &tasks); err != nil {
		return nil, false, err
	}

	return tasks, true, nil
}

func (c *taskCacheInMemory) SetTasks(ctx context.Context, filter repository.TaskFilter, tasks []models.Task) error {
	key := c.cacheKey(filter)

	data, err := json.Marshal(tasks)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = cacheItem{
		value:     string(data),
		expiresAt: time.Now().Add(c.ttl),
	}

	return nil
}

func (c *taskCacheInMemory) InvalidateTeam(ctx context.Context, teamID uint) error {
	prefix := fmt.Sprintf("tasks:team:%d:", teamID)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Удаляем все ключи с префиксом команды
	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			delete(c.data, key)
		}
	}

	return nil
}
