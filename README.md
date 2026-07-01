# SkillMap

Сборщик навыков с hh.ru → Excel.

Консольное приложение на Go: вводите город — приложение собирает навыки из
вакансий hh.ru по 20 фиксированным профессиям (IT, аналитика, финансы) через
официальное публичное API (без токена) и сохраняет результат в один
`.xlsx`-файл — таблицу «навыки × профессии».

## Возможности

- Поиск города по названию с динамическим разрешением ID через API hh.ru
- Сбор вакансий за последние 30 дней, до 100 вакансий на профессию
- Кэширование прогресса (`cache_<город>.json`) — можно продолжить прерванный сбор
- Live-прогресс в консоли: текущая профессия, ETA, счётчик ошибок
- Итоговый `.xlsx` со стилизацией: заливка, границы, freeze panes, альбомная ориентация
- Собирается в один `.exe` без внешних зависимостей

## Установка / запуск

Скачайте `skillmap.exe` (Windows 10/11 x64) и запустите. Приложение спросит
город и начнёт сбор.

```
Введите название города: Алматы
Город найден: Алматы (ID: 160)
```

По завершении в рабочей директории появятся:
- `<город>_навыки.xlsx` — итоговая таблица
- `cache_<город>.json` — кэш прогресса (можно удалить после успешного завершения)

## Сборка из исходников

Требуется Go 1.21+.

```bash
# Windows 64-bit (основная цель)
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o skillmap.exe .

# Mac
GOOS=darwin GOARCH=arm64 go build -o skillmap_mac .

# Linux
GOOS=linux GOARCH=amd64 go build -o skillmap_linux .
```

## Структура проекта

```
skillmap/
├── main.go        — точка входа, диалог с пользователем
├── api.go         — все запросы к hh.ru API
├── cache.go       — чтение/запись кэша
├── excel.go       — генерация .xlsx
├── progress.go    — отображение прогресса в консоли
├── go.mod
├── docs/
│   ├── adr/           — architecture decision records
│   ├── architecture.md
│   ├── milestones.md  — план разработки
│   └── runbook.md
├── README.md
└── LICENSE        — MIT
```

## Документация

- [docs/spec.md](docs/spec.md) — техническое задание
- [docs/milestones.md](docs/milestones.md) — план разработки по этапам
- [docs/architecture.md](docs/architecture.md) — устройство приложения
- [docs/runbook.md](docs/runbook.md) — эксплуатация: запуск, восстановление, диагностика
- [docs/adr/](docs/adr/) — ключевые архитектурные решения

## Технологии

- Go 1.21+, стандартный `net/http` и `bufio.Scanner`
- [`github.com/xuri/excelize/v2`](https://github.com/xuri/excelize) для генерации Excel
- Публичное API hh.ru (без токена)

## Автор

Baizhanov Arman — [@vonahziab](https://github.com/vonahziab)

Вопросы и предложения — через [issues репозитория](https://github.com/vonahziab/skillmap/issues).

## Лицензия

MIT
