# Пакет `internal/search`

Пакет выполняет полный перебор кандидатов. Он не знает формат хэша: каждый кандидат передаётся в `checker.Checker`.

## Файлы

| Файл | Назначение |
|---|---|
| `config.go` | Алфавиты, параметры поиска и размер пространства перебора |
| `cursor.go` | Разбиение перебора на задания и генерация кандидатов |
| `engine.go` | Workers, проверка кандидатов, прогресс и остановка поиска |

## Общий поток

```text
CLI
 ├─ ResolveCharset(...) → Charset
 ├─ NewConfig(...)      → Config
 └─ Search(...)
      ├─ Jobs(cfg) создаёт задания
      ├─ workers читают задания
      ├─ Cursor генерирует кандидатов
      ├─ Checker.Match проверяет кандидатов
      └─ Search возвращает Result
```

## `config.go`

### `Charset`

```go
type Charset struct {
    Name    string
    Symbols []byte
}
```

- `Name` — имя профиля, например `digits`.
- `Symbols` — символы, из которых строятся кандидаты.

Встроенные профили:

| Имя | Символы |
|---|---|
| `digits` | `0-9` |
| `lower` | `a-z` |
| `alnum` | `a-z`, `A-Z`, `0-9` |
| `printable` | печатные ASCII-символы с кодами 32–126 |

`ResolveCharset(name)` возвращает профиль. Неизвестное имя считается ошибкой.

### `Config`

```go
type Config struct {
    Charset          Charset
    MinLength        int
    MaxLength        int
    Workers          int
    ProgressInterval time.Duration
    Total            uint64
}
```

- `MinLength`, `MaxLength` — диапазон длины кандидата.
- `Workers` — количество параллельных обработчиков.
- `ProgressInterval` — интервал обновления прогресса. `0` отключает прогресс.
- `Total` — полное количество кандидатов.

`NewConfig` проверяет параметры и вычисляет `Total`.

Размер пространства:

```text
Total = charsetSize^MinLength + ... + charsetSize^MaxLength
```

Пример: `digits`, длина `1..2`:

```text
10¹ + 10² = 110 кандидатов
```

Ограничения:

- алфавит не должен быть пустым;
- минимальная длина — не меньше `1`;
- минимальная длина — не больше максимальной;
- максимальная длина — не больше `64`;
- workers — больше `0`;
- интервал прогресса — неотрицательный;
- `Total` должен помещаться в `uint64`.

Для расчёта используется `math/big`, чтобы обнаружить переполнение до преобразования в `uint64`.

## `cursor.go`

### `Job`

```go
type Job struct {
    Length     int
    FirstIndex int
}
```

Одно задание фиксирует:

- длину кандидата;
- индекс первого символа в алфавите.

`Jobs(cfg)` создаёт задание для каждой пары `длина + первый символ`.

Пример для алфавита `ab` и длины `1..2`:

```text
{Length: 1, FirstIndex: 0} → a
{Length: 1, FirstIndex: 1} → b
{Length: 2, FirstIndex: 0} → aa, ab
{Length: 2, FirstIndex: 1} → ba, bb
```

Так пространство делится между workers без пересечений.

Количество заданий:

```text
(MaxLength - MinLength + 1) × размер алфавита
```

### `Cursor`

`Cursor` хранит один текущий кандидат и изменяет его на месте. Он работает только внутри одного `Job`: длина и первый символ уже зафиксированы.

### Поля `Cursor`

Исходный код:

```go
type Cursor struct {
	charset []byte
	storage []byte
	length  int
	indexes []int
	done    bool
}
```

Что хранится в полях:

| Поле | Что содержит |
|---|---|
| `charset` | Алфавит, например `[]byte{'a', 'b', 'c'}` |
| `storage` | Байты текущего кандидата, например `[]byte{'b', 'a', 'c'}` |
| `length` | Длина текущего кандидата |
| `indexes` | Индекс каждого байта `storage` в `charset`; для `bac` это `[1, 0, 2]` |
| `done` | `true`, когда внутри задания больше нет кандидатов |

`storage` и `indexes` описывают одно состояние в двух формах:

```text
charset = [a b c]
indexes = [1 0 2]
storage = [b a c]
```

Для каждой позиции выполняется:

```text
storage[i] = charset[indexes[i]]
```

### Создание: `NewCursor`

Код с комментариями:

