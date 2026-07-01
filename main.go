package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const banner = `╔══════════════════════════════════════════════╗
║              SkillMap                        ║
║      Сборщик навыков с hh.ru → Excel         ║
║                                              ║
║  Author : Baizhanov Arman                   ║
║  GitHub : github.com/vonahziab/skillmap      ║
║  TG/Git : @vonahziab                        ║
╚══════════════════════════════════════════════╝`

func main() {
	fmt.Println(banner)

	client := NewClient()
	reader := bufio.NewScanner(os.Stdin)

	area := promptCity(client, reader)
	fmt.Println()

	cache, err := LoadCache(area.Name, area.ID)
	if err != nil {
		fmt.Printf("Не удалось прочитать кэш: %v\n", err)
		cache = nil
	}

	if cache != nil && !promptContinueCache(cache, reader) {
		cache = nil
	}

	cache, reporter, err := collectSkills(client, area, cache, reader)
	if err != nil {
		fmt.Printf("Ошибка сбора данных: %v\n", err)
		return
	}

	if err := GenerateExcel(cache, professions); err != nil {
		fmt.Printf("Ошибка генерации Excel: %v\n", err)
		return
	}
	reporter.Summary(len(aggregateSkills(cache.Data)), excelFileName(cache.City))
}

// collectSkills проходит по фиксированному списку профессий, пропуская уже
// завершённые (из кэша), собирает счётчики навыков по вакансиям и
// сохраняет кэш на диск сразу после каждой профессии (см. ТЗ, шаг 4;
// ADR-0004). Ход сбора отображается репортером прогресса (progress.go,
// milestone 5); возвращённый репортер используется вызывающим кодом для
// печати итоговой статистики (milestone 7).
func collectSkills(client *Client, area Area, cache *CacheData, reader *bufio.Scanner) (*CacheData, *Reporter, error) {
	if cache == nil {
		cache = NewCache(area.Name, area.ID)
	}

	done := make(map[string]bool, len(cache.Completed))
	for _, p := range cache.Completed {
		done[p] = true
	}

	reporter := NewReporter(area, len(professions), client)
	for _, profession := range cache.Completed {
		reporter.AddCompleted(profession, len(cache.Data[profession]))
	}

	for i, profession := range professions {
		if done[profession] {
			continue
		}

		reporter.StartProfession(profession, i+1)
		start := time.Now()

		vacancies, err := client.ListVacancies(profession, area.ID)
		if err != nil {
			reporter.LogError(fmt.Sprintf("список вакансий %q: %v", profession, err))
			continue
		}
		reporter.SetVacancyTotal(len(vacancies))

		skills := make(map[string]int)
		for i, v := range vacancies {
			names, err := client.VacancyKeySkills(v.ID)
			if err != nil {
				reporter.LogError(fmt.Sprintf("вакансия %s (%s): %v", v.ID, v.Name, err))
				reporter.VacancyProgress(i+1, len(skills))
				continue
			}
			for _, s := range names {
				skills[s]++
			}
			reporter.VacancyProgress(i+1, len(skills))
		}

		if err := cache.MarkCompleted(profession, skills); err != nil {
			return nil, nil, fmt.Errorf("сохранить кэш после %q: %w", profession, err)
		}
		reporter.FinishProfession(profession, len(skills), time.Since(start))
	}

	reporter.Done()
	return cache, reporter, nil
}

// promptCity запрашивает у пользователя город, пока не будет однозначно
// выбран один вариант (см. ТЗ, шаг 2).
func promptCity(client *Client, reader *bufio.Scanner) Area {
	for {
		fmt.Print("Введите название города: ")
		city := readLine(reader)
		if city == "" {
			continue
		}

		areas, err := client.SearchAreas(city)
		if err != nil {
			fmt.Printf("Ошибка поиска города: %v\n", err)
			continue
		}

		switch len(areas) {
		case 0:
			fmt.Println("Город не найден, попробуйте снова.")
		case 1:
			fmt.Printf("Город найден: %s (ID: %s)\n", areas[0].Name, areas[0].ID)
			return areas[0]
		default:
			fmt.Println("Найдено несколько городов:")
			for i, a := range areas {
				fmt.Printf("  %d. %s (ID: %s)\n", i+1, a.Name, a.ID)
			}
			if idx, ok := promptChoice(reader, "Выберите номер: ", len(areas)); ok {
				return areas[idx]
			}
			fmt.Println("Некорректный номер, попробуйте снова.")
		}
	}
}

// promptContinueCache показывает найденный кэш и спрашивает, продолжить
// сбор с последней незавершённой профессии или начать заново (см. ТЗ, шаг 3).
// Возвращает true, если нужно продолжать с существующим кэшем; false, если
// пользователь выбрал начать заново (тогда кэш отбрасывается вызывающим кодом).
func promptContinueCache(cache *CacheData, reader *bufio.Scanner) bool {
	next := cache.NextProfession(professions)

	fmt.Printf("Найден кэш от %s (собрано %d/%d профессий)\n",
		cache.UpdatedAt.Format("2006-01-02 15:04"), len(cache.Completed), len(professions))
	if len(cache.Completed) > 0 {
		fmt.Printf("Последняя завершённая: %s\n", cache.Completed[len(cache.Completed)-1])
	}
	fmt.Println()
	fmt.Printf("  1. Продолжить с [%s]\n", next)
	fmt.Println("  2. Начать заново")

	for {
		idx, ok := promptChoice(reader, "Выберите: ", 2)
		if !ok {
			fmt.Println("Некорректный выбор, попробуйте снова.")
			continue
		}
		return idx == 0
	}
}

// promptChoice читает номер варианта в диапазоне [1, n] и возвращает его
// индекс с основанием 0. ok=false при некорректном вводе.
func promptChoice(reader *bufio.Scanner, prompt string, n int) (idx int, ok bool) {
	fmt.Print(prompt)
	input := readLine(reader)
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > n {
		return 0, false
	}
	return num - 1, true
}

func readLine(reader *bufio.Scanner) string {
	if !reader.Scan() {
		return ""
	}
	return strings.TrimSpace(reader.Text())
}
