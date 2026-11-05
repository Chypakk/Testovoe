package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testovoe/internal/models"
	"testovoe/internal/repository"
	"time"

	"github.com/redis/go-redis/v9"
)

type taskCacheRedis struct {
	client *redis.Client
	ttl    time.Duration
}

func NewTaskCacheRedis(client *redis.Client) TaskCache {
	return &taskCacheRedis{
		client: client,
		ttl:    5 * time.Minute,
	}
}

func (c *taskCacheRedis) cacheKey(filter repository.TaskFilter) string {
	raw := fmt.Sprintf("t:%d|s:%s|a:%d|l:%d|o:%d",
		filter.TeamID, filter.Status, filter.AssigneeID, filter.Limit, filter.Offset)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("tasks:team:%d:hash:%s", filter.TeamID, hex.EncodeToString(hash[:]))
}

func (c *taskCacheRedis) GetTasks(ctx context.Context, filter repository.TaskFilter) ([]models.Task, bool, error) {
	key := c.cacheKey(filter)
	
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil // Cache miss
		}
		return nil, false, err
	}
	
	var tasks []models.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, false, err
	}
	
	return tasks, true, nil
}

func (c *taskCacheRedis) SetTasks(ctx context.Context, filter repository.TaskFilter, tasks []models.Task) error {
	key := c.cacheKey(filter)
	
	data, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	
	return c.client.Set(ctx, key, data, c.ttl).Err()
}

func (c *taskCacheRedis) InvalidateTeam(ctx context.Context, teamID uint) error {
	pattern := fmt.Sprintf("tasks:team:%d:*", teamID)
	
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	
	return nil
}