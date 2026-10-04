# Контракт API

Спека: [openapi.yaml](openapi.yaml). Это целевой контракт, не описание текущего роутера. Код под него ещё не переписан. Операции с `x-implemented: false` в коде отсутствуют.

В контракт не входят методы, которые есть в коде, но не имеют отдельной роли:

- `POST /v1/users` — второй способ создать аккаунт рядом с `register`, токен не выдаёт;
- `PUT /v1/users/{id}` и `DELETE /v1/users/{id}` — меняет и удаляет только свой профиль, чужой id отвечает 403; в контракте это `/v1/me`;
- голый `POST /v1/tracks` — файл не идёт через API, создание трека это `initTrackUpload`.

Профиль меняет только владелец: полная замена `replaceCurrentUser`, частичная `updateCurrentUser`. Пароль меняется только частичным обновлением.

## Ревью контракта

Методы. Создание уникальной записи — `POST` и ответ 201 или 204. Полная замена полей — `PUT`. Частичная замена абсолютных значений — `PATCH`: пропущенное поле не затирается, пустое тело — 400. Снятие связи — `DELETE`.

Идемпотентность. `PUT` и `PATCH` с абсолютными полями можно повторять. `followUser` и `unfollowUser` при повторе снова отвечают 204. `unlikeTrack` тоже 204, а 404 только если нет трека. `likeTrack` и `addPlaylistTrack` при повторе пары отвечают 409: это создание уникальной строки, не «убедиться, что она есть». `DELETE` сущности (`deleteTrack`, `deletePlaylist`, `deletePick`, `deleteCurrentUser`) на уже отсутствующий ресурс отвечает 404.

Коды. 400 — битое тело или параметры, 401 — нет или плохой токен, 403 — токен есть, но это не владелец, 404 — нет ресурса или он скрыт, 409 — занятый email, повтор пары или неверный статус, 429 — только регистрация и вход, 500 — сбой без деталей. Стрим готового трека — 302, неготового — 409.

Обязательные поля. Регистрация требует email и пароль от 8 символов, имя можно не передавать. Вход требует непустые email и пароль, короткий пароль здесь не 400. Замена профиля требует email и имя. Замена трека требует title и artist. В плейлист при добавлении обязательны `track_id` и `position`. Статус, размер и ключ объекта клиент не пишет.

Пагинация. У страниц одна форма: `items`, `next_cursor`, `limit`. `limit` от 1 до 100, по умолчанию 20. `listUsers` курсор не использует: справочник приходит целиком.

Проверка из каталога `docs`: `npm install`, затем `npm run lint`. Линтер Spectral включает правила `oas3-valid-media-example` и `oas3-valid-schema-example`. Mock поднимается контейнером: `docker compose up` из каталога `docs`, адрес `http://127.0.0.1:4010`. Сценарий публикации в другом терминале: `npm run scenario` (`login` → `initTrackUpload` → `completeTrackUpload` → `listMyTracks`).

## Что есть в коде

| operationId | В коде |
| --- | --- |
| `register`, `login` | да; 429 по IP из `AUTH_RATE_LIMIT` и `AUTH_RATE_WINDOW` |
| `listUsers`, `getUser`, `getCurrentUser` | да |
| `replaceCurrentUser`, `updateCurrentUser`, `deleteCurrentUser` | нет |
| `listTracks`, `listUserTracks`, `listMyTracks`, `getTrack` | да |
| `streamTrack`, `initTrackUpload`, `completeTrackUpload` | да |
| `replaceTrack`, `deleteTrack` | да, чужой трек отвечает 403 |
| `updateTrack` | нет |
| `likeTrack`, `unlikeTrack`, `listMyLikes` | да |
| плейлисты, подборки, подписки | нет |

Расхождения уже существующих операций с контрактом:

- `PUT /v1/users/{id}` не записывает пароль, даже если он передан.

## Сценарий → шаг → operationId

| Сценарий | Шаг | operationId |
| --- | --- | --- |
| Гость слушает ленту | Открыть ленту готовых треков | `listTracks` |
| Гость слушает ленту | Включить трек | `streamTrack` |
| Автор публикует трек | Войти или зарегистрироваться | `login` или `register` |
| Автор публикует трек | Создать карточку и получить ссылку записи | `initTrackUpload` |
| Автор публикует трек | Подтвердить объект в хранилище | `completeTrackUpload` |
| Автор публикует трек | Увидеть загрузку у себя | `listMyTracks` |
| Пользователь собирает любимое | Войти | `login` |
| Пользователь собирает любимое | Поставить отметку | `likeTrack` |
| Пользователь собирает любимое | Снять отметку | `unlikeTrack` |
| Пользователь собирает любимое | Открыть список любимого | `listMyLikes` |
| Поиск артиста | Запросить ленту с фильтром `artist` | `listTracks` |
| Сборка плейлиста | Создать список | `createPlaylist` |
| Сборка плейлиста | Добавить готовый трек | `addPlaylistTrack` |
| Сборка плейлиста | Прочитать состав | `listPlaylistTracks` |

## CRUD и частичное обновление

| Ресурс | Создать | Читать | Заменить | Частично | Удалить |
| --- | --- | --- | --- | --- | --- |
| Профиль | `register` | `getCurrentUser`, `getUser`, `listUsers` | `replaceCurrentUser` | `updateCurrentUser` | `deleteCurrentUser` |
| Трек | `initTrackUpload` | `getTrack`, `listTracks`, `listMyTracks`, `listUserTracks` | `replaceTrack` | `updateTrack` | `deleteTrack` |
| Плейлист | `createPlaylist` | `getPlaylist`, `listMyPlaylists`, `listUserPlaylists` | `replacePlaylist` | `updatePlaylist` | `deletePlaylist` |
| Трек в плейлисте | `addPlaylistTrack` | `listPlaylistTracks` | — | `movePlaylistTrack` | `removePlaylistTrack` |
| Подборка | `generatePick` | `getMyPick`, `listMyPicks` | — | `renamePick` | `deletePick` |
| Лайк | `likeTrack` | `listMyLikes` | — | — | `unlikeTrack` |
| Подписка | `followUser` | `listMyFollowing`, `listFollowers`, `listUserFollowing` | — | — | `unfollowUser` |

У подборки нет полной замены: состав собирает система, клиент меняет только название. Лайк и подписка — пара без тела, отдельное частичное обновление им не нужно. Позиция трека в плейлисте меняется через `movePlaylistTrack`.
