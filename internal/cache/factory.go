package cache

import (
	"context"

	"github.com/redis/go-redis/v9"

	"testovoe/internal/configs"
	"testovoe/internal/logger"
)

// NewTaskCache создает TaskCache: пытается Redis, если не получается — fallback на InMemory
func NewTaskCache(cfg configs.RedisConfig) TaskCache {
	logger.Info("Creating task cache",
		"redis_host", cfg.Host,
		"redis_port", cfg.Port,
		"redis_db", cfg.DB,
	)

	// Пытаемся подключиться к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	
	// Проверяем соединение
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("Redis connection failed, falling back to in-memory cache", err,
			"redis_host", cfg.Host,
			"redis_port", cfg.Port,
			"redis_db", cfg.DB,
		)
		_ = rdb.Close()
		return NewTaskCacheInMemory()
	}
	
	logger.Info("Using Redis cache",
		"redis_host", cfg.Host,
		"redis_port", cfg.Port,
		"redis_db", cfg.DB,
	)
	return NewTaskCacheRedis(rdb)
}