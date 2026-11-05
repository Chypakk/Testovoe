package cache

import (
	"context"
	"log"
	"testovoe/internal/configs"

	"github.com/redis/go-redis/v9"
)

// NewTaskCache создает TaskCache: пытается Redis, если не получается — fallback на InMemory
func NewTaskCache(cfg configs.RedisConfig) TaskCache {
	// Пытаемся подключиться к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	
	// Проверяем соединение
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed, using in-memory cache: %v", err)
		_ = rdb.Close()
		return NewTaskCacheInMemory()
	}
	
	log.Println("Using Redis cache")
	return NewTaskCacheRedis(rdb)
}