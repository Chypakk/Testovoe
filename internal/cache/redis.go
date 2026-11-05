package cache

import (
    "context"
    "time"
    
    "github.com/redis/go-redis/v9"
)

type redisCache struct {
    client *redis.Client
}

func NewRedisCache(client *redis.Client) Cache {
    return &redisCache{
        client: client,
    }
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return "", &ErrNotFound{Key: key}
    }
    if err != nil {
        return "", err
    }
    
    return val, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
    return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
    return c.client.Del(ctx, key).Err()
}