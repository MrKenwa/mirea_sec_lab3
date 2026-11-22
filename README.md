# hashbrute

Учебный инструмент полного перебора предоставленных или явно разрешённых хэшей. Не используйте его против чужих учётных данных.


## Команды запуска

Сначала соберите бинарник:

```bash
go build -o hashbrute ./cmd/hashbrute
```

Быстрая проверка успешного поиска: MD5 от `a`, один кандидат:

```bash
./hashbrute \
  -algorithm md5 \
  -hash 0cc175b9c0f1b6a831c399e269772661 \
  -charset lower \
  -min-length 1 \
  -max-length 1 \
  -workers 1 \
  -progress 0
```

Поиск `123456` из `testdata/hashes.tsv` по SHA-1:

```bash
./hashbrute \
  -algorithm sha1 \
  -hash 7c4a8d09ca3762af61e59520943dc26494f8941b \
  -label easy \
  -charset digits \
  -min-length 6 \
  -max-length 6 \
  -workers 4 \
  -progress 1s
```

Проверка полного перебора без совпадения (`Status: exhausted`): целевой хэш соответствует `a`, но перебираются только цифры:

```bash
./hashbrute \
  -algorithm md5 \
  -hash 0cc175b9c0f1b6a831c399e269772661 \
  -charset digits \
  -min-length 1 \
  -max-length 2 \
  -workers 2 \
  -progress 0
```

Проверка остановки по тайм-ауту на медленном bcrypt (`Status: timeout`, код возврата `1`):

```bash
./hashbrute \
  -algorithm bcrypt \
  -hash '$2a$10$z4u9ZkvopUiiytaNX7wfGedy9Lu2ywUxwYpbsAR5YBrAuUs3YGXdi' \
  -label easy-timeout \
  -charset digits \
  -min-length 6 \
  -max-length 6 \
  -workers 1 \
  -timeout 2s \
  -progress 500ms
```

Для Argon2id весь PHC-хэш также нужно заключать в одинарные кавычки, иначе shell обработает символы `$`. Автоматический режим выбирает одного worker-а:

```bash
./hashbrute \
  -algorithm argon2id \
  -hash '$argon2id$v=19$m=65536,t=3,p=2$c2FsdHNhbHQ$PUF5UxxoUY++mMekkQwFurL0ZsTtB7lelO23zcyZQ0c' \
  -label easy-timeout \
  -charset digits \
  -min-length 6 \
  -max-length 6 \
  -timeout 2s \
  -progress 500ms
```

Сохранение результата в CSV; родительский каталог создаётся автоматически:

```bash
./hashbrute \
  -algorithm md5 \
  -hash e10adc3949ba59abbe56e057f20f883e \
  -label md5-easy \
  -charset digits \
  -min-length 6 \
  -max-length 6 \
  -workers 4 \
  -progress 0 \
  -output results/manual.csv
```

Повторный запуск с тем же `-output` добавляет новую строку, поэтому так можно сравнить `-workers 1`, `-workers 2`, `-workers 4` и автоматический режим без флага `-workers`.

Поддерживаются `md5`, `sha1`, `bcrypt`, `argon2id`. По умолчанию: `charset=digits`, длины `1..8`, автоматические workers, без ограничения времени (`timeout=0`), `progress=1s`; пустые `label` и `output`. Положительный `-timeout`, например `-timeout 30s`, можно явно задать для ограниченного эксперимента. Допустимы только четыре встроенных профиля: `digits` (`0123456789`), `lower` (`abcdefghijklmnopqrstuvwxyz`), `alnum` (строчные, прописные, цифры) и `printable` (ASCII 32–126). Произвольные алфавиты не поддерживаются.

Статусы: `found`, `exhausted`, `timeout`, `cancelled`, `error`; коды: 0, 0, 1, 130, 2 соответственно. Hex-дайджест использует две цифры `0-9a-f` на байт. bcrypt `$2a$10$...` включает вариант, cost, соль и checksum; PHC Argon2id содержит вариант, версию, параметры, соль и ключ. Явные несколько workers для Argon2id выводят оценку памяти.

## Серийные запуски на наборах данных

`testdata/hashes.tsv` — каталог 16 исходных хэшей. `testdata/experiments.example.tsv` — короткий манифест, который задаёт алгоритм, метку хэша, алфавит, границы длины и тайм-аут. Стандартная серия запускает каждый случай сначала с одним worker-ом, затем с несколькими:

```bash
rm -f /tmp/hashbrute-go.csv
OUTPUT=/tmp/hashbrute-go.csv MULTI_WORKERS=2 ./scripts/run-go.sh
cat /tmp/hashbrute-go.csv
```

Свой набор экспериментов можно передать без изменения файлов проекта:

```bash
cat > /tmp/hashbrute-experiments.tsv <<'EOF'
case_id	algorithm	label	charset	min_length	max_length	timeout
md5-easy	md5	easy	digits	6	6	0
sha1-easy	sha1	easy	digits	6	6	0
bcrypt-timeout	bcrypt	easy	digits	6	6	2s
argon2id-timeout	argon2id	easy	digits	6	6	2s
EOF

rm -f /tmp/hashbrute-custom.csv
EXPERIMENTS_FILE=/tmp/hashbrute-experiments.tsv \
OUTPUT=/tmp/hashbrute-custom.csv \
MULTI_WORKERS=4 \
./scripts/run-go.sh
cat /tmp/hashbrute-custom.csv
```

Для собственного каталога хэшей создайте TSV с колонками `algorithm`, `label`, `hash` и передайте оба файла:

```bash
HASHES_FILE=/path/to/hashes.tsv \
EXPERIMENTS_FILE=/path/to/experiments.tsv \
OUTPUT=/tmp/hashbrute-own.csv \
MULTI_WORKERS=4 \
./scripts/run-go.sh
```

Значение `algorithm + label` в манифесте должно существовать в каталоге хэшей. Поле `timeout` принимает `0` (без ограничения времени) либо положительное целое с суффиксом `s`, `m` или `h`, например `30s`, `5m`, `1h`. При `0` Go запускается с `-timeout 0`, а Hashcat — без аргумента `--runtime`.

Для сравнения с Hashcat: `brew install hashcat`, затем проверьте доступные устройства командами `hashcat --version` и `hashcat -I`. Режимы: MD5=0, SHA-1=100, bcrypt=3200, Argon2id=34000.

```bash
# CPU, стандартный манифест
DEVICE_TYPE=cpu OUTPUT=/tmp/hashbrute-hashcat-cpu.csv ./scripts/run-hashcat.sh

# GPU, свой манифест
DEVICE_TYPE=gpu \
EXPERIMENTS_FILE=/tmp/hashbrute-experiments.tsv \
OUTPUT=/tmp/hashbrute-hashcat-gpu.csv \
./scripts/run-hashcat.sh
```

Сравнивайте запуски только при одинаковых hash/charset/границах/ограничении времени и типе устройства. Для throughput полностью исчерпывайте малое пространство; порядок кандидатов у Go и Hashcat может отличаться.

