package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// CacheData — содержимое cache_<город>.json (см. ТЗ, шаг 3).
// Запись кэша после каждой профессии — предмет milestone 4;
// здесь определена модель и чтение, нужные диалогу запуска (milestone 3).
type CacheData struct {
	City      string                    `json:"city"`
	CityID    string                    `json:"city_id"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Completed []string                  `json:"completed"`
	Data      map[string]map[string]int `json:"data"`
}

// cacheFileName — имя файла кэша для города.
func cacheFileName(city string) string {
	return fmt.Sprintf("cache_%s.json", city)
}

// LoadCache читает кэш для города. Если файла нет — возвращает (nil, nil).
func LoadCache(city string) (*CacheData, error) {
	body, err := os.ReadFile(cacheFileName(city))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cache CacheData
	if err := json.Unmarshal(body, &cache); err != nil {
		return nil, fmt.Errorf("decode %s: %w", cacheFileName(city), err)
	}
	return &cache, nil
}

// NextProfession возвращает первую профессию из списка, отсутствующую
// в Completed — с неё продолжается прерванный сбор.
func (c *CacheData) NextProfession(all []string) string {
	done := make(map[string]bool, len(c.Completed))
	for _, p := range c.Completed {
		done[p] = true
	}
	for _, p := range all {
		if !done[p] {
			return p
		}
	}
	return ""
}
