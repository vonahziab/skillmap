package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// boxWidth — внутренняя ширина рамки, как в приветственном баннере (main.go).
const boxWidth = 46

const progressBarWidth = 20

// completedEntry — строка в списке завершённых профессий блока прогресса.
type completedEntry struct {
	Profession string
	Skills     int
	Duration   time.Duration
}

// Reporter отображает live-прогресс сбора в консоли: перерисовывает блок
// целиком на месте (см. ТЗ, шаг 4), не оставляя мусора в истории терминала.
type Reporter struct {
	area   Area
	total  int
	client *Client

	startTime time.Time
	completed []completedEntry

	current         string
	currentIndex    int
	currentVacIdx   int
	currentVacTotal int
	currentSkills   int
	manualErrors    int

	// errorLog — тексты пропусков/ошибок после исчерпания retry (ТЗ, шаг 6/
	// раздел 10), выводятся отдельным списком в итоговой статистике (Summary),
	// а не в live-блоке — иначе они ломали бы перерисовку блока на месте.
	errorLog []string
	// zeroResult — профессии, по которым не набралось ни одного навыка (см.
	// ТЗ, раздел 10: 0 вакансий за период или все найденные без key_skills).
	zeroResult []string

	linesPrinted int
}

// NewReporter создаёт репортер для города и общего числа профессий.
// client используется только для чтения счётчика повторов запросов.
func NewReporter(area Area, total int, client *Client) *Reporter {
	return &Reporter{area: area, total: total, client: client, startTime: time.Now()}
}

// AddCompleted добавляет в список завершённых профессию, восстановленную
// из кэша при старте (без известного времени выполнения).
func (r *Reporter) AddCompleted(profession string, skills int) {
	r.completed = append(r.completed, completedEntry{Profession: profession, Skills: skills})
	if skills == 0 {
		r.zeroResult = append(r.zeroResult, profession)
	}
}

// StartProfession отмечает начало сбора новой профессии и перерисовывает блок.
func (r *Reporter) StartProfession(name string, index int) {
	r.current = name
	r.currentIndex = index
	r.currentVacIdx = 0
	r.currentVacTotal = 0
	r.currentSkills = 0
	r.render()
}

// SetVacancyTotal фиксирует размер списка вакансий текущей профессии.
func (r *Reporter) SetVacancyTotal(n int) {
	r.currentVacTotal = n
	r.render()
}

// VacancyProgress обновляет счётчик обработанных вакансий и уникальных
// навыков текущей профессии.
func (r *Reporter) VacancyProgress(vacancyIdx, skillsSoFar int) {
	r.currentVacIdx = vacancyIdx
	r.currentSkills = skillsSoFar
	r.render()
}

// LogError увеличивает счётчик ошибок/пропусков и сохраняет текст записи
// для итогового списка (Summary) — сам live-блок не печатает произвольный
// текст, чтобы не ломать перерисовку на месте.
func (r *Reporter) LogError(msg string) {
	r.manualErrors++
	r.errorLog = append(r.errorLog, msg)
	r.render()
}

// FinishProfession переносит профессию из "текущей" в список завершённых.
func (r *Reporter) FinishProfession(name string, skills int, duration time.Duration) {
	r.completed = append(r.completed, completedEntry{Profession: name, Skills: skills, Duration: duration})
	if skills == 0 {
		r.zeroResult = append(r.zeroResult, name)
	}
	r.current = ""
	r.render()
}

// Done завершает live-отображение финальной строкой.
func (r *Reporter) Done() {
	r.render()
	fmt.Println("\nСбор данных завершён.")
}

// Summary печатает итоговый экран статистики после генерации Excel (ТЗ,
// milestone 7): сводку по городу, времени, ошибкам и профессиям без
// результата, плюс список пропусков/ошибок, накопленный LogError.
func (r *Reporter) Summary(uniqueSkills int, filePath string) {
	fmt.Println()
	fmt.Println(boxTop())
	fmt.Println(boxLine("  Итоги сбора"))
	fmt.Println(boxBottom())
	fmt.Println()
	fmt.Printf("Город                    : %s\n", r.area.Name)
	fmt.Printf("Профессий собрано        : %d/%d\n", len(r.completed), r.total)
	fmt.Printf("Уникальных навыков       : %d\n", uniqueSkills)
	fmt.Printf("Время сбора              : %s\n", formatDurationLong(time.Since(r.startTime)))
	fmt.Printf("Ошибок/повторов          : %d\n", r.errorCount())
	if len(r.zeroResult) > 0 {
		fmt.Printf("Профессии без результата : %s\n", strings.Join(r.zeroResult, ", "))
	}
	fmt.Printf("Файл отчёта              : %s\n", filePath)

	if len(r.errorLog) > 0 {
		fmt.Println()
		fmt.Println("Пропуски и ошибки:")
		for _, e := range r.errorLog {
			fmt.Printf("  - %s\n", e)
		}
	}
}

