# Спецификация тестов бэкенда

Этот документ описывает, что именно проверяют тесты бэкенда. У каждого
кейса есть идентификатор (например `IT-UC-TRK-LIKE-02`). Тот же идентификатор
стоит в Allure, поэтому упавший кейс в отчёте указывает на строку
[части 3](#part-3) и на поведение из [части 1](#part-1).

Как запускать наборы и собирать отчёт, описано в [TESTING.md](TESTING.md).
Тексты внутри Allure (заголовки, Given / When / Then, шаги) остаются на
английском: отчёт читается по ним, а эта спецификация их поясняет.

## Область

| Слой | Пакеты | Стили тестов | База данных |
| --- | --- | --- | --- |
| Домен | `internal/domain` | classic | нет |
| Бизнес-логика | `internal/usecase` | classic (фейки с состоянием), london (моки и шпионы), integration | integration: настоящий PostgreSQL |
| Доступ к данным | `internal/infra/adapters/postgres` | stub (`stubDB`), pgxmock, integration | integration: настоящий PostgreSQL |
| Сквозной сценарий | `e2e` | e2e | docker compose: PostgreSQL и RustFS |

Сквозной тест `E2E-DEMO-01` проходит демо-сценарий MVP по HTTP: регистрация,
загрузка, публикация, лайк и стрим. Отдельными модульными тестами по-прежнему
не покрыты все обработчики (`internal/transport/http`), адаптер S3 вне этого
сценария, краевые случаи разбора JWT, загрузка конфигурации и монитор
готовности. Дымовая проверка системы в CI (`.github/workflows/ci.yml`, job
`System under test`) открывает API и фронтенд и регистрирует одного пользователя.

### Формат идентификатора

`<префикс>-<компонент>-<метод>-<NN>`

- Префикс: `DOM` — модульный тест домена, `UC` — модульный тест бизнес-логики,
  `DA` — модульный тест доступа к данным на `stubDB`, `DA-PGX` — то же на
  pgxmock, `IT-UC` и `IT-DA` — интеграция с PostgreSQL, `E2E` — сквозной сценарий
  против поднятого docker compose.
- Компонент: `AUTH`, `USER`/`USR`, `TRK`, `LIKE`, `PAGE`, `ERR`, `DEMO`.
- Метод: короткое имя проверяемого метода, например `LIKE`, `COMPLETE`,
  `GETEMAIL`.

### Что показывает кейс в Allure

- Вкладка Behaviors: epic — слой, feature — компонент, story — метод.
- Вкладка Suites: родительский набор — слой, набор — компонент, поднабор — метод.
- Заголовок `[ID] title`, severity, тег со стилем теста, метка `testTechnique`.
- Параметры: входы, которые отличают кейс (email, размер, лимит, статус).
- Описание: таблица Given / When / Then и ссылка на раздел спецификации.
- Шаги: `Arrange`, `Act`, `Assert`. У сквозного сценария шаги именованные
  (`01 ready`, `02 register artist`, …). Падение крепится к шагу, на котором
  оно случилось.

Названия слоёв, стилей и техник в отчёте английские: `Domain`,
`Business logic`, `Data access`, `End to end`; `classic`, `london`, `stub`,
`pgxmock`, `integration`, `e2e`; `boundary-value analysis`,
`equivalence partitioning`, `state-transition testing`, `decision-table testing`,
`error guessing`.

### Контракт ошибок

Бизнес-ошибки — это значения `domain.Error` с кодом и фиксированным текстом.
HTTP-слой переводит код в статус в `internal/transport/http/respond.go`.

| Код | HTTP-статус | Обычная причина |
| --- | --- | --- |
| `invalid` | 400 | проверка входа, некорректный UUID (SQLSTATE 22P02) |
| `unauthorized` | 401 | неверные учётные данные, вызывающий не владелец трека |
| `not_found` | 404 | нет строки, нарушение внешнего ключа (SQLSTATE 23503, см. KI-1) |
| `conflict` | 409 | нарушение уникальности (SQLSTATE 23505), неверный статус трека |
| `internal` | 500 | любой другой сбой зависимости, с сохранённой причиной |

## <a id="part-1"></a>Часть 1. Поведенческая спецификация

Каждый раздел перечисляет входы, ограничения, наблюдаемый результат, ошибки
и побочные эффекты одного метода. Имена разделов совпадают со значением
`Component.Method` в описании Allure и поэтому оставлены как в коде.

### Домен

#### RegisterInput.Validate

- Вход: email, пароль, имя.
- Перед проверками email обрезается по краям и переводится в нижний регистр
  на месте.
- Ошибки, в этом порядке: пустой email или email без `@` даёт invalid
  `valid email is required`; пароль короче 8 байт даёт invalid
  `password must be at least 8 characters`.
- Имя не проверяется.

#### RegisterInput.User

- Возвращает `User` с email и именем. Идентификатор остаётся пустым, пароль
  не копируется.

#### LoginInput.Validate

- Email обрезается и переводится в нижний регистр.
- Ошибки: пустой email даёт invalid `email is required`; пустой пароль даёт
  invalid `password is required`. Любой непустой пароль проходит: длина
  проверяется только при регистрации.

#### UserID.Validate

- Пустой идентификатор даёт invalid `user id is required`. Формат здесь не
  проверяется: некорректный UUID отклоняет PostgreSQL (22P02, invalid
  `invalid id`).

#### UserWrite.Validate

- Email нормализуется так же, как при регистрации. Пустой или без `@` даёт
  invalid `valid email is required`. При обновлении пароль необязателен.

#### UserWrite.ValidateCreate

- Сначала `Validate`, затем длина пароля не меньше 8, иначе invalid
  `password must be at least 8 characters`.

#### UserWrite.User

- Переносит email и имя и ставит переданный идентификатор. Пароль не копируется.

#### Track.AudioObjectKey

- Возвращает сохранённый ключ объекта или `<id>.mp3`, если ключ пуст.

#### Track.OwnedBy

- Чужой идентификатор пользователя даёт unauthorized `not allowed`.

#### Track.ConfirmUpload

- Вход: размер, который сообщило хранилище объектов.
- Размер 0 или меньше даёт invalid `upload not found`.
- Если у трека заявленный размер больше 0 и размеры не совпали: invalid
  `upload size mismatch: expected <declared>, got <stored>`.

#### Track.MarkProcessing

- `pending` становится `processing`. `processing` остаётся `processing`
  (повторный вызов ничего не меняет).
- `ready` даёт conflict `track is not awaiting upload`; статус не меняется.

#### Track.MarkReady

- `processing` становится `ready`. Любой другой статус даёт conflict
  `track is not ready to publish`; статус не меняется.

#### Track.EnsureReady

- Любой статус, кроме `ready`, даёт conflict `track is not ready`.

#### TrackID.Validate, TrackWrite.Validate

- Пустой идентификатор трека даёт invalid `track id is required`.
- Пустое название даёт invalid `title is required`. Исполнитель необязателен.

#### TrackUploadInit.Validate, TrackUploadInit.ValidateSize, TrackUploadInit.Track

- Обязательны в таком порядке: идентификатор пользователя
  (`user id is required`), название (`title is required`), размер больше 0
  (`size is required`).
- `ValidateSize(max)`: при `max > 0` размер больше `max` даёт invalid
  `file too large`; размер, равный `max`, проходит. `max <= 0` означает, что
  предела нет.
- `Track()` собирает трек в статусе `pending` с идентификатором пользователя,
  названием, исполнителем и размером.

#### TrackUploadComplete.Validate

- Сначала проверяется идентификатор трека (`track id is required`), затем
  идентификатор пользователя (`user id is required`).

#### TrackLike.Validate

- Сначала идентификатор пользователя (`user id is required`), затем
  идентификатор трека (`track id is required`).

#### PageQuery.Validate

- Лимит: 0 и меньше становится 20; от 1 до 100 сохраняется; больше 100 даёт
  invalid `limit exceeded`.
- Курсор: пустой означает смещение 0; неотрицательное целое — это смещение;
  всё остальное даёт invalid `invalid cursor`.

#### PageQuery.Page

- Режет уже отфильтрованный список в памяти. `NextCursor` — следующее
  смещение, на последней странице он пуст. Курсор за концом списка даёт
  пустую страницу. `Limit` в результате — действующий лимит.

#### TrackFeedQuery.Validate, TrackFeedQuery.Filter

- Исполнитель обрезается по краям, затем проверяется страница. Фильтр всегда
  `status = ready` плюс исполнитель; владелец не задаётся.

#### TrackOwnerQuery.Validate, TrackOwnerQuery.Filter

- Идентификатор пользователя обязателен, затем проверяется страница. В фильтре
  владелец и переданный статус (пустой статус означает все статусы);
  исполнитель не задаётся.

#### TrackLikeListQuery.Validate

- Идентификатор пользователя обязателен, затем проверяется страница.

#### Error.Error, Error.Unwrap

- `Error()` — это `<code>: <message>` или `<code>: <message>: <cause>`.
- `Unwrap` возвращает причину или `nil`, если её нет.

#### Error.NewError, Error.Wrap

- Собирают ошибку с заданными кодом и текстом. `Wrap` сохраняет причину для
  `errors.Is` и `errors.As`.

#### Error.NotFound, Error.Invalid, Error.Unauthorized, Error.Conflict, Error.Internal

- Собирают ошибки с соответствующим кодом и без причины.

#### Error.As, Error.Is

- `As` находит `*domain.Error` в любом месте цепочки обёрток. `Is(err, code)`
  сообщает, есть ли у этой ошибки заданный код. Оба возвращают ложь для `nil`
  и для чужих ошибок.

### Бизнес-логика

#### AuthService.Register

- Проверяет вход, хеширует пароль, сохраняет пользователя, выпускает токен.
- Побочный эффект: одна строка `users` с хешем bcrypt. Пароль в открытом виде
  не сохраняется.
- Ошибки: проверка входа (зависимости не вызываются), ошибка хешера, ошибка
  репозитория (повторный email: conflict `email already exists`), ошибка
  выпуска токена (пользователь к этому моменту уже сохранён).

#### AuthService.Login

- Проверяет вход, загружает пользователя по нормализованному email, сравнивает
  хеш, выпускает токен.
- Неизвестный email (репозиторий вернул not_found) и неверный пароль
  (несовпадение bcrypt) оба дают unauthorized `invalid credentials`, поэтому
  ответ не сообщает, какие учётки существуют. Остальные ошибки репозитория
  возвращаются как есть.

Каждый метод `UserService` и `TrackService` возвращает ошибки репозитория без
изменений, а вход, не прошедший проверку, до репозитория не доходит.

#### UserService.List

- Возвращает всех пользователей из репозитория в его порядке.

#### UserService.GetByID

- Требует непустой идентификатор (invalid `user id is required`).

#### UserService.Create

- Проверяет вход через `UserWrite.ValidateCreate`, хеширует пароль, сохраняет
  пользователя вместе с хешем. Ошибка хешера останавливает вызов до репозитория.

#### UserService.Update

- Требует идентификатор, проверяет и нормализует email, сохраняет email и имя.
  Пароль не меняется.

#### UserService.Delete

- Требует идентификатор. Удаление пользователя, у которого есть треки, база
  отклоняет (KI-1).

#### TrackService.List

- Публичная лента: только треки `ready`, необязательный фильтр по исполнителю
  (точное совпадение, без учёта регистра, пробелы по краям игнорируются),
  постранично.

#### TrackService.ListByUploader

- Каталог загрузившего: все статусы, если статус не передан, постранично.

#### TrackService.ListLiked

- Треки `ready`, которые пользователь лайкнул, сначала самый новый лайк,
  постранично.

#### TrackService.Like

- Трек должен существовать и быть `ready`. Иначе not_found или conflict
  `track is not ready`, и ничего не записывается.
- Повторный лайк того же трека даёт conflict `track already liked`.

#### TrackService.Unlike

- Трек должен существовать. Снятие лайка, которого нет, завершается успехом
  (повторный вызов ничего не меняет).

#### TrackService.GetByID

- Требует идентификатор. Возвращает трек в любом статусе (KI-3).

#### TrackService.Update

- Требует идентификатор и название. Загружает трек и меняет только название и
  исполнителя; статус, размер, ключ объекта и владелец сохраняются. Проверки
  владельца нет (KI-2).

#### TrackService.Delete

- Требует идентификатор. Удаляет трек и каскадом его лайки. Проверки владельца
  нет (KI-2).

#### TrackService.InitUpload

- Проверяет вход и размер относительно `TRACK_MAX_BYTES`.
- Создаёт трек `pending` с ключом объекта `<id>.mp3`, затем подписывает URL
  загрузки.
- Если подпись не удалась, созданный трек удаляется и возвращается ошибка
  подписи. Если не удалось и это удаление, возвращается ошибка удаления.

#### TrackService.CompleteUpload

- Порядок: загрузить трек, проверить владельца (unauthorized `not allowed`),
  проверить объект (`upload not found`, `upload size mismatch: ...`), перевести
  `pending -> processing` и сохранить, проверить объект ещё раз, перевести
  `processing -> ready` и сохранить.
- Трек `ready` даёт conflict `track is not awaiting upload`, уже после проверки
  объекта.
- При любой ошибке до первого обновления сохранённый статус не меняется. Два
  обновления не входят в одну транзакцию: если второй шаг падает, трек остаётся
  `processing`, и повторный вызов его завершает, потому что `MarkProcessing`
  на `processing` ничего не меняет.

#### TrackService.StreamURL

- Только для треков `ready`, иначе conflict `track is not ready`. Возвращает
  подписанный URL чтения.

### Доступ к данным

Ошибки PostgreSQL для каждого метода переводит `mapError` (`pool.go`):

| SQLSTATE | Результат |
| --- | --- |
| 23505 на `track_likes_pkey` | conflict `track already liked` |
| 23505 на любом другом ограничении | conflict `email already exists` (KI-5) |
| 23503 | not_found `not found` (KI-1) |
| 22P02 (некорректный UUID) | invalid `invalid id` |
| всё остальное | internal, текст называет операцию, причина сохраняется |

Модульные кейсы (`stub`, `pgxmock`) проверяют аргументы SQL, чтение строки и
этот перевод на заготовленных строках и ошибках. Интеграционные кейсы проверяют
то же поведение и сохранённые строки на настоящей базе.

#### UserRepository.Create

- Вставляет email, имя и хеш; возвращает пользователя со сгенерированным UUID.
- Повторный email даёт conflict `email already exists`. База сравнивает email
  с учётом регистра; нижний регистр делает домен.

#### UserRepository.GetByID

- Нет строки — not_found `user not found`; некорректный идентификатор — invalid
  `invalid id`.

#### UserRepository.GetByEmail

- Возвращает пользователя и хеш пароля для входа. Нет строки — not_found
  `user not found`. Совпадение точное.

#### UserRepository.List

- Порядок по `created_at`. Нет пользователей — пустой ненулевой срез.

#### UserRepository.Update

- Обновляет email и имя. Нет строки — not_found `user not found`; email другого
  пользователя — conflict `email already exists`.

#### UserRepository.Delete

- Ноль затронутых строк — not_found `user not found`. Пользователь, у которого
  есть треки, — not_found `not found` (KI-1).

#### TrackRepository.Create

- В одном операторе генерирует идентификатор и ключ объекта `<id>.mp3`.
- Неизвестный владелец: not_found `not found` (внешний ключ).

#### TrackRepository.GetByID

- Нет строки — not_found `track not found`.

#### TrackRepository.List

- Фильтры необязательны и соединяются через AND: статус, владелец, исполнитель.
  Порядок по `created_at`.
- Исполнитель сравнивается как `lower(btrim(artist)) = lower(btrim($3))`. Это
  связанный параметр, поэтому кавычки, `%`, `_` и SQL-текст — обычные символы.

#### TrackRepository.Update

- Записывает все изменяемые колонки (владелец, название, исполнитель, ключ
  объекта, размер, статус) из переданного трека. Пустой ключ объекта
  сохраняется как `<id>.mp3`. Нет строки — not_found `track not found`.

#### TrackRepository.Delete

- Ноль затронутых строк — not_found `track not found`. Лайки трека удаляются
  через `ON DELETE CASCADE`.

#### TrackLikeRepository.Add

- Первичный ключ `(user_id, track_id)`: повтор даёт conflict
  `track already liked`. Неизвестный трек или пользователь — not_found
  `not found`; некорректный идентификатор — invalid `invalid id`.

#### TrackLikeRepository.Remove

- Завершается успехом, когда снимать нечего.

#### TrackLikeRepository.ListReadyByUser

- Соединяет с треками, оставляет только `ready`, сначала самый новый лайк.

### Сквозной сценарий

#### DemoScenario.MVP

Один кейс `E2E-DEMO-01`. Клиент — HTTP, без браузера: те же запросы, что
фронтенд собирает в `frontend/src/api/client.ts`. Артист и слушатель новые на
каждый прогон. Аудио — несколько КиБ байт. Пока трек `pending`, публичная лента
его не показывает. Подписанный URL считается для публичного хоста
(`S3_PRESIGN_ENDPOINT`); само соединение API с хранилищем идёт на внутренний
адрес, иначе `PUT` получает 403 `SignatureDoesNotMatch`.

| Шаг | Запрос | Статус | Что проверяется |
| --- | --- | --- | --- |
| 01 | `GET /ready` | 200 | API готов |
| 02 | `POST /v1/auth/register` артист | 201 | в ответе есть `token` |
| 03 | `POST /v1/auth/login` | 200 | новый `token` |
| 04 | `GET /v1/me` | 200 | email артиста |
| 05 | `POST /v1/tracks/upload-init` | 201 | статус `pending`, есть `upload_url` |
| 06 | `PUT upload_url` | 200 | байты лежат в RustFS |
| 07 | `GET /v1/tracks?artist=` | 200 | трека в ленте нет |
| 08 | `POST /v1/tracks/{id}/upload-complete` | 200 | статус `ready` |
| 09 | `GET /v1/tracks?artist=` | 200 | трек в ленте, статус `ready` |
| 10 | `POST /v1/auth/register` слушатель | 201 | отдельный `token` |
| 11 | `POST /v1/tracks/{id}/like` без токена | 401 | код `unauthorized` |
| 12 | `POST /v1/tracks/{id}/like` | 204 | лайк записан |
| 13 | повтор шага 12 | 409 | `conflict`, текст `track already liked` |
| 14 | `GET /v1/me/likes` | 200 | трек в списке |
| 15 | `GET /v1/tracks/{id}/stream` | 302 | заголовок `Location` |
| 16 | `GET Location` | 200 | байты совпали с загруженными |
| 17 | `DELETE /v1/tracks/{id}/like` | 204 | лайк снят |
| 18 | `GET /v1/me/likes` | 200 | трека в списке нет |

Тот же порядок повторяет `scripts/demo-requests.sh`. Захват этого прогона —
`make demo-capture`.

## <a id="part-2"></a>Часть 2. Границы, классы эквивалентности и инварианты

### Входы

| Вход | Допустимые классы | Недопустимые классы | Проверенные границы |
| --- | --- | --- | --- |
| Email | содержит `@`, любой регистр, пробелы по краям | пустой, одни пробелы, без `@` | `""`, `"   "`, `"a"`, `" A@B.C "` |
| Пароль при регистрации или создании | 8 байт и больше | меньше 8 байт | 7, 8 |
| Пароль при входе | 1 байт и больше | пустой | 0, 1 |
| Лимит страницы | 1..100; 0 и меньше становится 20 | больше 100 | -1, 0, 1, 100, 101 |
| Курсор | пустой, целое 0 и больше | отрицательный, не число | `""`, `"0"`, `"-1"`, `"abc"`, за концом списка |
| Размер загрузки | 1..`TRACK_MAX_BYTES` | 0 и меньше, больше максимума | 0, -1, 1, max, max+1; max 0 и меньше означает, что предела нет |
| Размер загруженного объекта | равен заявленному | 0 и меньше, другой | 0, отрицательный, заявленный, другой |
| Статус трека для лайка, стрима, ленты | `ready` | `pending`, `processing` | каждый статус |
| Фильтр по исполнителю | точный текст, любой регистр, пробелы по краям | — | `O'Brien`, `100%`, `A_B`, кириллица, SQL-текст |
| Идентификаторы | существующий UUID | пустой, некорректный, неизвестный UUID | `""`, `not-a-uuid`, нулевой UUID |

### Предусловия сценариев

- Анонимный вызывающий и держатель токена: покрыто только на уровне HTTP,
  дымовой проверкой в CI.
- Владелец трека и другой пользователь (`CompleteUpload`).
- Статус трека до вызова: `pending`, `processing`, `ready`.
- Состояние лайка до вызова: не лайкнут, лайкнут.
- Состояние базы: пустая таблица, одна строка, конфликтующая строка, строка,
  на которую ссылается внешний ключ.

### Инварианты

- I-1. Статус движется только `pending -> processing -> ready`. Повторный
  `MarkProcessing` на `processing` разрешён; любой другой переход отклоняется
  и оставляет статус прежним.
- I-2. Лайкать, стримить и показывать в публичной ленте и в списке лайков можно
  только треки `ready`.
- I-3. Email хранится в нижнем регистре и не больше одного раза.
- I-4. Пароли хранятся только как хеши bcrypt и никогда не возвращаются в
  пользователе.
- I-5. Трек публикуется только когда сохранённый объект существует и его размер
  равен заявленному.
- I-6. Неудачный `InitUpload` не оставляет строку `pending`, если только само
  удаление при откате тоже не упало (KI-8).
- I-7. Пользователь может лайкнуть трек не больше одного раза; удаление трека
  удаляет его лайки.
- I-8. Вход не сообщает, зарегистрирован ли email.

## <a id="part-3"></a>Часть 3. Матрица трассируемости

Собрана из результатов Allure командой `make spec-matrix`. Каждая строка —
один кейс Allure. Заголовки колонок и значения в них английские: это поля
отчёта (слой, компонент, метод, заголовок, стиль, техника, severity), и
`make spec-check` сверяет блок с этими полями как есть.

<!-- matrix:start -->

| ID | Layer | Component.Method | Title | Style | Technique | Severity |
| --- | --- | --- | --- | --- | --- | --- |
| `DOM-ERR-AS-01` | Domain | `Error.As` | extracts domain error through error chain | classic | equivalence partitioning | normal |
| `DOM-ERR-AS-02` | Domain | `Error.As` | returns false for ordinary error | classic | equivalence partitioning | normal |
| `DOM-ERR-AS-03` | Domain | `Error.As` | returns false for nil error | classic | error guessing | normal |
| `DOM-ERR-CONFLICT-01` | Domain | `Error.Conflict` | uses conflict code | classic | equivalence partitioning | normal |
| `DOM-ERR-CONFLICT-02` | Domain | `Error.Conflict` | does not attach a cause to conflict error | classic | equivalence partitioning | normal |
| `DOM-ERR-STR-01` | Domain | `Error.Error` | formats code and message without cause | classic | equivalence partitioning | normal |
| `DOM-ERR-STR-02` | Domain | `Error.Error` | includes wrapped cause | classic | equivalence partitioning | normal |
| `DOM-ERR-INTERNAL-01` | Domain | `Error.Internal` | uses internal code | classic | equivalence partitioning | normal |
| `DOM-ERR-INTERNAL-02` | Domain | `Error.Internal` | does not attach a cause to internal error | classic | equivalence partitioning | normal |
| `DOM-ERR-INVALID-01` | Domain | `Error.Invalid` | uses invalid code | classic | equivalence partitioning | normal |
| `DOM-ERR-INVALID-02` | Domain | `Error.Invalid` | does not attach a cause to invalid error | classic | equivalence partitioning | normal |
| `DOM-ERR-IS-01` | Domain | `Error.Is` | matches code through error chain | classic | equivalence partitioning | normal |
| `DOM-ERR-IS-02` | Domain | `Error.Is` | rejects different code | classic | equivalence partitioning | normal |
| `DOM-ERR-IS-03` | Domain | `Error.Is` | rejects ordinary error | classic | equivalence partitioning | normal |
| `DOM-ERR-IS-04` | Domain | `Error.Is` | rejects nil error | classic | error guessing | normal |
| `DOM-ERR-NEW-01` | Domain | `Error.NewError` | constructs error with code and message | classic | equivalence partitioning | normal |
| `DOM-ERR-NEW-02` | Domain | `Error.NewError` | preserves empty message | classic | boundary-value analysis | normal |
| `DOM-ERR-NOTFOUND-01` | Domain | `Error.NotFound` | uses not_found code | classic | equivalence partitioning | normal |
| `DOM-ERR-NOTFOUND-02` | Domain | `Error.NotFound` | does not attach a cause to not_found error | classic | equivalence partitioning | normal |
| `DOM-ERR-NOTFOUND-03` | Domain | `Error.NotFound` | creates independent errors for different messages | classic | error guessing | normal |
| `DOM-ERR-UNAUTH-01` | Domain | `Error.Unauthorized` | uses unauthorized code | classic | equivalence partitioning | normal |
| `DOM-ERR-UNAUTH-02` | Domain | `Error.Unauthorized` | does not attach a cause to unauthorized error | classic | equivalence partitioning | normal |
| `DOM-ERR-UNAUTH-03` | Domain | `Error.Unauthorized` | preserves empty unauthorized message | classic | boundary-value analysis | normal |
| `DOM-ERR-UNWRAP-01` | Domain | `Error.Unwrap` | returns wrapped cause | classic | equivalence partitioning | normal |
| `DOM-ERR-UNWRAP-02` | Domain | `Error.Unwrap` | returns nil when error has no cause | classic | equivalence partitioning | normal |
| `DOM-ERR-WRAP-01` | Domain | `Error.Wrap` | constructs error retaining cause | classic | equivalence partitioning | normal |
| `DOM-ERR-WRAP-02` | Domain | `Error.Wrap` | accepts nil cause | classic | error guessing | normal |
| `DOM-AUTH-LOGIN-01` | Domain | `LoginInput.Validate` | accepts credentials and normalizes email | classic | equivalence partitioning | critical |
| `DOM-AUTH-LOGIN-02` | Domain | `LoginInput.Validate` | accepts a one-character password | classic | boundary-value analysis | normal |
| `DOM-AUTH-LOGIN-03` | Domain | `LoginInput.Validate` | rejects blank email after normalization | classic | boundary-value analysis | normal |
| `DOM-AUTH-LOGIN-04` | Domain | `LoginInput.Validate` | rejects empty password | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-01` | Domain | `PageQuery.Page` | returns first page and next cursor | classic | equivalence partitioning | normal |
| `DOM-PAGE-PAGE-02` | Domain | `PageQuery.Page` | returns middle slice and next cursor | classic | equivalence partitioning | normal |
| `DOM-PAGE-PAGE-03` | Domain | `PageQuery.Page` | omits next cursor on exactly last page | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-04` | Domain | `PageQuery.Page` | omits next cursor when limit equals collection size | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-05` | Domain | `PageQuery.Page` | returns empty page when cursor equals collection size | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-06` | Domain | `PageQuery.Page` | returns empty page when cursor exceeds collection | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-07` | Domain | `PageQuery.Page` | returns empty page for empty collection | classic | boundary-value analysis | normal |
| `DOM-PAGE-PAGE-08` | Domain | `PageQuery.Page` | unvalidated zero limit yields empty page with cursor 0 | classic | error guessing | normal |
| `DOM-PAGE-VAL-01` | Domain | `PageQuery.Validate` | defaults negative limit to 20 | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-02` | Domain | `PageQuery.Validate` | defaults zero limit to 20 | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-03` | Domain | `PageQuery.Validate` | accepts minimum limit 1 | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-04` | Domain | `PageQuery.Validate` | accepts maximum limit 100 | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-05` | Domain | `PageQuery.Validate` | rejects limit 101 above maximum | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-06` | Domain | `PageQuery.Validate` | accepts empty cursor as first page | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-07` | Domain | `PageQuery.Validate` | accepts zero cursor | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-08` | Domain | `PageQuery.Validate` | accepts positive cursor | classic | equivalence partitioning | normal |
| `DOM-PAGE-VAL-09` | Domain | `PageQuery.Validate` | rejects negative cursor | classic | boundary-value analysis | normal |
| `DOM-PAGE-VAL-10` | Domain | `PageQuery.Validate` | rejects non-numeric cursor | classic | error guessing | normal |
| `DOM-PAGE-VAL-11` | Domain | `PageQuery.Validate` | rejects cursor with surrounding spaces | classic | error guessing | normal |
| `DOM-PAGE-VAL-12` | Domain | `PageQuery.Validate` | checks limit before cursor | classic | decision-table testing | normal |
| `DOM-AUTH-USER-01` | Domain | `RegisterInput.User` | maps email and name | classic | equivalence partitioning | normal |
| `DOM-AUTH-USER-02` | Domain | `RegisterInput.User` | does not populate an id or leak the password | classic | error guessing | critical |
| `DOM-AUTH-REG-01` | Domain | `RegisterInput.Validate` | accepts valid input and normalizes email | classic | equivalence partitioning | critical |
| `DOM-AUTH-REG-02` | Domain | `RegisterInput.Validate` | accepts password at minimum length 8 | classic | boundary-value analysis | normal |
| `DOM-AUTH-REG-03` | Domain | `RegisterInput.Validate` | rejects password of length 7 | classic | boundary-value analysis | normal |
| `DOM-AUTH-REG-04` | Domain | `RegisterInput.Validate` | rejects empty password | classic | boundary-value analysis | normal |
| `DOM-AUTH-REG-05` | Domain | `RegisterInput.Validate` | rejects email without at sign | classic | equivalence partitioning | normal |
| `DOM-AUTH-REG-06` | Domain | `RegisterInput.Validate` | rejects blank email | classic | boundary-value analysis | normal |
| `DOM-AUTH-REG-07` | Domain | `RegisterInput.Validate` | rejects invalid email before checking password | classic | decision-table testing | normal |
| `DOM-TRK-KEY-01` | Domain | `Track.AudioObjectKey` | returns stored object key | classic | equivalence partitioning | normal |
| `DOM-TRK-KEY-02` | Domain | `Track.AudioObjectKey` | derives object key when stored key is empty | classic | boundary-value analysis | normal |
| `DOM-TRK-CONFIRM-01` | Domain | `Track.ConfirmUpload` | accepts matching expected size | classic | equivalence partitioning | normal |
| `DOM-TRK-CONFIRM-02` | Domain | `Track.ConfirmUpload` | accepts any positive size when expected size is absent | classic | boundary-value analysis | normal |
| `DOM-TRK-CONFIRM-03` | Domain | `Track.ConfirmUpload` | treats negative expected size as absent | classic | error guessing | normal |
| `DOM-TRK-CONFIRM-04` | Domain | `Track.ConfirmUpload` | rejects zero uploaded size | classic | boundary-value analysis | normal |
| `DOM-TRK-CONFIRM-05` | Domain | `Track.ConfirmUpload` | rejects negative uploaded size | classic | boundary-value analysis | normal |
| `DOM-TRK-CONFIRM-06` | Domain | `Track.ConfirmUpload` | rejects size one byte below expected | classic | boundary-value analysis | normal |
| `DOM-TRK-CONFIRM-07` | Domain | `Track.ConfirmUpload` | rejects size one byte above expected | classic | boundary-value analysis | normal |
| `DOM-TRK-ENSURE-01` | Domain | `Track.EnsureReady` | accepts ready track | classic | state-transition testing | critical |
| `DOM-TRK-ENSURE-02` | Domain | `Track.EnsureReady` | rejects pending track | classic | state-transition testing | critical |
| `DOM-TRK-ENSURE-03` | Domain | `Track.EnsureReady` | rejects processing track | classic | state-transition testing | critical |
| `DOM-TRK-ENSURE-04` | Domain | `Track.EnsureReady` | rejects unknown status | classic | error guessing | critical |
| `DOM-TRK-PROC-01` | Domain | `Track.MarkProcessing` | transitions pending track to processing | classic | state-transition testing | critical |
| `DOM-TRK-PROC-02` | Domain | `Track.MarkProcessing` | is idempotent for processing track | classic | state-transition testing | critical |
| `DOM-TRK-PROC-03` | Domain | `Track.MarkProcessing` | rejects ready track without changing state | classic | state-transition testing | critical |
| `DOM-TRK-PROC-04` | Domain | `Track.MarkProcessing` | rejects unknown status without changing state | classic | error guessing | critical |
| `DOM-TRK-READY-01` | Domain | `Track.MarkReady` | transitions processing track to ready | classic | state-transition testing | critical |
| `DOM-TRK-READY-02` | Domain | `Track.MarkReady` | rejects pending track without changing state | classic | state-transition testing | critical |
| `DOM-TRK-READY-03` | Domain | `Track.MarkReady` | rejects repeated ready transition | classic | state-transition testing | critical |
| `DOM-TRK-READY-04` | Domain | `Track.MarkReady` | rejects empty status without changing state | classic | error guessing | critical |
| `DOM-TRK-OWN-01` | Domain | `Track.OwnedBy` | accepts owner | classic | equivalence partitioning | critical |
| `DOM-TRK-OWN-02` | Domain | `Track.OwnedBy` | rejects another user | classic | equivalence partitioning | critical |
| `DOM-TRK-OWN-03` | Domain | `Track.OwnedBy` | rejects empty caller | classic | error guessing | critical |
| `DOM-TRK-OWN-04` | Domain | `Track.OwnedBy` | accepts empty caller for ownerless track | classic | error guessing | critical |
| `DOM-TRK-FEEDFLT-01` | Domain | `TrackFeedQuery.Filter` | builds ready artist filter | classic | equivalence partitioning | normal |
| `DOM-TRK-FEEDFLT-02` | Domain | `TrackFeedQuery.Filter` | does not scope feed to a user | classic | equivalence partitioning | normal |
| `DOM-TRK-FEED-01` | Domain | `TrackFeedQuery.Validate` | trims artist and validates page | classic | equivalence partitioning | normal |
| `DOM-TRK-FEED-02` | Domain | `TrackFeedQuery.Validate` | accepts blank artist as unfiltered query | classic | boundary-value analysis | normal |
| `DOM-TRK-FEED-03` | Domain | `TrackFeedQuery.Validate` | rejects invalid page limit | classic | boundary-value analysis | normal |
| `DOM-TRK-ID-01` | Domain | `TrackID.Validate` | accepts non-empty id | classic | equivalence partitioning | normal |
| `DOM-TRK-ID-02` | Domain | `TrackID.Validate` | accepts whitespace because it is non-empty | classic | error guessing | normal |
| `DOM-TRK-ID-03` | Domain | `TrackID.Validate` | rejects empty id | classic | boundary-value analysis | normal |
| `DOM-TRK-LIKE-01` | Domain | `TrackLike.Validate` | accepts user and track ids | classic | equivalence partitioning | normal |
| `DOM-TRK-LIKE-02` | Domain | `TrackLike.Validate` | rejects empty user id before track id | classic | decision-table testing | normal |
| `DOM-TRK-LIKE-03` | Domain | `TrackLike.Validate` | rejects empty track id | classic | equivalence partitioning | normal |
| `DOM-TRK-LIKELIST-01` | Domain | `TrackLikeListQuery.Validate` | accepts user and valid page | classic | equivalence partitioning | normal |
| `DOM-TRK-LIKELIST-02` | Domain | `TrackLikeListQuery.Validate` | rejects empty user before page validation | classic | decision-table testing | normal |
| `DOM-TRK-LIKELIST-03` | Domain | `TrackLikeListQuery.Validate` | rejects invalid page for valid user | classic | equivalence partitioning | normal |
| `DOM-TRK-OWNERFLT-01` | Domain | `TrackOwnerQuery.Filter` | builds owner and status filter | classic | equivalence partitioning | normal |
| `DOM-TRK-OWNERFLT-02` | Domain | `TrackOwnerQuery.Filter` | does not populate artist filter | classic | equivalence partitioning | normal |
| `DOM-TRK-OWNERQ-01` | Domain | `TrackOwnerQuery.Validate` | accepts owner and valid page | classic | equivalence partitioning | normal |
| `DOM-TRK-OWNERQ-02` | Domain | `TrackOwnerQuery.Validate` | rejects empty owner before page validation | classic | decision-table testing | normal |
| `DOM-TRK-OWNERQ-03` | Domain | `TrackOwnerQuery.Validate` | rejects invalid page for valid owner | classic | equivalence partitioning | normal |
| `DOM-TRK-COMPLETE-01` | Domain | `TrackUploadComplete.Validate` | accepts track and user ids | classic | equivalence partitioning | normal |
| `DOM-TRK-COMPLETE-02` | Domain | `TrackUploadComplete.Validate` | rejects empty track id | classic | equivalence partitioning | normal |
| `DOM-TRK-COMPLETE-03` | Domain | `TrackUploadComplete.Validate` | rejects empty user id | classic | equivalence partitioning | normal |
| `DOM-TRK-COMPLETE-04` | Domain | `TrackUploadComplete.Validate` | checks track id before user id | classic | decision-table testing | normal |
| `DOM-TRK-TOTRACK-01` | Domain | `TrackUploadInit.Track` | maps upload input to pending track | classic | equivalence partitioning | normal |
| `DOM-TRK-TOTRACK-02` | Domain | `TrackUploadInit.Track` | leaves persistence-assigned fields empty | classic | equivalence partitioning | normal |
| `DOM-TRK-INIT-01` | Domain | `TrackUploadInit.Validate` | accepts complete upload input | classic | equivalence partitioning | normal |
| `DOM-TRK-INIT-02` | Domain | `TrackUploadInit.Validate` | accepts one byte size | classic | boundary-value analysis | normal |
| `DOM-TRK-INIT-03` | Domain | `TrackUploadInit.Validate` | rejects zero size | classic | boundary-value analysis | normal |
| `DOM-TRK-INIT-04` | Domain | `TrackUploadInit.Validate` | rejects negative size | classic | boundary-value analysis | normal |
| `DOM-TRK-INIT-05` | Domain | `TrackUploadInit.Validate` | rejects empty user id | classic | equivalence partitioning | normal |
| `DOM-TRK-INIT-06` | Domain | `TrackUploadInit.Validate` | rejects empty title | classic | equivalence partitioning | normal |
| `DOM-TRK-INIT-07` | Domain | `TrackUploadInit.Validate` | checks user id before title and size | classic | decision-table testing | normal |
| `DOM-TRK-INIT-08` | Domain | `TrackUploadInit.Validate` | checks title before size | classic | decision-table testing | normal |
| `DOM-TRK-SIZE-01` | Domain | `TrackUploadInit.ValidateSize` | accepts size equal to maximum | classic | boundary-value analysis | normal |
| `DOM-TRK-SIZE-02` | Domain | `TrackUploadInit.ValidateSize` | treats zero maximum as unlimited | classic | boundary-value analysis | normal |
| `DOM-TRK-SIZE-03` | Domain | `TrackUploadInit.ValidateSize` | treats negative maximum as unlimited | classic | boundary-value analysis | normal |
| `DOM-TRK-SIZE-04` | Domain | `TrackUploadInit.ValidateSize` | rejects size one byte above maximum | classic | boundary-value analysis | normal |
| `DOM-TRK-WRITE-01` | Domain | `TrackWrite.Validate` | accepts title without artist | classic | equivalence partitioning | normal |
| `DOM-TRK-WRITE-02` | Domain | `TrackWrite.Validate` | accepts whitespace title because it is non-empty | classic | error guessing | normal |
| `DOM-TRK-WRITE-03` | Domain | `TrackWrite.Validate` | rejects empty title | classic | boundary-value analysis | normal |
| `DOM-USER-ID-01` | Domain | `UserID.Validate` | accepts non-empty id | classic | equivalence partitioning | normal |
| `DOM-USER-ID-02` | Domain | `UserID.Validate` | rejects empty id | classic | boundary-value analysis | normal |
| `DOM-USER-MAP-01` | Domain | `UserWrite.User` | maps write fields and supplied id | classic | equivalence partitioning | normal |
| `DOM-USER-MAP-02` | Domain | `UserWrite.User` | allows an empty supplied id without leaking password | classic | error guessing | critical |
| `DOM-USER-VAL-01` | Domain | `UserWrite.Validate` | accepts valid write and normalizes email | classic | equivalence partitioning | critical |
| `DOM-USER-VAL-02` | Domain | `UserWrite.Validate` | normalizes shortest padded email | classic | boundary-value analysis | critical |
| `DOM-USER-VAL-03` | Domain | `UserWrite.Validate` | accepts bare at sign as email | classic | error guessing | critical |
| `DOM-USER-VAL-04` | Domain | `UserWrite.Validate` | does not require password for update | classic | equivalence partitioning | critical |
| `DOM-USER-VAL-05` | Domain | `UserWrite.Validate` | rejects email without at sign | classic | equivalence partitioning | critical |
| `DOM-USER-VAL-06` | Domain | `UserWrite.Validate` | rejects empty email | classic | boundary-value analysis | critical |
| `DOM-USER-VAL-07` | Domain | `UserWrite.Validate` | rejects single-character email | classic | boundary-value analysis | critical |
| `DOM-USER-VAL-08` | Domain | `UserWrite.Validate` | rejects whitespace-only email | classic | error guessing | critical |
| `DOM-USER-CREATE-01` | Domain | `UserWrite.ValidateCreate` | accepts password at minimum length 8 | classic | boundary-value analysis | critical |
| `DOM-USER-CREATE-02` | Domain | `UserWrite.ValidateCreate` | rejects password of length 7 | classic | boundary-value analysis | critical |
| `DOM-USER-CREATE-03` | Domain | `UserWrite.ValidateCreate` | rejects empty password | classic | boundary-value analysis | critical |
| `DOM-USER-CREATE-04` | Domain | `UserWrite.ValidateCreate` | normalizes valid email | classic | equivalence partitioning | critical |
| `DOM-USER-CREATE-05` | Domain | `UserWrite.ValidateCreate` | rejects invalid email before password | classic | decision-table testing | critical |
| `IT-UC-AUTH-LOGIN-01` | Business logic | `AuthService.Login` | wrong password returns unauthorized | integration | error guessing | blocker |
| `IT-UC-AUTH-LOGIN-02` | Business logic | `AuthService.Login` | unknown email returns the same unauthorized error | integration | error guessing | critical |
| `IT-UC-AUTH-LOGIN-03` | Business logic | `AuthService.Login` | login normalizes the email before lookup | integration | equivalence partitioning | normal |
| `UC-AUTH-LOGIN-01` | Business logic | `AuthService.Login` | authenticates normalized email and issues token | classic | equivalence partitioning | critical |
| `UC-AUTH-LOGIN-02` | Business logic | `AuthService.Login` | rejects empty password before user lookup | classic | boundary-value analysis | critical |
| `UC-AUTH-LOGIN-03` | Business logic | `AuthService.Login` | rejects whitespace-only email before user lookup | classic | boundary-value analysis | critical |
| `UC-AUTH-LOGIN-04` | Business logic | `AuthService.Login` | maps unknown email to invalid credentials | classic | equivalence partitioning | critical |
| `UC-AUTH-LOGIN-05` | Business logic | `AuthService.Login` | returns repository lookup error unchanged | classic | error guessing | critical |
| `UC-AUTH-LOGIN-06` | Business logic | `AuthService.Login` | returns hasher mismatch error for wrong password | classic | equivalence partitioning | critical |
| `UC-AUTH-LOGIN-07` | Business logic | `AuthService.Login` | returns password comparison failure without token | classic | error guessing | critical |
| `UC-AUTH-LOGIN-08` | Business logic | `AuthService.Login` | returns token issuer error | classic | error guessing | critical |
| `IT-UC-AUTH-REG-01` | Business logic | `AuthService.Register` | register stores a bcrypt hash and login issues a token | integration | equivalence partitioning | blocker |
| `IT-UC-AUTH-REG-02` | Business logic | `AuthService.Register` | duplicate registration returns conflict | integration | error guessing | critical |
| `IT-UC-AUTH-REG-03` | Business logic | `AuthService.Register` | short password is rejected before anything is stored | integration | boundary-value analysis | normal |
| `UC-AUTH-REG-01` | Business logic | `AuthService.Register` | registers user with normalized email and issues token | classic | equivalence partitioning | critical |
| `UC-AUTH-REG-02` | Business logic | `AuthService.Register` | rejects invalid email before hashing or storing | classic | decision-table testing | critical |
| `UC-AUTH-REG-03` | Business logic | `AuthService.Register` | rejects password of length 7 before hashing | classic | boundary-value analysis | critical |
| `UC-AUTH-REG-04` | Business logic | `AuthService.Register` | returns hashing error without storing user | classic | error guessing | critical |
| `UC-AUTH-REG-05` | Business logic | `AuthService.Register` | returns repository create error without token | classic | error guessing | critical |
| `UC-AUTH-REG-06` | Business logic | `AuthService.Register` | returns token error and keeps the created user | classic | error guessing | critical |
| `IT-UC-TRK-COMPLETE-01` | Business logic | `TrackService.CompleteUpload` | complete upload moves pending to ready | integration | state-transition testing | blocker |
| `IT-UC-TRK-COMPLETE-02` | Business logic | `TrackService.CompleteUpload` | complete upload by another user returns unauthorized | integration | decision-table testing | blocker |
| `IT-UC-TRK-COMPLETE-03` | Business logic | `TrackService.CompleteUpload` | uploaded size mismatch keeps the track pending | integration | boundary-value analysis | critical |
| `IT-UC-TRK-COMPLETE-04` | Business logic | `TrackService.CompleteUpload` | complete upload of a ready track returns conflict | integration | state-transition testing | normal |
| `UC-TRK-COMPLETE-01` | Business logic | `TrackService.CompleteUpload` | publishes pending track after verifying upload | classic | state-transition testing | critical |
| `UC-TRK-COMPLETE-02` | Business logic | `TrackService.CompleteUpload` | resumes processing track to ready | classic | state-transition testing | critical |
| `UC-TRK-COMPLETE-03` | Business logic | `TrackService.CompleteUpload` | reports missing track id first for empty input | classic | decision-table testing | normal |
| `UC-TRK-COMPLETE-04` | Business logic | `TrackService.CompleteUpload` | rejects missing user id | classic | equivalence partitioning | normal |
| `UC-TRK-COMPLETE-05` | Business logic | `TrackService.CompleteUpload` | returns track lookup error | classic | error guessing | normal |
| `UC-TRK-COMPLETE-06` | Business logic | `TrackService.CompleteUpload` | propagates not found for unknown track | classic | equivalence partitioning | normal |
| `UC-TRK-COMPLETE-07` | Business logic | `TrackService.CompleteUpload` | rejects another user's track before checking storage | classic | decision-table testing | critical |
| `UC-TRK-COMPLETE-08` | Business logic | `TrackService.CompleteUpload` | returns initial object stat error | classic | error guessing | normal |
| `UC-TRK-COMPLETE-09` | Business logic | `TrackService.CompleteUpload` | rejects missing uploaded object | classic | boundary-value analysis | normal |
| `UC-TRK-COMPLETE-10` | Business logic | `TrackService.CompleteUpload` | rejects uploaded size one byte larger than declared | classic | boundary-value analysis | normal |
| `UC-TRK-COMPLETE-11` | Business logic | `TrackService.CompleteUpload` | rejects completing an already ready track | classic | state-transition testing | normal |
| `UC-TRK-COMPLETE-12` | Business logic | `TrackService.CompleteUpload` | returns processing update error and keeps track pending | classic | error guessing | critical |
| `UC-TRK-COMPLETE-13` | Business logic | `TrackService.CompleteUpload` | returns publish stat error and leaves track processing | classic | error guessing | critical |
| `UC-TRK-COMPLETE-14` | Business logic | `TrackService.CompleteUpload` | returns ready update error and leaves track processing | classic | error guessing | critical |
| `IT-UC-TRK-DELETE-01` | Business logic | `TrackService.Delete` | delete removes the track and its likes | integration | equivalence partitioning | normal |
| `UC-TRK-DELETE-01` | Business logic | `TrackService.Delete` | deletes stored track | classic | equivalence partitioning | normal |
| `UC-TRK-DELETE-02` | Business logic | `TrackService.Delete` | rejects empty id without deleting | classic | boundary-value analysis | normal |
| `UC-TRK-DELETE-03` | Business logic | `TrackService.Delete` | propagates not found for unknown track | classic | equivalence partitioning | normal |
| `UC-TRK-DELETE-04` | Business logic | `TrackService.Delete` | returns repository delete error and keeps track | classic | error guessing | normal |
| `IT-UC-TRK-GET-01` | Business logic | `TrackService.GetByID` | get of an unknown track returns not found | integration | error guessing | normal |
| `UC-TRK-GET-01` | Business logic | `TrackService.GetByID` | returns track by id | classic | equivalence partitioning | normal |
| `UC-TRK-GET-02` | Business logic | `TrackService.GetByID` | rejects empty id before lookup | classic | boundary-value analysis | normal |
| `UC-TRK-GET-03` | Business logic | `TrackService.GetByID` | propagates not found for unknown id | classic | equivalence partitioning | normal |
| `UC-TRK-GET-04` | Business logic | `TrackService.GetByID` | returns repository lookup error | classic | error guessing | normal |
| `IT-UC-TRK-INIT-01` | Business logic | `TrackService.InitUpload` | init upload with size 1 of max 1024 | integration | boundary-value analysis | critical |
| `IT-UC-TRK-INIT-02` | Business logic | `TrackService.InitUpload` | init upload with size 1024 of max 1024 | integration | boundary-value analysis | critical |
| `IT-UC-TRK-INIT-03` | Business logic | `TrackService.InitUpload` | init upload with size 1025 of max 1024 | integration | boundary-value analysis | critical |
| `IT-UC-TRK-INIT-04` | Business logic | `TrackService.InitUpload` | presign failure rolls back the created track | integration | error guessing | critical |
| `UC-TRK-INIT-01` | Business logic | `TrackService.InitUpload` | creates pending track and presigns upload | classic | equivalence partitioning | critical |
| `UC-TRK-INIT-02` | Business logic | `TrackService.InitUpload` | accepts size exactly at max | classic | boundary-value analysis | critical |
| `UC-TRK-INIT-03` | Business logic | `TrackService.InitUpload` | accepts minimum size of 1 byte | classic | boundary-value analysis | critical |
| `UC-TRK-INIT-04` | Business logic | `TrackService.InitUpload` | rejects size max+1 without creating track | classic | boundary-value analysis | critical |
| `UC-TRK-INIT-05` | Business logic | `TrackService.InitUpload` | rejects zero size without creating track | classic | boundary-value analysis | critical |
| `UC-TRK-INIT-06` | Business logic | `TrackService.InitUpload` | reports missing user id first for empty input | classic | decision-table testing | critical |
| `UC-TRK-INIT-07` | Business logic | `TrackService.InitUpload` | reports missing title before oversize | classic | decision-table testing | critical |
| `UC-TRK-INIT-08` | Business logic | `TrackService.InitUpload` | returns create error without session | classic | error guessing | critical |
| `UC-TRK-INIT-09` | Business logic | `TrackService.InitUpload` | deletes created track when presigning fails | classic | error guessing | critical |
| `UC-TRK-INIT-10` | Business logic | `TrackService.InitUpload` | returns rollback error and drops presign error when cleanup fails | classic | error guessing | critical |
| `IT-UC-TRK-LIKE-01` | Business logic | `TrackService.Like` | like of a ready track is stored | integration | state-transition testing | normal |
| `IT-UC-TRK-LIKE-02` | Business logic | `TrackService.Like` | like of a pending track returns conflict | integration | state-transition testing | critical |
| `IT-UC-TRK-LIKE-03` | Business logic | `TrackService.Like` | like of a processing track returns conflict | integration | state-transition testing | critical |
| `IT-UC-TRK-LIKE-04` | Business logic | `TrackService.Like` | second like of the same track returns conflict | integration | error guessing | normal |
| `IT-UC-TRK-LIKE-05` | Business logic | `TrackService.Like` | like of an unknown track returns not found | integration | error guessing | normal |
| `UC-TRK-LIKE-01` | Business logic | `TrackService.Like` | stores like for ready track | classic | state-transition testing | normal |
| `UC-TRK-LIKE-02` | Business logic | `TrackService.Like` | rejects pending track without storing like | classic | state-transition testing | normal |
| `UC-TRK-LIKE-03` | Business logic | `TrackService.Like` | rejects processing track without storing like | classic | state-transition testing | normal |
| `UC-TRK-LIKE-04` | Business logic | `TrackService.Like` | reports missing user id before track lookup | classic | decision-table testing | normal |
| `UC-TRK-LIKE-05` | Business logic | `TrackService.Like` | propagates not found for unknown track without storing like | classic | equivalence partitioning | normal |
| `UC-TRK-LIKE-06` | Business logic | `TrackService.Like` | returns track lookup error without storing like | classic | error guessing | normal |
| `UC-TRK-LIKE-07` | Business logic | `TrackService.Like` | returns add like error | classic | error guessing | normal |
| `UC-TRK-LIKE-08` | Business logic | `TrackService.Like` | looks up exact track then adds exact like once | london | equivalence partitioning | normal |
| `UC-TRK-LIKE-09` | Business logic | `TrackService.Like` | calls no collaborator for invalid input | london | equivalence partitioning | normal |
| `UC-TRK-LIKE-10` | Business logic | `TrackService.Like` | does not call add when lookup fails | london | error guessing | normal |
| `UC-TRK-LIKE-11` | Business logic | `TrackService.Like` | does not call add for pending track | london | state-transition testing | normal |
| `IT-UC-TRK-LIST-01` | Business logic | `TrackService.List` | feed returns only ready tracks for the artist | integration | decision-table testing | critical |
| `IT-UC-TRK-LIST-02` | Business logic | `TrackService.List` | cursor walks the feed one page at a time | integration | boundary-value analysis | normal |
| `IT-UC-TRK-LIST-03` | Business logic | `TrackService.List` | limit above 100 is rejected | integration | boundary-value analysis | normal |
| `UC-TRK-LIST-01` | Business logic | `TrackService.List` | returns ready tracks of trimmed artist | classic | equivalence partitioning | normal |
| `UC-TRK-LIST-02` | Business logic | `TrackService.List` | applies cursor offset and returns next cursor | classic | equivalence partitioning | normal |
| `UC-TRK-LIST-03` | Business logic | `TrackService.List` | defaults zero limit to 20 | classic | boundary-value analysis | normal |
| `UC-TRK-LIST-04` | Business logic | `TrackService.List` | accepts maximum limit 100 | classic | boundary-value analysis | normal |
| `UC-TRK-LIST-05` | Business logic | `TrackService.List` | rejects limit 101 before listing | classic | boundary-value analysis | normal |
| `UC-TRK-LIST-06` | Business logic | `TrackService.List` | rejects non-numeric cursor abc before listing | classic | equivalence partitioning | normal |
| `UC-TRK-LIST-07` | Business logic | `TrackService.List` | rejects negative cursor -1 before listing | classic | boundary-value analysis | normal |
| `UC-TRK-LIST-08` | Business logic | `TrackService.List` | returns repository list error | classic | error guessing | normal |
| `IT-UC-TRK-LISTUP-01` | Business logic | `TrackService.ListByUploader` | uploader list includes a pending track | integration | state-transition testing | normal |
| `UC-TRK-LISTUP-01` | Business logic | `TrackService.ListByUploader` | returns uploader tracks with requested status | classic | equivalence partitioning | normal |
| `UC-TRK-LISTUP-02` | Business logic | `TrackService.ListByUploader` | returns uploader tracks of every status when status is omitted | classic | equivalence partitioning | normal |
| `UC-TRK-LISTUP-03` | Business logic | `TrackService.ListByUploader` | rejects missing uploader before listing | classic | equivalence partitioning | normal |
| `UC-TRK-LISTUP-04` | Business logic | `TrackService.ListByUploader` | returns repository list error | classic | error guessing | normal |
| `IT-UC-TRK-LISTLIKED-01` | Business logic | `TrackService.ListLiked` | liked list hides a liked pending track | integration | state-transition testing | critical |
| `UC-TRK-LISTLIKED-01` | Business logic | `TrackService.ListLiked` | returns only liked tracks in ready status | classic | state-transition testing | normal |
| `UC-TRK-LISTLIKED-02` | Business logic | `TrackService.ListLiked` | rejects missing user before listing likes | classic | equivalence partitioning | normal |
| `UC-TRK-LISTLIKED-03` | Business logic | `TrackService.ListLiked` | returns like repository error | classic | error guessing | normal |
| `IT-UC-TRK-STREAM-01` | Business logic | `TrackService.StreamURL` | stream url is returned for a ready track | integration | state-transition testing | normal |
| `IT-UC-TRK-STREAM-02` | Business logic | `TrackService.StreamURL` | stream url of a pending track returns conflict | integration | state-transition testing | critical |
| `UC-TRK-STREAM-01` | Business logic | `TrackService.StreamURL` | presigns stream url for ready track | classic | state-transition testing | normal |
| `UC-TRK-STREAM-02` | Business logic | `TrackService.StreamURL` | rejects pending track without presigning | classic | state-transition testing | normal |
| `UC-TRK-STREAM-03` | Business logic | `TrackService.StreamURL` | rejects processing track without presigning | classic | state-transition testing | normal |
| `UC-TRK-STREAM-04` | Business logic | `TrackService.StreamURL` | rejects empty id before lookup | classic | boundary-value analysis | normal |
| `UC-TRK-STREAM-05` | Business logic | `TrackService.StreamURL` | returns lookup error | classic | error guessing | normal |
| `UC-TRK-STREAM-06` | Business logic | `TrackService.StreamURL` | returns presign error | classic | error guessing | normal |
| `IT-UC-TRK-UNLIKE-01` | Business logic | `TrackService.Unlike` | unlike removes the stored like | integration | state-transition testing | normal |
| `IT-UC-TRK-UNLIKE-02` | Business logic | `TrackService.Unlike` | unlike of a track that was never liked succeeds | integration | state-transition testing | normal |
| `UC-TRK-UNLIKE-01` | Business logic | `TrackService.Unlike` | removes only the requested like | classic | equivalence partitioning | normal |
| `UC-TRK-UNLIKE-02` | Business logic | `TrackService.Unlike` | succeeds idempotently when track is not liked | classic | state-transition testing | normal |
| `UC-TRK-UNLIKE-03` | Business logic | `TrackService.Unlike` | removes like of pending track without ready check | classic | state-transition testing | normal |
| `UC-TRK-UNLIKE-04` | Business logic | `TrackService.Unlike` | rejects missing track id without removing like | classic | equivalence partitioning | normal |
| `UC-TRK-UNLIKE-05` | Business logic | `TrackService.Unlike` | propagates not found for unknown track without removing like | classic | equivalence partitioning | normal |
| `UC-TRK-UNLIKE-06` | Business logic | `TrackService.Unlike` | returns track lookup error without removing like | classic | error guessing | normal |
| `UC-TRK-UNLIKE-07` | Business logic | `TrackService.Unlike` | returns remove like error | classic | error guessing | normal |
| `IT-UC-TRK-UPDATE-01` | Business logic | `TrackService.Update` | update persists the title and keeps the status | integration | equivalence partitioning | normal |
| `UC-TRK-UPDATE-01` | Business logic | `TrackService.Update` | updates title and artist preserving status, owner and object | classic | equivalence partitioning | normal |
| `UC-TRK-UPDATE-02` | Business logic | `TrackService.Update` | clears artist when artist is omitted | classic | equivalence partitioning | normal |
| `UC-TRK-UPDATE-03` | Business logic | `TrackService.Update` | reports empty id before missing title | classic | decision-table testing | normal |
| `UC-TRK-UPDATE-04` | Business logic | `TrackService.Update` | rejects missing title before lookup | classic | equivalence partitioning | normal |
| `UC-TRK-UPDATE-05` | Business logic | `TrackService.Update` | propagates not found for unknown track | classic | equivalence partitioning | normal |
| `UC-TRK-UPDATE-06` | Business logic | `TrackService.Update` | returns lookup error | classic | error guessing | normal |
| `UC-TRK-UPDATE-07` | Business logic | `TrackService.Update` | returns update error and keeps stored track | classic | error guessing | normal |
| `UC-USR-CREATE-01` | Business logic | `UserService.Create` | normalizes email, hashes password and stores user | classic | equivalence partitioning | critical |
| `UC-USR-CREATE-02` | Business logic | `UserService.Create` | rejects password of length 7 before hashing | classic | boundary-value analysis | critical |
| `UC-USR-CREATE-03` | Business logic | `UserService.Create` | reports invalid email before short password | classic | decision-table testing | normal |
| `UC-USR-CREATE-04` | Business logic | `UserService.Create` | returns hashing error without storing user | classic | error guessing | normal |
| `UC-USR-CREATE-05` | Business logic | `UserService.Create` | returns repository create error | classic | error guessing | normal |
| `IT-UC-USR-DELETE-01` | Business logic | `UserService.Delete` | delete of a user who owns tracks is refused | integration | error guessing | critical |
| `UC-USR-DELETE-01` | Business logic | `UserService.Delete` | deletes user and password hash | classic | equivalence partitioning | normal |
| `UC-USR-DELETE-02` | Business logic | `UserService.Delete` | rejects empty id without deleting | classic | boundary-value analysis | normal |
| `UC-USR-DELETE-03` | Business logic | `UserService.Delete` | propagates not found for unknown user | classic | equivalence partitioning | normal |
| `UC-USR-DELETE-04` | Business logic | `UserService.Delete` | returns repository delete error and keeps user | classic | error guessing | normal |
| `UC-USR-GET-01` | Business logic | `UserService.GetByID` | returns user by id | classic | equivalence partitioning | normal |
| `UC-USR-GET-02` | Business logic | `UserService.GetByID` | rejects empty id before lookup | classic | boundary-value analysis | normal |
| `UC-USR-GET-03` | Business logic | `UserService.GetByID` | propagates not found for unknown id | classic | equivalence partitioning | normal |
| `UC-USR-GET-04` | Business logic | `UserService.GetByID` | returns repository lookup error | classic | error guessing | normal |
| `UC-USR-LIST-01` | Business logic | `UserService.List` | returns stored users | classic | equivalence partitioning | normal |
| `UC-USR-LIST-02` | Business logic | `UserService.List` | returns repository list error | classic | error guessing | normal |
| `IT-UC-USR-UPDATE-01` | Business logic | `UserService.Update` | update persists the profile | integration | equivalence partitioning | normal |
| `UC-USR-UPDATE-01` | Business logic | `UserService.Update` | updates email and name but not password | classic | equivalence partitioning | normal |
| `UC-USR-UPDATE-02` | Business logic | `UserService.Update` | reports empty id before invalid email | classic | decision-table testing | normal |
| `UC-USR-UPDATE-03` | Business logic | `UserService.Update` | rejects invalid email without updating | classic | equivalence partitioning | normal |
| `UC-USR-UPDATE-04` | Business logic | `UserService.Update` | propagates not found for unknown user | classic | equivalence partitioning | normal |
| `UC-USR-UPDATE-05` | Business logic | `UserService.Update` | returns repository update error | classic | error guessing | normal |
| `DA-LIKE-ADD-01` | Data access | `TrackLikeRepository.Add` | add sends user and track ids and succeeds on insert | stub | equivalence partitioning | normal |
| `DA-LIKE-ADD-02` | Data access | `TrackLikeRepository.Add` | add maps unique violation on track_likes_pkey to track already liked | stub | decision-table testing | critical |
| `DA-LIKE-ADD-03` | Data access | `TrackLikeRepository.Add` | add maps unique violation on another constraint to email already exists | stub | decision-table testing | critical |
| `DA-LIKE-ADD-04` | Data access | `TrackLikeRepository.Add` | add maps foreign key violation to not found | stub | decision-table testing | critical |
| `DA-PGX-LIKE-ADD-01` | Data access | `TrackLikeRepository.Add` | add wraps an unavailable database as internal like track error preserving the cause | pgxmock | error guessing | normal |
| `IT-DA-LIKE-ADD-01` | Data access | `TrackLikeRepository.Add` | add persists a like row | integration | equivalence partitioning | normal |
| `IT-DA-LIKE-ADD-02` | Data access | `TrackLikeRepository.Add` | duplicate like returns conflict | integration | error guessing | critical |
| `IT-DA-LIKE-ADD-03` | Data access | `TrackLikeRepository.Add` | like of an unknown track returns not found | integration | error guessing | normal |
| `IT-DA-LIKE-ADD-04` | Data access | `TrackLikeRepository.Add` | malformed track id returns invalid id | integration | error guessing | normal |
| `DA-LIKE-LISTREADY-01` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready by user sends user id and ready status and returns liked tracks | stub | equivalence partitioning | normal |
| `DA-LIKE-LISTREADY-02` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready by user wraps a query failure as internal list liked tracks error | stub | error guessing | normal |
| `DA-LIKE-LISTREADY-03` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready by user maps a rows iteration error to internal list tracks error | stub | error guessing | normal |
| `IT-DA-LIKE-LISTREADY-01` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready likes returns the liked ready track | integration | equivalence partitioning | normal |
| `IT-DA-LIKE-LISTREADY-02` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready likes skips a liked pending track | integration | state-transition testing | critical |
| `IT-DA-LIKE-LISTREADY-03` | Data access | `TrackLikeRepository.ListReadyByUser` | list ready likes returns only the requested user's likes | integration | equivalence partitioning | normal |
| `DA-LIKE-REMOVE-01` | Data access | `TrackLikeRepository.Remove` | remove sends user and track ids and succeeds when one row is deleted | stub | equivalence partitioning | normal |
| `DA-LIKE-REMOVE-02` | Data access | `TrackLikeRepository.Remove` | remove of a missing like succeeds without error | stub | boundary-value analysis | normal |
| `DA-LIKE-REMOVE-03` | Data access | `TrackLikeRepository.Remove` | remove maps sqlstate 22P02 to invalid id | stub | error guessing | normal |
| `DA-LIKE-REMOVE-04` | Data access | `TrackLikeRepository.Remove` | remove wraps a driver failure as internal unlike track error | stub | error guessing | normal |
| `IT-DA-LIKE-REMOVE-01` | Data access | `TrackLikeRepository.Remove` | remove deletes the like and keeps user and track | integration | equivalence partitioning | normal |
| `IT-DA-LIKE-REMOVE-02` | Data access | `TrackLikeRepository.Remove` | remove of a missing like succeeds without changes | integration | boundary-value analysis | normal |
| `DA-PGX-TRK-CREATE-01` | Data access | `TrackRepository.Create` | create sends exact insert arguments and reads back the generated id and object key | pgxmock | equivalence partitioning | critical |
| `DA-TRK-CREATE-01` | Data access | `TrackRepository.Create` | create sends owner title artist size and status and returns generated id and object key | stub | equivalence partitioning | critical |
| `DA-TRK-CREATE-02` | Data access | `TrackRepository.Create` | create maps foreign key violation on tracks_user_id_fkey to not found | stub | decision-table testing | critical |
| `IT-DA-TRK-CREATE-01` | Data access | `TrackRepository.Create` | create persists a pending track and derives the object key | integration | equivalence partitioning | critical |
| `IT-DA-TRK-CREATE-02` | Data access | `TrackRepository.Create` | create for an unknown user returns not found | integration | error guessing | normal |
| `DA-TRK-DELETE-01` | Data access | `TrackRepository.Delete` | delete sends the id and succeeds when one row is affected | stub | equivalence partitioning | normal |
| `DA-TRK-DELETE-02` | Data access | `TrackRepository.Delete` | delete maps zero affected rows to track not found | stub | boundary-value analysis | normal |
| `DA-TRK-DELETE-03` | Data access | `TrackRepository.Delete` | delete maps sqlstate 22P02 to invalid id | stub | error guessing | normal |
| `IT-DA-TRK-DELETE-01` | Data access | `TrackRepository.Delete` | delete removes the track row | integration | equivalence partitioning | normal |
| `IT-DA-TRK-DELETE-02` | Data access | `TrackRepository.Delete` | delete of an unknown uuid returns not found | integration | error guessing | normal |
| `IT-DA-TRK-DELETE-03` | Data access | `TrackRepository.Delete` | delete cascades the track's likes | integration | state-transition testing | normal |
| `DA-TRK-GET-01` | Data access | `TrackRepository.GetByID` | get by id sends the id and returns the scanned track with status | stub | equivalence partitioning | normal |
| `DA-TRK-GET-02` | Data access | `TrackRepository.GetByID` | get by id maps pgx no rows to track not found | stub | error guessing | normal |
| `DA-TRK-GET-03` | Data access | `TrackRepository.GetByID` | get by id maps sqlstate 22P02 to invalid id | stub | error guessing | normal |
| `IT-DA-TRK-GET-01` | Data access | `TrackRepository.GetByID` | get by id reads a seeded track | integration | equivalence partitioning | normal |
| `DA-TRK-LIST-01` | Data access | `TrackRepository.List` | list with all filters sends status user and artist and returns scanned tracks | stub | decision-table testing | normal |
| `DA-TRK-LIST-02` | Data access | `TrackRepository.List` | list with empty filter passes nil for all three filter arguments | stub | boundary-value analysis | normal |
| `DA-TRK-LIST-03` | Data access | `TrackRepository.List` | list with only status filter passes nil user and artist | stub | decision-table testing | normal |
| `DA-TRK-LIST-04` | Data access | `TrackRepository.List` | list with only user filter passes nil status and artist | stub | decision-table testing | normal |
| `DA-TRK-LIST-05` | Data access | `TrackRepository.List` | list with only artist filter passes nil status and user | stub | decision-table testing | normal |
| `DA-TRK-LIST-06` | Data access | `TrackRepository.List` | list wraps a query failure as internal list tracks error | stub | error guessing | normal |
| `DA-TRK-LIST-07` | Data access | `TrackRepository.List` | list maps a row scan failure to internal scan track error and closes rows | stub | error guessing | normal |
| `DA-TRK-LIST-08` | Data access | `TrackRepository.List` | list maps a rows iteration error to internal list tracks error | stub | error guessing | normal |
| `IT-DA-TRK-LIST-01` | Data access | `TrackRepository.List` | list by artist and ready status returns the matching track | integration | decision-table testing | normal |
| `IT-DA-TRK-LIST-02` | Data access | `TrackRepository.List` | artist filter ignores case and surrounding spaces and keeps the status filter | integration | decision-table testing | normal |
| `IT-DA-TRK-LIST-03` | Data access | `TrackRepository.List` | owner filter without status returns every status | integration | decision-table testing | normal |
| `IT-DA-TRK-LIST-04` | Data access | `TrackRepository.List` | apostrophe is matched literally | integration | error guessing | critical |
| `IT-DA-TRK-LIST-05` | Data access | `TrackRepository.List` | percent sign is not a wildcard | integration | error guessing | critical |
| `IT-DA-TRK-LIST-06` | Data access | `TrackRepository.List` | underscore is not a wildcard | integration | error guessing | critical |
| `IT-DA-TRK-LIST-07` | Data access | `TrackRepository.List` | cyrillic artist is matched exactly | integration | error guessing | critical |
| `IT-DA-TRK-LIST-08` | Data access | `TrackRepository.List` | sql text in the filter is treated as data | integration | error guessing | critical |
| `DA-TRK-UPDATE-01` | Data access | `TrackRepository.Update` | update sends every column including the explicit object key and returns the updated track | stub | equivalence partitioning | normal |
| `DA-TRK-UPDATE-02` | Data access | `TrackRepository.Update` | update with empty object key sends the id based default key | stub | boundary-value analysis | critical |
| `DA-TRK-UPDATE-03` | Data access | `TrackRepository.Update` | update maps pgx no rows to track not found | stub | error guessing | normal |
| `DA-TRK-UPDATE-04` | Data access | `TrackRepository.Update` | update maps foreign key violation on tracks_user_id_fkey to not found | stub | decision-table testing | critical |
| `IT-DA-TRK-UPDATE-01` | Data access | `TrackRepository.Update` | update persists title and status | integration | state-transition testing | normal |
| `IT-DA-TRK-UPDATE-02` | Data access | `TrackRepository.Update` | update of an unknown uuid returns not found | integration | error guessing | normal |
| `DA-USR-CREATE-01` | Data access | `UserRepository.Create` | create sends email name and password hash and returns the generated id | stub | equivalence partitioning | critical |
| `DA-USR-CREATE-02` | Data access | `UserRepository.Create` | create maps unique violation on users_email_key to email already exists | stub | decision-table testing | critical |
| `DA-USR-CREATE-03` | Data access | `UserRepository.Create` | create wraps an unmapped sqlstate as internal create user error preserving the cause | stub | error guessing | normal |
| `IT-DA-USR-CREATE-01` | Data access | `UserRepository.Create` | create persists a user row | integration | equivalence partitioning | critical |
| `IT-DA-USR-CREATE-02` | Data access | `UserRepository.Create` | duplicate email returns conflict | integration | error guessing | critical |
| `IT-DA-USR-CREATE-03` | Data access | `UserRepository.Create` | email uniqueness in the database is case-sensitive | integration | equivalence partitioning | normal |
| `DA-USR-DELETE-01` | Data access | `UserRepository.Delete` | delete sends the id and succeeds when one row is affected | stub | equivalence partitioning | normal |
| `DA-USR-DELETE-02` | Data access | `UserRepository.Delete` | delete maps zero affected rows to user not found | stub | boundary-value analysis | normal |
| `DA-USR-DELETE-03` | Data access | `UserRepository.Delete` | delete maps sqlstate 22P02 to invalid id | stub | error guessing | normal |
| `IT-DA-USR-DELETE-01` | Data access | `UserRepository.Delete` | delete removes the user row | integration | equivalence partitioning | normal |
| `IT-DA-USR-DELETE-02` | Data access | `UserRepository.Delete` | delete of a user who owns a track is refused | integration | error guessing | critical |
| `IT-DA-USR-DELETE-03` | Data access | `UserRepository.Delete` | delete of an unknown uuid returns not found | integration | error guessing | normal |
| `DA-USR-GETEMAIL-01` | Data access | `UserRepository.GetByEmail` | get by email sends the email and returns the user with password hash | stub | equivalence partitioning | critical |
| `DA-USR-GETEMAIL-02` | Data access | `UserRepository.GetByEmail` | get by email maps pgx no rows to user not found | stub | error guessing | normal |
| `DA-USR-GETEMAIL-03` | Data access | `UserRepository.GetByEmail` | get by email wraps a driver failure as internal get user by email error | stub | error guessing | normal |
| `IT-DA-USR-GETEMAIL-01` | Data access | `UserRepository.GetByEmail` | get by email returns the stored password hash | integration | equivalence partitioning | critical |
| `IT-DA-USR-GETEMAIL-02` | Data access | `UserRepository.GetByEmail` | get by unknown email returns not found | integration | error guessing | normal |
| `DA-PGX-USR-GET-01` | Data access | `UserRepository.GetByID` | get by id issues the exact user lookup query and returns the row | pgxmock | equivalence partitioning | normal |
| `DA-PGX-USR-GET-02` | Data access | `UserRepository.GetByID` | get by id maps pgx no rows from the driver to user not found | pgxmock | error guessing | normal |
| `DA-USR-GET-01` | Data access | `UserRepository.GetByID` | get by id sends the id and returns the scanned user | stub | equivalence partitioning | normal |
| `DA-USR-GET-02` | Data access | `UserRepository.GetByID` | get by id maps pgx no rows to user not found | stub | error guessing | normal |
| `DA-USR-GET-03` | Data access | `UserRepository.GetByID` | get by id maps sqlstate 22P02 to invalid id | stub | error guessing | normal |
| `IT-DA-USR-GET-01` | Data access | `UserRepository.GetByID` | get by id reads a seeded user | integration | equivalence partitioning | normal |
| `IT-DA-USR-GET-02` | Data access | `UserRepository.GetByID` | get by id of an unknown uuid returns not found | integration | error guessing | normal |
| `IT-DA-USR-GET-03` | Data access | `UserRepository.GetByID` | malformed uuid returns invalid id | integration | error guessing | normal |
| `DA-USR-LIST-01` | Data access | `UserRepository.List` | list returns every scanned user in row order and closes rows | stub | equivalence partitioning | normal |
| `DA-USR-LIST-02` | Data access | `UserRepository.List` | list returns an empty non-nil slice when there are no users | stub | boundary-value analysis | normal |
| `DA-USR-LIST-03` | Data access | `UserRepository.List` | list wraps a query failure as internal list users error | stub | error guessing | normal |
| `DA-USR-LIST-04` | Data access | `UserRepository.List` | list maps a row scan failure to internal scan user error and closes rows | stub | error guessing | normal |
| `DA-USR-LIST-05` | Data access | `UserRepository.List` | list maps a rows iteration error to internal list users error | stub | error guessing | normal |
| `IT-DA-USR-LIST-01` | Data access | `UserRepository.List` | list returns every stored user | integration | equivalence partitioning | normal |
| `IT-DA-USR-LIST-02` | Data access | `UserRepository.List` | list of an empty table returns an empty slice | integration | boundary-value analysis | normal |
| `DA-USR-UPDATE-01` | Data access | `UserRepository.Update` | update sends id email and name and returns the updated user | stub | equivalence partitioning | normal |
| `DA-USR-UPDATE-02` | Data access | `UserRepository.Update` | update maps pgx no rows to user not found | stub | error guessing | normal |
| `DA-USR-UPDATE-03` | Data access | `UserRepository.Update` | update maps unique violation on users_email_key to email already exists | stub | decision-table testing | critical |
| `IT-DA-USR-UPDATE-01` | Data access | `UserRepository.Update` | update persists a changed email and name | integration | equivalence partitioning | normal |
| `IT-DA-USR-UPDATE-02` | Data access | `UserRepository.Update` | update of an unknown uuid returns not found | integration | error guessing | normal |
| `IT-DA-USR-UPDATE-03` | Data access | `UserRepository.Update` | update to another user's email returns conflict | integration | error guessing | critical |
| `E2E-DEMO-01` | End to end | `DemoScenario.MVP` | publishes a track and lets another user like and stream it | e2e | state-transition testing | blocker |

Total: 384 cases.

<!-- matrix:end -->

## Известные проблемы и ограничения

- KI-1. Любое нарушение внешнего ключа (SQLSTATE 23503) становится not_found
  `not found` (HTTP 404). Удаление пользователя, у которого ещё есть треки,
  отклоняется с 404, как будто пользователя нет. Это фиксируют
  `IT-DA-USR-DELETE-02` и `IT-UC-USR-DELETE-01`: тесты закрепляют текущее
  поведение. Точнее был бы 409 conflict.
- KI-2. `TrackService.Update` и `TrackService.Delete` не принимают
  идентификатор вызывающего, а `PUT/DELETE /v1/tracks/{id}` не проверяют
  владельца. Любой авторизованный пользователь может переименовать или удалить
  чужой трек. То же относится к `PUT/DELETE /v1/users/{id}`.
- KI-3. `GET /v1/tracks/{id}` публичный, а `TrackService.GetByID` не требует
  `ready`, поэтому черновик `pending` читается по идентификатору.
- KI-4. Не-владелец на `CompleteUpload` получает unauthorized (401), а не
  forbidden (403).
- KI-5. Любое нарушение уникальности, кроме `track_likes_pkey`, сообщается как
  `email already exists`. Сейчас `users.email` — единственное другое
  ограничение уникальности.
- KI-6. Пагинация выполняется в памяти, после загрузки всех подходящих строк.
- KI-7. Ограничение частоты регистрации и входа не реализовано (документ
  архитектуры, раздел 2).
- KI-8. Если в `InitUpload` не удалась подпись и удаление при откате тоже не
  удалось, возвращается только ошибка удаления, а строка `pending` остаётся
  (`UC-TRK-INIT-10`).
- KI-9. У `Register` нет отката: если выпуск токена не удался, пользователь уже
  сохранён, и повторный вызов получает conflict `email already exists`
  (`UC-AUTH-REG-06`).
- KI-10. `TrackService.Delete` удаляет строку, но не сохранённый объект, поэтому
  аудиофайлы остаются в хранилище без владельца.
- KI-11. Проверка поверхностная: email достаточно содержать `@` (проходит даже
  голое `@`, `DOM-USER-VAL-03`), а идентификаторы и названия из одних пробелов
  проходят, потому что не обрезаются (`DOM-TRK-ID-02`, `DOM-TRK-WRITE-02`).
- KI-12. Нарушение внешнего ключа не говорит, отсутствует пользователь или трек,
  а SQLSTATE 22P02 всегда сообщается как `invalid id`, хотя этот код покрывает
  любое некорректное текстовое значение.

### Поведение, которое закрепляют тесты

Это осознанное или по крайней мере текущее поведение. Менять его стоит
отдельным решением, а не побочным эффектом другой правки.

- `Unlike` ничего не меняет при повторном вызове и не требует `ready`, но трек
  всё равно должен существовать (`UC-TRK-UNLIKE-02`, `UC-TRK-UNLIKE-03`).
  Репозиторный `Remove` не смотрит на число затронутых строк, в отличие от
  `Delete` у пользователей и треков (`DA-LIKE-REMOVE-02`).
- `TrackService.Update` заменяет исполнителя: пропущенный исполнитель очищает
  поле (`UC-TRK-UPDATE-02`). `UserService.Update` игнорирует пароль
  (`UC-USR-UPDATE-01`).
- `TrackRepository.Update` записывает `<id>.mp3`, когда ключ объекта пуст
  (`DA-TRK-UPDATE-02`).
- Порядок проверок фиксирован: лимит раньше курсора; идентификатор пользователя
  раньше страницы; `TrackUploadInit` проверяет идентификатор пользователя,
  название, затем размер; `TrackUploadComplete` — идентификатор трека, затем
  пользователя; `TrackLike` — идентификатор пользователя, затем трека.
- `Track.OwnedBy("")` проходит для трека, у которого владелец тоже пуст
  (`DOM-TRK-OWN-04`). У сохранённых треков владелец всегда есть.
- Отрицательный заявленный размер считается отсутствием заявленного размера
  (`DOM-TRK-CONFIRM-03`); максимум 0 и меньше отключает предел размера
  (`DOM-TRK-SIZE-02`, `DOM-TRK-SIZE-03`).
- `PageQuery.Page` рассчитывает, что `Validate` уже вызван: при непроверенном
  нулевом лимите возвращается пустая страница со следующим курсором `0`
  (`DOM-PAGE-PAGE-08`). Каждый метод сервиса сначала вызывает `Validate`.
- Курсор с пробелами по краям отклоняется (`DOM-PAGE-VAL-11`).
- Конструкторы доменных ошибок принимают пустой текст (`DOM-ERR-NEW-02`).
