package main

// professionGroup — группа однородных профессий: один лист Excel на группу,
// со своим топ-навыков (см. ADR-0009). Name используется как имя листа и
// должно быть валидным (без []:*?/\, ≤31 символа).
type professionGroup struct {
	Name    string
	Members []string
}

// professionGroups — фиксированный набор профессий, разбитый на однородные
// группы (см. ТЗ, раздел 4; ADR-0005, ADR-0009). Раньше это был один плоский
// список; профессии из разных миров (разработка / аналитика / финансы) имеют
// разный объём вакансий, поэтому в общей таблице топ навыков одной группы
// вытеснял навыки остальных. Порядок групп и профессий внутри важен: он задаёт
// порядок листов, столбцов и обхода при сборе.
var professionGroups = []professionGroup{
	{
		Name: "Разработка",
		Members: []string{
			"Backend-разработчик",
			"Frontend-разработчик",
			"Fullstack-разработчик",
			"Python-разработчик",
			"Java-разработчик",
			"DevOps-инженер",
			"Data Scientist",
			"Data Analyst",
		},
	},
	{
		Name: "Аналитика и управление",
		Members: []string{
			"Бизнес-аналитик",
			"Системный аналитик",
			"BI-аналитик",
			"Product Manager",
			"Project Manager (IT)",
		},
	},
	{
		Name: "Финансы и экономика",
		Members: []string{
			"Экономист",
			"Финансовый аналитик",
			"Бухгалтер",
			"Аудитор",
			"Налоговый консультант",
			"Инвестиционный аналитик",
			"Финансовый менеджер",
		},
	},
}

// professions — плоский список всех профессий в порядке групп. Используется
// для сбора данных и совместимости кэша (порядок совпадает с прежним).
var professions = flattenGroups(professionGroups)

func flattenGroups(groups []professionGroup) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g.Members...)
	}
	return out
}