```go
func NewCursor(charset []byte, job Job, storage []byte) (Cursor, error) {
	// Нельзя работать без алфавита, с некорректной длиной,
	// с отсутствующим первым символом или со слишком коротким буфером.
	if len(charset) == 0 ||
		job.Length < 1 ||
		job.FirstIndex < 0 ||
		job.FirstIndex >= len(charset) ||
		len(storage) < job.Length {
		return Cursor{}, fmt.Errorf("invalid cursor")
	}

	// Для каждой позиции кандидата хранится индекс символа в charset.
	// Новый []int заполнен нулями.
	indexes := make([]int, job.Length)

	// Первый символ закреплён за Job и в Next не изменяется.
	indexes[0] = job.FirstIndex
	storage[0] = charset[job.FirstIndex]

	// Остальные позиции начинают с первого символа алфавита.
	// indexes[i] уже равен 0, поэтому отдельно меняется только storage.
	for i := 1; i < job.Length; i++ {
		storage[i] = charset[0]
	}

	return Cursor{
		charset: charset,
		storage: storage,
		length:  job.Length,
		indexes: indexes,
	}, nil
}
```

Пример:

```text
charset        = [a b c]
job.Length     = 3
job.FirstIndex = 1

indexes = [1 0 0]
storage = [b a a]
Value() = "baa"
```

Первый кандидат готов сразу после `NewCursor`. Перед ним не нужно вызывать `Next`.

### Текущее значение: `Value`

Исходный код:

```go
func (c *Cursor) Value() []byte {
	return c.storage[:c.length]
}
```

`Value` ничего не вычисляет. Он возвращает срез используемой части буфера `storage`.

Если буфер имеет размер `5`, а `length == 3`:

```text
storage = [b a c ? ?]
Value() = [b a c]
```

Возвращаемый `[]byte` ссылается на тот же массив, что и `storage`. Копии нет. Поэтому:

- вызов дешёвый;
- после `Next()` содержимое ранее полученного среза изменится;
- если значение нужно сохранить, его необходимо скопировать.

Именно поэтому при найденном совпадении `engine.go` делает копию:

```go
password := string(append([]byte(nil), cursor.Value()...))
```

Без копирования следующий вызов `Next` мог бы изменить найденный пароль.

### Переход: `Next`

Исходный код с комментариями:

```go
func (c *Cursor) Next() bool {
	// done означает, что задание уже закончено.
	// При length == 1 есть только один кандидат:
	// первый символ закреплён в Job, других позиций нет.
	if c.done || c.length == 1 {
		c.done = true
		return false
	}

	// Движемся справа налево.
	// Позиция 0 не рассматривается: она закреплена за Job.
	for i := c.length - 1; i >= 1; i-- {
		// Если справа есть следующий символ алфавита,
		// увеличиваем индекс этой позиции.
		if c.indexes[i]+1 < len(c.charset) {
			c.indexes[i]++
			c.storage[i] = c.charset[c.indexes[i]]
			return true // новый кандидат готов
		}

		// Позиция дошла до последнего символа алфавита.
		// Сбрасываем её в 0 и переносим увеличение влево.
		c.indexes[i] = 0
		c.storage[i] = c.charset[0]
	}

	// Все изменяемые позиции переполнились.
	// Кандидатов внутри этого Job больше нет.
	c.done = true
	return false
}
```

Возвращаемое значение:

| Результат | Значение |
|---|---|
| `true` | `storage` изменён, новый кандидат доступен через `Value()` |
| `false` | задание закончено, нового кандидата нет |

`Next` работает как прибавление единицы к числу. Основание системы счисления равно `len(charset)`, а `indexes` — его цифры. Перенос идёт справа налево.

Пример для `charset = [a b c]`, длины `3` и закреплённого первого символа `b`:

| До `Next` | Действие | После `Next` | Результат |
|---|---|---|---|
| `[1,0,0]` = `baa` | последняя позиция `0 → 1` | `[1,0,1]` = `bab` | `true` |
| `[1,0,1]` = `bab` | последняя позиция `1 → 2` | `[1,0,2]` = `bac` | `true` |
| `[1,0,2]` = `bac` | последняя позиция сбрасывается, средняя `0 → 1` | `[1,1,0]` = `bba` | `true` |
| `[1,2,2]` = `bcc` | обе изменяемые позиции сбрасываются | `[1,0,0]` = `baa` | `false`, `done = true` |

Последнее состояние `baa` после `false` не является новым кандидатом: `baa` уже был проверен первым. После `false` worker завершает это задание и берёт следующее.

### Как worker использует курсор

Упрощённый цикл из `engine.go`:

