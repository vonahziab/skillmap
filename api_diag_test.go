//go:build manual

package main

import (
	"fmt"
	"testing"
)

// TestDiagAPI бьёт живыми запросами в hh.ru и проверяет новый HTML-подход
// (см. api.go): /suggests/areas остаётся JSON API, /vacancies и /vacancies/<id>
// теперь читаются со страниц сайта. Не входит в обычный `go test ./...`
// (build tag "manual"), т.к. зависит от сети и внешнего сервиса.
//
// Запуск: make diag
func TestDiagAPI(t *testing.T) {
	c := NewClient()

	fmt.Println("--- /suggests/areas (JSON API) ---")
	areas, err := c.SearchAreas("Алматы")
	if err != nil {
		t.Fatalf("SearchAreas: %v", err)
	}
	fmt.Printf("OK: %+v\n", areas)
	if len(areas) == 0 {
		t.Fatal("expected at least one area")
	}
	areaID := areas[0].ID

	fmt.Println("--- hh.ru/search/vacancy (HTML) ---")
	vacancies, err := c.ListVacancies("Backend-разработчик", areaID)
	if err != nil {
		t.Fatalf("ListVacancies: %v", err)
	}
	fmt.Printf("OK: found %d vacancies, first 3:\n", len(vacancies))
	for i, v := range vacancies {
		if i >= 3 {
			break
		}
		fmt.Printf("  %s: %s\n", v.ID, v.Name)
	}
	if len(vacancies) == 0 {
		t.Fatal("expected at least one vacancy")
	}

	fmt.Println("--- hh.ru/vacancy/<id> (HTML) ---")
	skills, err := c.VacancyKeySkills(vacancies[0].ID)
	if err != nil {
		t.Fatalf("VacancyKeySkills: %v", err)
	}
	fmt.Printf("OK: skills for %s: %v\n", vacancies[0].ID, skills)
}
