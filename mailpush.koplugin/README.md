# MailPush for KOReader

**English** | [Русский](#русский)

MailPush is a KOReader plugin for Kindle that downloads books and other files directly from an IMAP mailbox. It is intended as a self-hosted, account-independent alternative to Amazon **Send to Kindle** for jailbroken Kindles running KOReader.

The plugin has a native KOReader UI and a small statically linked Go backend. **KUAL and Python are not required.** Mail is checked only when you request it, or once at KOReader startup if that option is enabled; there is no background polling.

> **Origin / credits**
>
> This project was inspired by and based on the idea and behavior of **Le-Maxime/MailPushRU**: https://github.com/Le-Maxime/MailPushRU
>
> MailPushRU itself is a Russian-language fork of **guo-yong-zhi/MailPush**. This KOReader version is a new Go/Lua implementation rather than a direct Python port. Many thanks to the original authors for the idea of receiving files on Kindle through an ordinary IMAP mailbox.

## Features

- Download one or multiple email attachments in a single run.
- Download files from `http://` and `https://` links in the subject or text body.
- Optional automatic archive extraction: ZIP, TAR, TAR.GZ/TGZ and TAR.BZ2/TBZ2.
- `save to`, legacy `saveto`, and `сохранить в` directives for destination paths/names.
- Manual **Fetch mail now** and **Test connection** commands.
- Optional **Fetch once when KOReader starts**; no periodic/background polling.
- IMAP over TLS without OAuth. Provider app passwords can be used where required.
- Bundled public CA certificates plus support for an additional custom CA file.
- Multiple books are processed independently; MailPush reports results and does not automatically open a downloaded book.
- Persistent processed-message history prevents duplicate downloads.

## Why this version differs from MailPushRU

The original MailPushRU is a KUAL/Python solution. This version integrates directly into KOReader and moves the backend to Go:

- KOReader `.koplugin` UI instead of KUAL.
- Single static ARMv7 Go binary; no Python runtime on Kindle.
- IMAP **UID** and `UIDVALIDITY` instead of unstable sequence numbers.
- `BODY.PEEK[]` fetches messages without implicitly marking them read.
- `\\Seen` is a separate, configurable step after successful processing.
- Only HTTP(S) URLs are accepted; `file://`, `ftp://` and redirects to unsafe schemes are rejected.
- Config accepts UTF-8 BOM for compatibility but is atomically written as UTF-8 without BOM.
- Root sandbox and checks against `../`, absolute escapes and existing symlink traversal.
- Safe archive extraction rejects ZIP/TAR traversal, symlinks and hardlinks.
- Limits for message/download/archive sizes, file count and network timeouts.

## Compatibility

Primary target and tested hardware:

- **Tested on a real Kindle Paperwhite 5 (11th generation / PW5)** — plugin loading, KOReader Network menu integration, IMAP connection and successful book download were verified on-device.
- jailbroken Kindle
- KOReader
- ARMv7 Linux userspace

The Kindle backend is built with:

```text
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0
```

Other ARMv7 Kindle models running KOReader may work, but PW5 / 11th generation is the primary target.

## Installation

1. Build the project (see [Building](#building)), or use a prebuilt `mailpush.koplugin.zip` release.
2. Connect the Kindle by USB/SFTP.
3. Extract/copy the plugin so the directory is exactly:

   ```text
   /mnt/us/koreader/plugins/mailpush.koplugin/
   ```

   The layout must look like:

   ```text
   mailpush.koplugin/
   ├── _meta.lua
   ├── main.lua
   ├── config.default.json
   ├── cacert.pem
   └── bin/
       └── mailpush
   ```

4. If necessary, restore the executable bit:

   ```sh
   chmod 755 /mnt/us/koreader/plugins/mailpush.koplugin/bin/mailpush
   ```

5. Fully restart KOReader.
6. Open **Settings → Network → MailPush**.
7. Enter the IMAP settings and select **Test connection**.
8. Send a book to the configured mailbox and select **Fetch mail now**.

## Configuration

Configuration is available from **Settings → Network → MailPush**.

Typical settings are:

- IMAP host and port (normally TLS port `993`);
- username/email address;
- password or provider-specific app password;
- mailbox (`INBOX` by default);
- root/download directory;
- age/message limits;
- whether successfully processed messages should be marked read;
- whether to fetch once at KOReader startup;
- optional custom CA file.

The persistent config is kept under KOReader's settings directory in `mailpush/config.json` with mode `0600`, so replacing the plugin during an update does not normally erase the account settings.

### Authentication

OAuth is intentionally not implemented. MailPush uses ordinary IMAP authentication. Some providers require an **app password** instead of the normal account password.

TLS certificate verification is enabled. Public services can use the CA bundle shipped with the plugin; a private/self-hosted IMAP server can additionally specify its own CA certificate.

## Sending books

The simplest message contains one or more attachments. No special subject is required.

You can also put HTTP(S) download links in the subject or plain-text body:

```text
https://example.org/book.epub
https://example.org/another-book.pdf
```

To choose the destination name/path, use:

```text
save to books/book.epub
save to books/one.epub|books/two.pdf
save to books/
saveto books/book.epub
сохранить в books/book.epub
```

A directory target such as `save to books/` applies to multiple files while preserving their original filenames.

All destination paths are constrained by the configured `root` (default `/mnt/us`). Attempts to escape it are rejected.

## How message processing works

1. MailPush connects to the configured mailbox over TLS.
2. It performs `UID SEARCH` using the configured filters.
3. UIDs already recorded for the current `UIDVALIDITY` are skipped.
4. Each selected message is retrieved with `UID FETCH <uid> (BODY.PEEK[])`.
5. Attachments and allowed HTTP(S) links are processed independently.
6. After complete success, the UID is added to local processed history.
7. If enabled, MailPush separately runs `UID STORE <uid> +FLAGS.SILENT (\\Seen)`.

If `UIDVALIDITY` changes, the old processed-UID history is discarded because those UIDs no longer identify the same mailbox state.

## Archive support and safety

Automatic extraction supports:

- `.zip`
- `.tar`
- `.tar.gz` / `.tgz`
- `.tar.bz2` / `.tbz2`

Mail received from the Internet is untrusted input. MailPush therefore:

- never executes downloaded files;
- accepts only HTTP(S) remote URLs;
- rejects filesystem/path traversal;
- rejects symlinks and hardlinks in archives;
- limits message and download sizes;
- limits total extracted bytes and number of archive entries;
- uses network timeouts;
- keeps writes inside the configured root.

The password must be stored on the Kindle because there is no general-purpose secure keyring available to this plugin. The config is restricted to mode `0600`, and the password is not included in normal result/error output.

## Building

### Docker

Docker is the recommended build method:

```sh
./scripts/build.sh
```

or:

```sh
mkdir -p dist
docker compose run --rm build
```

Artifacts:

```text
dist/mailpush.koplugin.zip
dist/mailpush-linux-amd64
```

The Docker build cross-compiles the static ARMv7 Kindle backend and packages the KOReader plugin.

## Tests

Run locally with Go:

```sh
go test ./...
go vet ./...
go test -race ./...
```

The test suite covers, among other things:

- UTF-8 BOM input and BOM-free config output;
- UID/UIDVALIDITY state handling;
- `BODY.PEEK[]` and separate `\\Seen` handling;
- RFC2047/RFC2231 MIME names;
- multiple attachments and `save to` variants;
- rejection of `file://` and `ftp://`;
- HTTP limits and unsafe redirects;
- filesystem and symlink traversal;
- ZIP-slip/archive limits;
- TLS with an additional CA certificate.

---

# Русский

**MailPush for KOReader** — плагин для Kindle, который загружает книги и другие файлы напрямую из обычного почтового ящика по IMAP. По сути, это независимая замена **Amazon Send to Kindle** для Kindle с jailbreak и KOReader: отправляете книгу на выбранный почтовый ящик и забираете её через MailPush.

Плагин встроен в интерфейс KOReader, а с почтой работает небольшой статически собранный Go-бинарник. **KUAL и Python не нужны.** Почта проверяется только по команде пользователя либо один раз при запуске KOReader, если это включено в настройках. Фонового периодического опроса нет.

> **Откуда взята идея / благодарности**
>
> Идея и исходное поведение проекта взяты из репозитория **Le-Maxime/MailPushRU**: https://github.com/Le-Maxime/MailPushRU
>
> Сам MailPushRU является русскоязычным форком **guo-yong-zhi/MailPush**. Эта версия для KOReader — новая реализация на Go + Lua, а не прямой перенос Python-кода. Спасибо авторам исходных проектов за идею доставки файлов на Kindle через обычный IMAP-ящик.

## Возможности

- Загрузка одного или нескольких вложений из одного или нескольких писем.
- Загрузка файлов по `http://` и `https://` ссылкам из темы или текста письма.
- Автоматическая распаковка ZIP, TAR, TAR.GZ/TGZ и TAR.BZ2/TBZ2.
- Управление именем и каталогом через `save to`, старый `saveto` или `сохранить в`.
- **Fetch mail now** — проверить почту вручную.
- **Test connection** — проверить IMAP-настройки без загрузки книг.
- Опциональный однократный **Fetch on KOReader start**.
- Никакого фонового polling.
- IMAP over TLS без OAuth; при необходимости используются пароли приложений почтового сервиса.
- Встроенный набор публичных CA-сертификатов и возможность добавить собственный CA.
- Корректная обработка нескольких книг: они скачиваются независимо, автоматически открывать одну из них MailPush не пытается.
- История UID защищает от повторного скачивания уже обработанных писем.

## Чем отличается от MailPushRU

MailPushRU работает через KUAL и Python. Эта версия переработана специально для KOReader:

- интерфейс нативного `.koplugin` вместо KUAL;
- один статический ARMv7 Go-бинарник, Python на Kindle не требуется;
- IMAP **UID + UIDVALIDITY** вместо порядковых номеров писем;
- чтение через **`BODY.PEEK[]`**, поэтому само получение письма не выставляет `Seen`;
- пометка `\\Seen` выполняется отдельной управляемой командой только после успешной обработки;
- разрешены только HTTP(S)-ссылки; `file://`, `ftp://` и небезопасные redirect запрещены;
- чтение конфига с UTF-8 BOM поддерживается, запись всегда UTF-8 без BOM и атомарная;
- защита от `../`, выхода за `root`, существующих symlink и небезопасной распаковки;
- лимиты размеров, количества файлов и таймауты.

## Совместимость

Основная цель проекта:

- **Протестировано на реальном Kindle Paperwhite 5 (11-е поколение / PW5)** — проверены загрузка плагина, интеграция в меню Network, IMAP-подключение и успешное скачивание книги;
- jailbreak;
- установленный KOReader;
- ARMv7 Linux userspace.

Kindle-бинарник собирается как:

```text
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0
```

На других ARMv7 Kindle с KOReader плагин также может работать, но основная проверяемая платформа — PW5 / 11th gen.

## Установка

1. Соберите проект через Docker или возьмите готовый `mailpush.koplugin.zip` из release.
2. Подключите Kindle по USB/SFTP.
3. Распакуйте плагин точно в:

   ```text
   /mnt/us/koreader/plugins/mailpush.koplugin/
   ```

4. Проверьте структуру:

   ```text
   mailpush.koplugin/
   ├── _meta.lua
   ├── main.lua
   ├── config.default.json
   ├── cacert.pem
   └── bin/
       └── mailpush
   ```

5. Если при копировании потерялся executable bit:

   ```sh
   chmod 755 /mnt/us/koreader/plugins/mailpush.koplugin/bin/mailpush
   ```

6. Полностью перезапустите KOReader.
7. Откройте **Settings → Network → MailPush**.
8. Заполните IMAP-настройки и нажмите **Test connection**.
9. Отправьте письмо с книгой и выберите **Fetch mail now**.

## Настройка

Все основные параметры доступны в **Settings → Network → MailPush**:

- IMAP host и port (обычно TLS `993`);
- email/username;
- пароль или пароль приложения;
- mailbox (`INBOX` по умолчанию);
- корневой каталог и каталог загрузки;
- максимальный возраст/количество писем;
- помечать ли успешно обработанное письмо прочитанным;
- проверять ли почту один раз при запуске KOReader;
- дополнительный пользовательский CA-файл.

Постоянный конфиг хранится отдельно от каталога плагина в настройках KOReader: `mailpush/config.json`, права `0600`. Поэтому обновление папки плагина обычно не удаляет настройки почты.

### Авторизация

OAuth намеренно не используется. MailPush работает с обычным IMAP. Если почтовый сервис запрещает вход с основным паролем, создайте **app password / пароль приложения**.

Проверка TLS-сертификатов включена. Для публичных почтовых сервисов используется CA bundle, поставляемый вместе с плагином; для собственного IMAP-сервера можно дополнительно указать свой CA.

## Как отправить книгу

Самый простой вариант — отправить письмо с одним или несколькими файлами во вложении. Тема может быть любой или пустой.

Можно отправить HTTP(S)-ссылки в теме или обычном текстовом теле письма:

```text
https://example.org/book.epub
https://example.org/another-book.pdf
```

Для выбора имени/пути:

```text
save to books/book.epub
save to books/one.epub|books/two.pdf
save to books/
saveto books/book.epub
сохранить в books/book.epub
```

`save to books/` означает каталог: если файлов несколько, все они попадут туда под исходными именами.

Все пути ограничены настроенным `root` (по умолчанию `/mnt/us`). Выйти через `../` или другим способом за пределы `root` нельзя.

## Как обрабатывается почта

1. MailPush устанавливает TLS-соединение с IMAP.
2. Выполняет `UID SEARCH` с выбранными фильтрами.
3. Уже обработанные UID для текущего `UIDVALIDITY` пропускаются.
4. Письмо читается командой `UID FETCH <uid> (BODY.PEEK[])`.
5. Вложения и допустимые HTTP(S)-ссылки обрабатываются независимо.
6. После полного успеха UID сохраняется в локальной истории.
7. Если включена соответствующая настройка, отдельно выполняется `UID STORE <uid> +FLAGS.SILENT (\\Seen)`.

При изменении `UIDVALIDITY` старая история UID автоматически сбрасывается.

## Архивы и безопасность

Поддерживается автоматическая распаковка:

- `.zip`
- `.tar`
- `.tar.gz` / `.tgz`
- `.tar.bz2` / `.tbz2`

Письмо считается недоверенным внешним вводом, поэтому MailPush:

- никогда не запускает скачанные файлы;
- принимает удалённые ссылки только HTTP(S);
- блокирует path traversal;
- блокирует symlink/hardlink в архивах;
- ограничивает размер писем и загрузок;
- ограничивает суммарный размер распаковки и количество файлов;
- использует сетевые таймауты;
- не позволяет писать за пределами настроенного `root`.

Пароль необходимо хранить на Kindle, поскольку у плагина нет универсального системного secure keyring. Файл конфигурации имеет права `0600`, а пароль не выводится в обычных результатах и сообщениях об ошибках.

## Сборка Docker

Рекомендуемый вариант:

```sh
./scripts/build.sh
```

или:

```sh
mkdir -p dist
docker compose run --rm build
```

Результат:

```text
dist/mailpush.koplugin.zip
dist/mailpush-linux-amd64
```

## Тесты

```sh
go test ./...
go vet ./...
go test -race ./...
```

Тестами проверяются в том числе BOM, UID/UIDVALIDITY, `BODY.PEEK[]`, отдельный `\\Seen`, MIME/RFC2047/RFC2231, несколько вложений, `save to`, запрет `file://`/`ftp://`, HTTP redirect/size limits, traversal/symlink, ZIP-slip, ограничения архивов и TLS с дополнительным CA.

## GitHub releases

The repository includes `.github/workflows/release.yml`. A release is built only after `go test ./...` and `go vet ./...` succeed. The workflow builds the Kindle package with the project's Docker build, verifies the ZIP, and publishes this exact release asset:

```text
mailpush.koplugin-bilingual.zip
```

Recommended release flow:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Pushing a `v*` tag automatically creates the GitHub Release and attaches the ZIP. The workflow can also be started manually from **Actions → Release → Run workflow**, where you provide a tag such as `v1.0.0`.

The workflow uses the repository's built-in `GITHUB_TOKEN`; no personal access token or release secret is required for a normal GitHub repository. The workflow therefore requests `contents: write` permission.

---

## Публикация релизов в GitHub

В проект добавлен `.github/workflows/release.yml`. Релиз собирается только после успешных `go test ./...` и `go vet ./...`. Затем GitHub Actions собирает пакет через Docker, проверяет ZIP и прикрепляет к GitHub Release файл с фиксированным именем:

```text
mailpush.koplugin-bilingual.zip
```

Рекомендуемый способ выпуска версии:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Push тега `v*` автоматически создаст GitHub Release и приложит ZIP. Также workflow можно запустить вручную через **Actions → Release → Run workflow**, указав тег, например `v1.0.0`.

Используется встроенный `GITHUB_TOKEN` репозитория, поэтому отдельный Personal Access Token или secret для обычной публикации релиза не требуется. Workflow запрашивает разрешение `contents: write`.
