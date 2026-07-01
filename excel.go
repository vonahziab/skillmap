package main

import (
	"fmt"
	"sort"

	"github.com/xuri/excelize/v2"
)

const (
	excelHeaderFill = "4472C4"
	excelEvenFill   = "DCE6F1"
	excelSkillWidth = 25.0
	excelColWidth   = 15.0
	excelTopSkills  = 20
)

// excelFileName — имя итогового отчёта для города (см. ТЗ, шаг 5).
func excelFileName(city string) string {
	return fmt.Sprintf("%s_навыки.xlsx", city)
}

// aggregateSkills сводит counts по профессиям в единый список навыков,
// отсортированный по убыванию суммы встречаемости (см. ADR-0007). При
// равной сумме порядок стабилизируется по алфавиту, чтобы вывод не зависел
// от порядка обхода map.
func aggregateSkills(data map[string]map[string]int) []string {
	totals := make(map[string]int)
	for _, skills := range data {
		for skill, count := range skills {
			totals[skill] += count
		}
	}

	skills := make([]string, 0, len(totals))
	for skill := range totals {
		skills = append(skills, skill)
	}
	sort.Slice(skills, func(i, j int) bool {
		if totals[skills[i]] != totals[skills[j]] {
			return totals[skills[i]] > totals[skills[j]]
		}
		return skills[i] < skills[j]
	})
	return skills
}

// GenerateExcel строит итоговый .xlsx с таблицей «навыки × профессии» для
// города и сохраняет его как <город>_навыки.xlsx, перезаписывая
// существующий файл (см. ТЗ, шаг 5).
func GenerateExcel(cache *CacheData, professions []string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := cache.City
	if _, err := f.NewSheet(sheet); err != nil {
		return fmt.Errorf("создать лист %q: %w", sheet, err)
	}
	f.DeleteSheet("Sheet1")

	skills := aggregateSkills(cache.Data)
	if len(skills) > excelTopSkills {
		skills = skills[:excelTopSkills]
	}
	lastCol, err := excelize.ColumnNumberToName(len(professions) + 1)
	if err != nil {
		return err
	}

	styles, err := newExcelStyles(f)
	if err != nil {
		return err
	}

	if err := writeExcelHeader(f, sheet, cache.City, professions, lastCol, styles); err != nil {
		return err
	}
	if err := writeExcelRows(f, sheet, skills, professions, cache.Data, styles); err != nil {
		return err
	}
	if err := styleExcelColumns(f, sheet, lastCol); err != nil {
		return err
	}

	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      1,
		YSplit:      2,
		TopLeftCell: "B3",
		ActivePane:  "bottomRight",
	}); err != nil {
		return fmt.Errorf("закрепить области: %w", err)
	}

	orientation := "landscape"
	if err := f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: &orientation}); err != nil {
		return fmt.Errorf("альбомная ориентация: %w", err)
	}

	if err := f.SaveAs(excelFileName(cache.City)); err != nil {
		return fmt.Errorf("сохранить %s: %w", excelFileName(cache.City), err)
	}
	return nil
}

// excelStyles — набор стилей отчёта (см. ТЗ, шаг 5, таблица «Стиль»).
type excelStyles struct {
	header    int // строка города / заголовки профессий
	skillEven int
	skillOdd  int
	dataEven  int
	dataOdd   int
}

func newExcelStyles(f *excelize.File) (excelStyles, error) {
	border := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	font := excelize.Font{Family: "Arial", Size: 10}

	header, err := f.NewStyle(&excelize.Style{
		Border:    border,
		Fill:      excelize.Fill{Type: "pattern", Color: []string{excelHeaderFill}, Pattern: 1},
		Font:      &excelize.Font{Family: "Arial", Size: 10, Bold: true, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return excelStyles{}, err
	}

	skillEven, err := f.NewStyle(&excelize.Style{
		Border: border,
		Fill:   excelize.Fill{Type: "pattern", Color: []string{excelEvenFill}, Pattern: 1},
		Font:   &excelize.Font{Family: "Arial", Size: 10, Bold: true},
	})
	if err != nil {
		return excelStyles{}, err
	}

	skillOdd, err := f.NewStyle(&excelize.Style{
		Border: border,
		Font:   &excelize.Font{Family: "Arial", Size: 10, Bold: true},
	})
	if err != nil {
		return excelStyles{}, err
	}

	dataEven, err := f.NewStyle(&excelize.Style{
		Border: border,
		Fill:   excelize.Fill{Type: "pattern", Color: []string{excelEvenFill}, Pattern: 1},
		Font:   &font,
	})
	if err != nil {
		return excelStyles{}, err
	}

	dataOdd, err := f.NewStyle(&excelize.Style{
		Border: border,
		Font:   &font,
	})
	if err != nil {
		return excelStyles{}, err
	}

	return excelStyles{
		header:    header,
		skillEven: skillEven,
		skillOdd:  skillOdd,
		dataEven:  dataEven,
		dataOdd:   dataOdd,
	}, nil
}

// writeExcelHeader пишет строку 1 (merged название города) и строку 2
// («Навыки» + список профессий).
func writeExcelHeader(f *excelize.File, sheet, city string, professions []string, lastCol string, styles excelStyles) error {
	if err := f.SetCellValue(sheet, "A1", city); err != nil {
		return err
	}
	if err := f.MergeCell(sheet, "A1", lastCol+"1"); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", lastCol+"1", styles.header); err != nil {
		return err
	}

	if err := f.SetCellValue(sheet, "A2", "Навыки"); err != nil {
		return err
	}
	for i, profession := range professions {
		cell, err := excelize.CoordinatesToCellName(i+2, 2)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, profession); err != nil {
			return err
		}
	}
	return f.SetCellStyle(sheet, "A2", lastCol+"2", styles.header)
}

// writeExcelRows пишет строки навыков (строка 3+): название навыка и
// количество упоминаний по каждой профессии (0, если навык не встречался).
func writeExcelRows(f *excelize.File, sheet string, skills, professions []string, data map[string]map[string]int, styles excelStyles) error {
	for row, skill := range skills {
		excelRow := row + 3
		skillCell, err := excelize.CoordinatesToCellName(1, excelRow)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, skillCell, skill); err != nil {
			return err
		}

		skillStyle, dataStyle := styles.skillOdd, styles.dataOdd
		if row%2 == 1 {
			skillStyle, dataStyle = styles.skillEven, styles.dataEven
		}
		if err := f.SetCellStyle(sheet, skillCell, skillCell, skillStyle); err != nil {
			return err
		}

		for col, profession := range professions {
			cell, err := excelize.CoordinatesToCellName(col+2, excelRow)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, data[profession][skill]); err != nil {
				return err
			}
			if err := f.SetCellStyle(sheet, cell, cell, dataStyle); err != nil {
				return err
			}
		}
	}
	return nil
}

// styleExcelColumns задаёт ширину колонки «Навыки» и колонок профессий
// (см. ТЗ, шаг 5, таблица «Стиль»).
func styleExcelColumns(f *excelize.File, sheet, lastCol string) error {
	if err := f.SetColWidth(sheet, "A", "A", excelSkillWidth); err != nil {
		return err
	}
	return f.SetColWidth(sheet, "B", lastCol, excelColWidth)
}