func (r *Reporter) errorCount() int {
	n := r.manualErrors
	if r.client != nil {
		n += r.client.Retries
	}
	return n
}

// render перерисовывает весь блок прогресса на месте: поднимает курсор на
// число строк предыдущего кадра, очищает их и печатает новый кадр целиком.
func (r *Reporter) render() {
	lines := r.buildLines()

	var b strings.Builder
	if r.linesPrinted > 0 {
		fmt.Fprintf(&b, "\033[%dA\033[J", r.linesPrinted)
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	fmt.Print(b.String())
	r.linesPrinted = len(lines)
}

func (r *Reporter) buildLines() []string {
	done := len(r.completed)
	percent := 0
	filled := 0
	if r.total > 0 {
		percent = done * 100 / r.total
		filled = done * progressBarWidth / r.total
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)

	lines := []string{
		boxTop(),
		boxLine(fmt.Sprintf("  Город: %s        Вакансий макс: %d", r.area.Name, vacanciesPerPage)),
		boxLine(fmt.Sprintf("  Период: последние %d дней", searchPeriodDays)),
		boxBottom(),
		"",
		fmt.Sprintf("Прогресс: [%s] %d/%d (%d%%)", bar, done, r.total, percent),
		"",
	}

	if r.current != "" {
		elapsed := time.Since(r.startTime)
		lines = append(lines,
			fmt.Sprintf("Текущая профессия : %s (%d/%d)", r.current, r.currentIndex, r.total),
			fmt.Sprintf("Текущая вакансия  : %d/%d", r.currentVacIdx, r.currentVacTotal),
			fmt.Sprintf("Собрано навыков   : %d уникальных", r.currentSkills),
			fmt.Sprintf("Прошло времени    : %s", formatDurationLong(elapsed)),
			fmt.Sprintf("Осталось (оценка) : %s", formatETA(elapsed, done, r.total)),
			fmt.Sprintf("Ошибок/повторов   : %d", r.errorCount()),
			"",
		)
	}

	lines = append(lines, "Завершённые профессии:")
	for _, c := range r.completed {
		lines = append(lines, fmt.Sprintf("  [✓] %-24s %d %-8s %s",
			c.Profession, c.Skills, pluralSkills(c.Skills), formatDurationShort(c.Duration)))
	}
	if r.current != "" {
		lines = append(lines, fmt.Sprintf("  [ ] %-24s ← сейчас", r.current))
	}

	return lines
}

func boxTop() string    { return "╔" + strings.Repeat("═", boxWidth) + "╗" }
func boxBottom() string { return "╚" + strings.Repeat("═", boxWidth) + "╝" }

func boxLine(content string) string {
	r := []rune(content)
	if len(r) > boxWidth {
		r = r[:boxWidth]
	}
	return "║" + string(r) + strings.Repeat(" ", boxWidth-len(r)) + "║"
}

func formatDurationLong(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dмин %dсек", m, s)
}

func formatDurationShort(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dм %dс", m, s)
}

func formatETA(elapsed time.Duration, done, total int) string {
	if done == 0 || total == 0 {
		return "—"
	}
	remaining := total - done
	avg := elapsed / time.Duration(done)
	etaMin := int(math.Ceil((avg * time.Duration(remaining)).Minutes()))
	if etaMin < 1 {
		etaMin = 1
	}
	return fmt.Sprintf("~%dмин", etaMin)
}

func pluralSkills(n int) string {
	n = n % 100
	if n >= 11 && n <= 14 {
		return "навыков"
	}
	switch n % 10 {
	case 1:
		return "навык"
	case 2, 3, 4:
		return "навыка"
	default:
		return "навыков"
	}
}
