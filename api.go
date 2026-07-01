package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	apiBaseURL = "https://api.hh.ru"
	userAgent  = "SkillMap/1.0 (github.com/vonahziab/skillmap)"

	requestTimeout = 10 * time.Second
	requestPause   = 400 * time.Millisecond

	maxRetries = 3
	retryPause = 2 * time.Second

	vacanciesPerPage = 100
)

// Client — HTTP-клиент к публичному API hh.ru.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: requestTimeout}}
}

// get выполняет GET-запрос с retry (maxRetries попыток, пауза retryPause
// между ними) и выдерживает паузу requestPause после каждого запроса,
// независимо от исхода, чтобы не превышать rate-limit анонимного API.
func (c *Client) get(path string, query url.Values) ([]byte, error) {
	reqURL := apiBaseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		body, err := c.doGet(reqURL)
		if err == nil {
			time.Sleep(requestPause)
			return body, nil
		}
		lastErr = err
		if attempt < maxRetries {
			time.Sleep(retryPause)
		}
	}
	time.Sleep(requestPause)
	return nil, fmt.Errorf("hh.ru api request failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *Client) doGet(reqURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// Area — результат поиска города через /suggests/areas.
type Area struct {
	ID   string
	Name string
}

type suggestAreaItem struct {
	ID    string            `json:"id"`
	Text  string            `json:"text"`
	Areas []suggestAreaItem `json:"areas"`
}

type suggestAreasResponse struct {
	Items []suggestAreaItem `json:"items"`
}

// SearchAreas ищет город по тексту. Возвращает 0, 1 или несколько
// совпадений — выбор варианта обработки остаётся за вызывающим кодом.
func (c *Client) SearchAreas(city string) ([]Area, error) {
	body, err := c.get("/suggests/areas", url.Values{"text": {city}})
	if err != nil {
		return nil, err
	}

	var parsed suggestAreasResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode suggests/areas response: %w", err)
	}

	var areas []Area
	var flatten func(items []suggestAreaItem)
	flatten = func(items []suggestAreaItem) {
		for _, item := range items {
			if item.ID != "" {
				areas = append(areas, Area{ID: item.ID, Name: item.Text})
			}
			flatten(item.Areas)
		}
	}
	flatten(parsed.Items)

	return areas, nil
}

// Vacancy — краткая карточка вакансии из списка.
type Vacancy struct {
	ID   string
	Name string
}

type vacanciesResponse struct {
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"items"`
	Found int `json:"found"`
}

// ListVacancies возвращает до 100 вакансий по профессии в заданном городе
// за период от dateFrom (формат YYYY-MM-DD) до сегодня.
func (c *Client) ListVacancies(profession, areaID, dateFrom string) ([]Vacancy, error) {
	query := url.Values{
		"text":      {profession},
		"area":      {areaID},
		"date_from": {dateFrom},
		"per_page":  {strconv.Itoa(vacanciesPerPage)},
	}

	body, err := c.get("/vacancies", query)
	if err != nil {
		return nil, err
	}

	var parsed vacanciesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode vacancies response: %w", err)
	}

	vacancies := make([]Vacancy, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		vacancies = append(vacancies, Vacancy{ID: item.ID, Name: item.Name})
	}
	return vacancies, nil
}

type vacancyDetailsResponse struct {
	KeySkills []struct {
		Name string `json:"name"`
	} `json:"key_skills"`
}

// VacancyKeySkills возвращает названия ключевых навыков вакансии.
// Пустой срез, если key_skills отсутствует — вызывающий код пропускает
// такую вакансию молча (см. edge cases в ТЗ).
func (c *Client) VacancyKeySkills(vacancyID string) ([]string, error) {
	body, err := c.get("/vacancies/"+vacancyID, nil)
	if err != nil {
		return nil, err
	}

	var parsed vacancyDetailsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode vacancy details response: %w", err)
	}

	skills := make([]string, 0, len(parsed.KeySkills))
	for _, s := range parsed.KeySkills {
		skills = append(skills, s.Name)
	}
	return skills, nil
}

// DateFrom30DaysAgo — дата начала периода сбора (сегодня минус 30 дней).
func DateFrom30DaysAgo() string {
	return time.Now().AddDate(0, 0, -30).Format("2006-01-02")
}
