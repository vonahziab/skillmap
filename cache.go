package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// CacheData — содержимое cache_<город>.json (см. ТЗ, шаг 3 и ADR-0004).
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

// NewCache создаёт пустой кэш перед началом сбора для города.
func NewCache(city, cityID string) *CacheData {
	now := time.Now()
	return &CacheData{
		City:      city,
		CityID:    cityID,
		CreatedAt: now,
		UpdatedAt: now,
		Data:      make(map[string]map[string]int),
	}
}

// LoadCache читает кэш для города. Возвращает (nil, nil), если файла нет,
// он повреждён или относится к другому area ID — одному названию города
// может соответствовать несколько разных ID (см. шаг 2 диалога), так что
// совпадения имени файла недостаточно для доверия содержимому.
func LoadCache(city, cityID string) (*CacheData, error) {
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
	if cache.CityID != cityID {
		return nil, nil
	}
	if cache.Data == nil {
		cache.Data = make(map[string]map[string]int)
	}
	return &cache, nil
}

// MarkCompleted фиксирует агрегированные навыки профессии и немедленно
// сохраняет кэш на диск — вызывается сразу после завершения каждой
// профессии, чтобы прерванный сбор не терял прогресс (ADR-0004).
func (c *CacheData) MarkCompleted(profession string, skills map[string]int) error {
	c.Completed = append(c.Completed, profession)
	c.Data[profession] = skills
	c.UpdatedAt = time.Now()
	return c.Save()
}

// Save пишет кэш во временный файл и переименовывает его в целевой —
// так прерывание процесса посреди записи не оставляет cache_<город>.json
// в повреждённом состоянии.
func (c *CacheData) Save() error {
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	name := cacheFileName(c.City)
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, name)
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
