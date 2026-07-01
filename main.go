package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
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

	if err := collectSkills(client, area, cache, reader); err != nil {
		fmt.Printf("Ошибка сбора данных: %v\n", err)
		return
	}
}

// collectSkills проходит по фиксированному списку профессий, пропуская уже
// завершённые (из кэша), собирает счётчики навыков по вакансиям и
// сохраняет кэш на диск сразу после каждой профессии (см. ТЗ, шаг 4;
// ADR-0004). Прогресс-бар и итоговая статистика — предмет milestone 5/7,
// здесь только сбор и чекпоинты.
func collectSkills(client *Client, area Area, cache *CacheData, reader *bufio.Scanner) error {
	if cache == nil {
		cache = NewCache(area.Name, area.ID)
	}

	done := make(map[string]bool, len(cache.Completed))
	for _, p := range cache.Completed {
		done[p] = true
	}

	for _, profession := range professions {
		if done[profession] {
			continue
		}

		fmt.Printf("\n=== %s ===\n", profession)

		vacancies, err := client.ListVacancies(profession, area.ID)
		if err != nil {
			fmt.Printf("Ошибка получения списка вакансий: %v (профессия пропущена)\n", err)
			continue
		}
		if len(vacancies) == 0 {
			fmt.Println("Вакансий не найдено за период.")
		}

		skills := make(map[string]int)
		for _, v := range vacancies {
			names, err := client.VacancyKeySkills(v.ID)
			if err != nil {
				fmt.Printf("  Пропуск вакансии %s (%s): %v\n", v.ID, v.Name, err)
				continue
			}
			for _, s := range names {
				skills[s]++
			}
		}

		if err := cache.MarkCompleted(profession, skills); err != nil {
			return fmt.Errorf("сохранить кэш после %q: %w", profession, err)
		}
		fmt.Printf("Собрано навыков: %d уникальных (вакансий: %d)\n", len(skills), len(vacancies))
	}

	fmt.Println("\nСбор данных завершён.")
	return nil
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