```go
cursor, _ := NewCursor(cfg.Charset.Symbols, job, storage)

for {
	// Сначала проверяется текущее значение.
	// На первой итерации это начальный кандидат из NewCursor.
	if target.Match(cursor.Value()) {
		// Совпадение найдено.
		return
	}

	// Подготавливаем следующий кандидат.
	// false означает, что текущий Job полностью перебран.
	if !cursor.Next() {
		break
	}
}
```

Порядок для задания `b**` при алфавите `abc`:

```text
NewCursor → baa
Next      → bab
Next      → bac
Next      → bba
...
Next      → bcc
Next      → false
```

## `engine.go`

### Вход `Search`

```go
func Search(
    ctx context.Context,
    cfg Config,
    target checker.Checker,
    onProgress func(Snapshot),
) (Result, error)
```

- `ctx` останавливает поиск по тайм-ауту или `Ctrl+C`.
- `cfg` задаёт пространство поиска и число workers.
- `target` проверяет кандидата методом `Match`.
- `onProgress` получает периодические снимки. `nil` отключает их.

### Запуск заданий

`Search` создаёт:

- `jobs` — канал заданий;
- `found` — буферизированный канал на один найденный пароль;
- внутренний `workCtx` для остановки всех goroutines после совпадения.

Отдельная goroutine вызывает `Jobs(cfg)` и отправляет задания в `jobs`. После отправки всех заданий канал закрывается.

### Работа worker-а

Каждый worker:

1. один раз выделяет буфер размером `MaxLength`;
2. получает `Job` из канала;
3. создаёт `Cursor` поверх своего буфера;
4. вызывает `target.Match(cursor.Value())`;
5. переходит к следующему кандидату через `cursor.Next()`;
6. после завершения задания берёт следующее.

Буфер переиспользуется между кандидатами и заданиями одного worker-а.

### Найденный пароль

При совпадении worker:

1. копирует текущий кандидат в отдельную строку;
2. отправляет строку в `found`;
3. вызывает `cancel()` для внутреннего контекста;
4. завершает работу.

Копия нужна, потому что буфер `Cursor` переиспользуется и меняется при следующей итерации.

Остальные workers видят отмену `workCtx` и завершаются.

### Счётчик попыток

Общий счётчик — `atomic.Uint64`.

Каждый worker сначала считает попытки локально. После каждых `4096` попыток значение добавляется в общий счётчик. Остаток добавляется при завершении worker-а.

Это уменьшает количество атомарных операций. Поэтому промежуточный прогресс может не учитывать до одного пакета из `4096` попыток на каждого worker-а. Итоговый `Result.Attempts` содержит полный счётчик.

### Прогресс

Если `onProgress != nil` и `ProgressInterval > 0`, отдельная goroutine периодически создаёт `Snapshot`:

```go
type Snapshot struct {
    Attempts     uint64
    Elapsed      time.Duration
    AverageRate  float64
    Progress     float64
    ETA          time.Duration
    ETAAvailable bool
}
```

Расчёты:

```text
AverageRate = Attempts / Elapsed
Progress    = Attempts / Total
ETA         = (Total - Attempts) / AverageRate
```

`ETAAvailable` устанавливается только при ненулевой скорости и пока перебрано меньше `Total`.

После завершения workers обновление прогресса останавливается.

### Результат

```go
type Result struct {
    Status   Status
    Password string
    Attempts uint64
    Elapsed  time.Duration
    Rate     float64
}
```

Возможные статусы:

| Статус | Условие |
|---|---|
| `found` | Checker принял один из кандидатов |
| `exhausted` | Все кандидаты проверены, совпадений нет |
| `timeout` | Истёк deadline контекста |
| `cancelled` | Контекст отменён по другой причине, например `Ctrl+C` |

`Password` заполняется только для `found`.

Средняя итоговая скорость:

```text
Rate = Attempts / Elapsed
```

## Пример полного перебора

Параметры:

```text
charset = ab
min     = 1
max     = 2
workers = 2
```

Создаются задания:

```text
длина 1, первый символ a → a
длина 1, первый символ b → b
длина 2, первый символ a → aa, ab
длина 2, первый символ b → ba, bb
```

Workers получают задания из общего канала. Порядок выполнения заданий может различаться, поэтому при нескольких workers порядок проверок не гарантирован. Каждый кандидат всё равно принадлежит ровно одному заданию.

## Связь с CLI

`internal/cli/run.go` использует пакет так:

```text
-charset    → ResolveCharset
длины,
workers,
-progress   → NewConfig
-timeout    → context.WithTimeout
checker     → Search
Result      → консоль и CSV
```

Пакет `search` отвечает только за перебор. Разбор аргументов, выбор алгоритма хэширования, вывод и запись CSV находятся в других пакетах.
