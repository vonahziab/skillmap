package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// api.hh.ru отдаёт /suggests/areas без ограничений — используется как есть.
	apiBaseURL   = "https://api.hh.ru"
	apiUserAgent = "SkillMap/1.0 (github.com/vonahziab/skillmap)"

	// /vacancies и /vacancies/<id> на api.hh.ru блокируются DDoS-Guard даже
	// анонимно с валидными заголовками (проверено вручную и curl, и Go-клиентом
	// с разных сетей). Рабочий путь — обычные HTML-страницы сайта с браузерным
	// User-Agent и извлечением встроенного JSON-состояния страницы
	// (см. ADR-0003). Тот же приём используют актуальные скрейперы hh.ru,
	// например github.com/LuaSavage/hh_ru_parser.
	htmlSearchURL     = "https://hh.ru/search/vacancy"
	htmlVacancyURLFmt = "https://hh.ru/vacancy/%s"
	browserUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36"

	requestTimeout = 10 * time.Second
	requestPause   = 400 * time.Millisecond

	maxRetries   = 3
	retryPause   = 2 * time.Second
	maxRedirects = 5

	vacanciesPerPage = 100
	searchPeriodDays = 30
)

// Client — HTTP-клиент к hh.ru: JSON API для поиска города,
// HTML-страницы сайта для списка вакансий и деталей (см. константы выше).
type Client struct {
	http *http.Client
	// Retries — число неудачных попыток запроса с начала работы клиента,
	// используется репортером прогресса для поля "Ошибок/повторов" (progress.go).
	Retries int
}

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{http: &http.Client{
		Timeout: requestTimeout,
		Jar:     jar,
		// Редиректы (например hh.ru → <регион>.hh.ru) обрабатываются вручную
		// в doHTMLFetch, чтобы гарантированно переносить заголовки на каждый хоп.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

// withRetry — общая retry-обёртка: maxRetries попыток с паузой retryPause
// между ними, пауза requestPause после каждого запроса независимо от исхода.
// Каждая неудачная попытка увеличивает c.Retries (см. progress.go).
func (c *Client) withRetry(fetchOnce func() ([]byte, error)) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		body, err := fetchOnce()
		if err == nil {
			time.Sleep(requestPause)
			return body, nil
		}
		lastErr = err
		c.Retries++
		if attempt < maxRetries {
			time.Sleep(retryPause)
		}
	}
	time.Sleep(requestPause)
	return nil, fmt.Errorf("hh.ru request failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *Client) getJSON(path string, query url.Values) ([]byte, error) {
	reqURL := apiBaseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	return c.withRetry(func() ([]byte, error) { return c.doJSONGet(reqURL) })
}

func (c *Client) doJSONGet(reqURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", apiUserAgent)
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

func (c *Client) getHTML(rawURL string) ([]byte, error) {
	return c.withRetry(func() ([]byte, error) { return c.doHTMLFetch(rawURL) })
}

// doHTMLFetch запрашивает HTML-страницу сайта с браузерными заголовками,
// вручную проходя редиректы (hh.ru часто переадресует на региональный
// поддомен вида ufa.hh.ru) — так на каждом хопе точно сохраняются заголовки.
func (c *Client) doHTMLFetch(startURL string) ([]byte, error) {
	currentURL := startURL

	for i := 0; i < maxRedirects; i++ {
		req, err := http.NewRequest(http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", browserUserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return nil, fmt.Errorf("redirect from %s without Location header", currentURL)
			}
			base, err := url.Parse(currentURL)
			if err != nil {
				return nil, err
			}
			next, err := base.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("parse redirect location: %w", err)
			}
			currentURL = next.String()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status %d for %s: %s", resp.StatusCode, currentURL, strings.TrimSpace(string(body)))
		}
		return body, nil
	}
	return nil, fmt.Errorf("too many redirects starting from %s", startURL)
}

// initialStateRe вытаскивает встроенный JSON-стейт страницы hh.ru —
// тот же приём, что у github.com/LuaSavage/hh_ru_parser: обычный HTML-парсинг
// DOM теряет часть вакансий, а этот JSON содержит полные данные так же,
// как отдавал бы официальный JSON API.
var initialStateRe = regexp.MustCompile(`(?s)<template[^>]*id="HH-Lux-InitialState"[^>]*>(.*?)</template>`)

type initialState struct {
	VacancySearchResult *struct {
		Vacancies []struct {
			VacancyID int    `json:"vacancyId"`
			Name      string `json:"name"`
		} `json:"vacancies"`
	} `json:"vacancySearchResult"`
	VacancyView *struct {
		KeySkills struct {
			KeySkill []string `json:"keySkill"`
		} `json:"keySkills"`
	} `json:"vacancyView"`
}

func parseInitialState(body []byte) (*initialState, error) {
	m := initialStateRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("HH-Lux-InitialState template not found on page")
	}
	raw := html.UnescapeString(string(m[1]))

	var state initialState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("decode initial state: %w", err)
	}
	return &state, nil
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
	body, err := c.getJSON("/suggests/areas", url.Values{"text": {city}})
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

// ListVacancies возвращает до 100 вакансий по профессии в заданном городе
// за последние searchPeriodDays дней, парся HTML-страницу поиска hh.ru.
func (c *Client) ListVacancies(profession, areaID string) ([]Vacancy, error) {
	query := url.Values{
		"text":          {profession},
		"area":          {areaID},
		"items_on_page": {strconv.Itoa(vacanciesPerPage)},
		"search_period": {strconv.Itoa(searchPeriodDays)},
	}
	reqURL := htmlSearchURL + "?" + query.Encode()

	body, err := c.getHTML(reqURL)
	if err != nil {
		return nil, err
	}

	state, err := parseInitialState(body)
	if err != nil {
		return nil, fmt.Errorf("parse search page: %w", err)
	}
	if state.VacancySearchResult == nil {
		return nil, fmt.Errorf("search page has no vacancySearchResult")
	}

	vacancies := make([]Vacancy, 0, len(state.VacancySearchResult.Vacancies))
	for _, v := range state.VacancySearchResult.Vacancies {
		vacancies = append(vacancies, Vacancy{ID: strconv.Itoa(v.VacancyID), Name: v.Name})
	}
	return vacancies, nil
}

// VacancyKeySkills возвращает названия ключевых навыков вакансии, парся
// HTML-страницу вакансии hh.ru. Пустой срез, если навыки не указаны —
// вызывающий код пропускает такую вакансию молча (см. edge cases в ТЗ).
func (c *Client) VacancyKeySkills(vacancyID string) ([]string, error) {
	reqURL := fmt.Sprintf(htmlVacancyURLFmt, vacancyID)

	body, err := c.getHTML(reqURL)
	if err != nil {
		return nil, err
	}

	state, err := parseInitialState(body)
	if err != nil {
		return nil, fmt.Errorf("parse vacancy page: %w", err)
	}
	if state.VacancyView == nil {
		return nil, fmt.Errorf("vacancy page has no vacancyView")
	}
	return state.VacancyView.KeySkills.KeySkill, nil
}

// DateFrom30DaysAgo — дата начала периода сбора (сегодня минус 30 дней),
// используется только для отображения периода в итоговой статистике.
func DateFrom30DaysAgo() string {
	return time.Now().AddDate(0, 0, -30).Format("2006-01-02")
}
