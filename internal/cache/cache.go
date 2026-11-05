package cache

import (
    "context"
    "time"
)

type Cache interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}

// Специальная ошибка для случая, когда ключ не найден
type ErrNotFound struct {
    Key string
}

func (e *ErrNotFound) Error() string {
    return "cache key not found: " + e.Key
}