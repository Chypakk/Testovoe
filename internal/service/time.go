package service

import "time"

// nowUTC — единая точка генерации времени закрытия задач (упрощает тестирование).
func nowUTC() time.Time { return time.Now().UTC() }
