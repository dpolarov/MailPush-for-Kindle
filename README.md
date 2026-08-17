# MailPush for KOReader

**English** | [Русский](#русский)

MailPush is a KOReader plugin for jailbroken Kindle devices that downloads books and other files from an ordinary IMAP mailbox. It is designed as a small, account-independent alternative to Amazon **Send to Kindle**: send an attachment or an HTTP(S) link to your mailbox, then fetch it from KOReader.

The UI is native KOReader Lua. Mail processing is handled by a small statically linked Go backend. **KUAL and Python are not required.**

> The project is inspired by [Le-Maxime/MailPushRU](https://github.com/Le-Maxime/MailPushRU), which in turn is based on [guo-yong-zhi/MailPush](https://github.com/guo-yong-zhi/MailPush). This repository is a new Go + Lua implementation rather than a direct Python port.

## Status

Primary real-device target: **Kindle Paperwhite 5 / 11th generation (PW5)** with jailbreak and KOReader.

Verified on a real PW5:

- KOReader plugin loading and Network-menu integration;
- IMAP TLS connection/authentication;
- downloading real book attachments;
- UID/processed-message behavior;
- GitHub release update detection;
- complete self-update to `v1.1.5`, including download, staged verification, installation and KOReader restart.

Other Kindle models may work, but they are not claimed as equally tested.

The production Kindle backend is deliberately conservative:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5
```

`GOARM=5` is intentional. Earlier updater experiments exposed a Kindle/PW5 `SIGBUS`; do not assume a more aggressive ARM target is safe merely because it compiles.

## Features

- Download one or multiple attachments from one or multiple messages.
- Download files referenced by `http://` or `https://` links in the subject/plain-text body.
- Optional automatic extraction of ZIP, TAR, TAR.GZ/TGZ and TAR.BZ2/TBZ2.
- Destination directives:
  - `save to`
  - legacy `saveto`
  - `сохранить в`
- Manual **Fetch mail now**.
- Manual **Test connection**.
- Optional one-time **Fetch once when KOReader starts**.
- No daemon and no periodic background mailbox polling.
- IMAP over TLS without OAuth; provider app passwords work where required.
- Bundled public CA certificates plus optional custom/private CA.
- Persistent UID/UIDVALIDITY state prevents duplicate downloads.
- Configurable message/file/archive limits and timeouts.
- Root filesystem sandbox for all message-requested output paths.
- Safe archive extraction that rejects traversal and links.
- Built-in GitHub Release update check and self-update.
- 30-day update reminder snooze after choosing **Not now**.
- Staged update verification and retained previous-plugin backup.

## How it is built

MailPush is intentionally split into two runtime parts:

```text
KOReader / Lua
  main.lua
    - menu and settings
    - backend invocation
    - result/error UI
    - update UX
       |
       +--> updater.lua
            - KOReader NetworkMgr
            - LuaSocket/socketutil
            - GitHub Releases API
            - ffi/archiver
            - staged update validation/install

Go backend
  bin/mailpush
    - IMAP
    - MIME parsing
    - attachment processing
    - mail-referenced HTTP(S) downloads
    - filesystem sandbox
    - normal archive extraction
    - UID state
```

Self-update networking is intentionally in KOReader Lua rather than the Go HTTP updater path because the latter caused real-device compatibility problems during earlier attempts.

## Installation

Download the release asset:

```text
mailpush.koplugin-bilingual.zip
```

Extract it so the plugin directory is exactly:

```text
/mnt/us/koreader/plugins/mailpush.koplugin/
```

Expected installed layout:

```text
mailpush.koplugin/
├── VERSION
├── _meta.lua
├── main.lua
├── updater.lua
├── updater_download.lua
├── config.default.json
├── cacert.pem
└── bin/
    └── mailpush
```

If the executable bit was lost while copying manually:

```sh
chmod 755 /mnt/us/koreader/plugins/mailpush.koplugin/bin/mailpush
```

Then fully restart KOReader and open:

```text
Settings -> Network -> MailPush
```

Configure IMAP and run **Test connection** before fetching mail.

A separate Russian installation guide is also available in [`INSTALL_RU.md`](INSTALL_RU.md).

## Updating

Use:

```text
Settings -> Network -> MailPush -> Check for updates
```

MailPush reads GitHub `releases/latest` and looks for the exact release asset:

```text
mailpush.koplugin-bilingual.zip
```

If a newer version is available, the user can install it immediately or choose **Not now**. Declining suppresses automatic reminders for 30 days; a manual forced check is still possible.

### What the updater validates

The current updater does **not** overwrite the running plugin directly. It:

1. waits for network availability through KOReader;
2. downloads the ZIP into a staging area;
3. checks compressed size;
4. opens the ZIP with KOReader `ffi/archiver`;
5. verifies archive root, entry paths/types, entry count and expanded-size limits;
6. extracts to a staged plugin directory;
7. verifies required files;
8. checks that packaged `VERSION` matches the GitHub release version;
9. runs the staged ARM backend with `version` and checks its embedded version;
10. only then renames the current plugin to `mailpush.koplugin.previous` and activates the staged plugin;
11. asks to restart KOReader.

Updater hard limits are intentionally smaller than normal book/archive limits:

```text
release ZIP:       <= 32 MiB
expanded update:   <= 64 MiB
archive entries:   <= 256
```

The previous plugin is retained as:

```text
/mnt/us/koreader/plugins/mailpush.koplugin.previous
```

There is currently no one-click rollback menu; the backup is primarily for manual recovery.

## Configuration

Persistent configuration is stored outside the plugin directory under KOReader's settings directory:

```text
<KOReader settings>/mailpush/config.json
```

Processed-message state is stored separately:

```text
<KOReader settings>/mailpush/state.json
```

Update snooze state and the last backend response are stored as:

```text
<KOReader settings>/mailpush/update_state.json
<KOReader settings>/mailpush/last_result.json
```

This separation means normal plugin replacement does not erase account settings or processed-message history.

Default configuration:

```json
{
  "host": "",
  "port": 993,
  "user": "",
  "password": "",
  "mailbox": "INBOX",
  "tls": true,
  "ca_file": "",
  "download_dir": "/mnt/us/documents/downloads",
  "root": "/mnt/us",
  "max_age_days": 7,
  "max_messages": 20,
  "fetch_unread_only": true,
  "mark_seen": true,
  "fetch_on_start": false,
  "auto_unpack": true,
  "max_file_bytes": 104857600,
  "max_message_bytes": 157286400,
  "max_archive_bytes": 314572800,
  "max_archive_files": 500,
  "connect_timeout_seconds": 15,
  "io_timeout_seconds": 45,
  "http_timeout_seconds": 60
}
```

Most user-facing fields are editable from the MailPush menu.

### Authentication

OAuth is intentionally not implemented. MailPush uses ordinary IMAP authentication. Providers that disallow the account password may require an **app password**.

The IMAP password must be stored on the Kindle because this target environment does not provide a portable secure keyring for the plugin. MailPush keeps its settings directory restricted and writes the config with mode `0600`; normal result/error output does not expose the password.

### TLS

Certificate verification is enabled.

- Public providers can use the CA bundle shipped with the plugin.
- Private/self-hosted servers can specify an additional CA certificate through `ca_file`.
- Incorrect Kindle date/time can also cause certificate validation failures.

## Sending books and files

The simplest message contains one or more attachments. No special subject is required.

HTTP(S) links may also be placed in the subject or plain-text body:

```text
https://example.org/book.epub
https://example.org/another-book.pdf
```

Only HTTP(S) is accepted. Schemes such as `file://` and `ftp://` are rejected, and redirect handling may not escape to an unsafe scheme.

### Destination directives

Examples:

```text
save to books/book.epub
save to books/one.epub|books/two.pdf
save to books/
saveto books/book.epub
сохранить в books/book.epub
```

A trailing `/` means a directory target. Multiple files keep their original filenames inside that directory.

All output paths are constrained by configured `root` (default `/mnt/us`). Attempts to escape through `../`, absolute paths or unsafe symlink traversal are rejected.

## IMAP processing semantics

MailPush deliberately uses IMAP UIDs rather than sequence numbers.

Normal fetch flow:

1. connect over TLS;
2. authenticate and select the configured mailbox;
3. read server `UIDVALIDITY`;
4. run `UID SEARCH` using age/unread filters;
5. skip UIDs already processed for the current `UIDVALIDITY`;
6. fetch selected mail with `UID FETCH ... BODY.PEEK[]`;
7. parse MIME content;
8. process attachments and allowed URLs;
9. record the UID only after complete success;
10. if enabled, separately mark it read with `UID STORE ... +FLAGS.SILENT (\\Seen)`.

`BODY.PEEK[]` matters: downloading a message must not implicitly mark it read before processing succeeds.

If `UIDVALIDITY` changes, the old stored UID set is discarded because those UIDs no longer identify the same mailbox state.

## Archive support and safety

Normal downloaded content can be automatically extracted when `auto_unpack` is enabled.

Supported formats:

- `.zip`
- `.tar`
- `.tar.gz` / `.tgz`
- `.tar.bz2` / `.tbz2`

Incoming mail and downloaded files are untrusted. The backend therefore rejects:

- traversal outside the destination;
- symlinks in archives;
- hardlinks in archives;
- excessive expanded bytes;
- excessive archive entry counts.

It also applies file/message/network limits before or during processing.

## Building

Go version:

```text
go 1.23
```

The project currently uses only the Go standard library, so no `go.sum` is expected.

### Recommended Docker build

```sh
sh ./scripts/build.sh
```

Artifacts:

```text
dist/mailpush.koplugin.zip
dist/mailpush-linux-amd64
```

The Docker build:

- runs Go tests;
- cross-compiles a static Kindle ARM binary with `GOARM=5`;
- builds an amd64 host binary used by CI version checks;
- adds the CA bundle;
- writes packaged `mailpush.koplugin/VERSION`;
- creates the installable plugin ZIP.

### Make targets

```sh
make test
make vet
make build-arm
make build-host
make docker
make clean
```

`make build-arm` uses the same conservative `GOARM=5` target as the production Docker build.

## Tests

Minimum Go checks:

```sh
go test ./...
go vet ./...
```

Additional host race testing:

```sh
go test -race ./...
```

Lua must remain compatible with Lua 5.1:

```sh
luac5.1 -p mailpush.koplugin/main.lua
luac5.1 -p mailpush.koplugin/updater.lua
luac5.1 -p mailpush.koplugin/updater_download.lua
```

The test/CI suite covers or validates, among other things:

- UTF-8 BOM config input and BOM-free output;
- UID/UIDVALIDITY state handling;
- `BODY.PEEK[]` and separate `\\Seen` behavior;
- MIME/RFC2047/RFC2231 filenames;
- multiple attachments and `save to` variants;
- unsafe URL schemes and redirects;
- filesystem and symlink traversal;
- ZIP/archive limits;
- TLS with an additional CA certificate;
- conservative ARM build;
- Lua 5.1 syntax;
- release ZIP structure and updater required files;
- packaged and embedded version agreement.

## GitHub Actions

### Test workflow

`.github/workflows/test.yml` runs separate Go, Lua and Docker/package checks.

A green Go job alone is not sufficient for update changes: release packaging has previously been the source of real-device update failures.

### Release workflow

Production release asset name is fixed:

```text
mailpush.koplugin-bilingual.zip
```

The intended normal release flow is:

1. change root `VERSION` to a valid `vX.Y.Z`;
2. run/verify tests;
3. push/merge the version change to `main`;
4. the Release workflow builds and validates the package;
5. it creates the version tag if needed;
6. it creates/updates the GitHub Release;
7. it uploads `mailpush.koplugin-bilingual.zip`.

The workflow can also be started manually from **Actions -> Release -> Run workflow** with a tag input.

Published tags are immutable release history. If a tag already exists at a different commit, do not force-move it simply to satisfy CI.

## Troubleshooting

### Authentication failed

- verify username/password;
- check whether the provider requires an app password;
- verify IMAP access is enabled for the account.

### Certificate / x509 error

- check Kindle date/time;
- verify `ca_file` if using a private CA;
- verify the IMAP server certificate chain.

### Connection refused / timeout

- verify Wi-Fi is actually online;
- check host/port;
- verify the network/provider permits IMAP.

### Outside configured root / symbolic link

The requested `save to` destination violates the configured filesystem sandbox. Keep destinations under `root`.

### Size/archive limit

The message, file or expanded archive exceeded the configured safety limits.

### Update archive error

Inspect the **actual latest GitHub Release asset**, not only a source-tree ZIP or CI artifact. The updater expects a single `mailpush.koplugin/` root and all required files listed in the installation section.

## Development notes

Detailed architecture, invariants, updater failure history, release rules and known technical debt are documented in [`AGENTS.md`](AGENTS.md).

Release history is in [`CHANGELOG.md`](CHANGELOG.md).

## Credits and license

MailPush for KOReader is inspired by the original MailPush/MailPushRU projects noted above.

See [`LICENSE`](LICENSE) for this repository's license.

---

# Русский

**MailPush for KOReader** — плагин для Kindle с jailbreak и KOReader, который скачивает книги и другие файлы из обычного IMAP-почтового ящика. Это независимая альтернатива Amazon **Send to Kindle**: отправляете вложение или HTTP(S)-ссылку на почту и забираете файл через меню KOReader.

Интерфейс написан на Lua и работает внутри KOReader. IMAP, MIME, скачивание файлов из писем, безопасные пути и распаковка реализованы небольшим статическим Go-бинарником. **Python и KUAL не нужны.**

## Проверенный статус

Основная тестовая платформа — **Kindle Paperwhite 5 / 11-е поколение (PW5)**.

На реальном PW5 проверены:

- загрузка плагина и меню KOReader;
- IMAP TLS и авторизация;
- реальное скачивание книги;
- история UID;
- поиск новой версии через GitHub Release;
- полноценное самообновление до `v1.1.5` с проверкой пакета, установкой и перезапуском KOReader.

Kindle-бинарник собирается специально консервативно:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5
```

`GOARM=5` выбран намеренно после реальной проблемы `SIGBUS` на PW5 во время ранних попыток обновления.

## Установка

Возьмите из GitHub Release файл:

```text
mailpush.koplugin-bilingual.zip
```

Распакуйте в:

```text
/mnt/us/koreader/plugins/mailpush.koplugin/
```

В каталоге должны быть как минимум:

```text
VERSION
_meta.lua
main.lua
updater.lua
updater_download.lua
config.default.json
cacert.pem
bin/mailpush
```

После установки полностью перезапустите KOReader и откройте:

```text
Settings -> Network -> MailPush
```

Заполните IMAP-настройки и сначала выполните **Test connection**.

Подробная русская инструкция также находится в [`INSTALL_RU.md`](INSTALL_RU.md).

## Основные возможности

- одно или несколько вложений;
- HTTP(S)-ссылки из темы/текста письма;
- ZIP/TAR/TAR.GZ/TAR.BZ2;
- `save to`, `saveto`, `сохранить в`;
- ручной Fetch;
- тест IMAP-подключения;
- однократный Fetch при старте KOReader;
- без фонового polling;
- UID + UIDVALIDITY вместо нестабильных порядковых номеров;
- `BODY.PEEK[]` без автоматического выставления Seen;
- опциональный отдельный `\\Seen` только после успешной обработки;
- root sandbox;
- ограничения размеров/таймаутов;
- встроенная проверка и установка новых GitHub Releases.

## Самообновление

Меню:

```text
Settings -> Network -> MailPush -> Check for updates
```

Обновление скачивается средствами KOReader, сначала полностью распаковывается и проверяется во временный staging-каталог и только после этого заменяет рабочий плагин.

Проверяется:

- корректный корень ZIP;
- отсутствие unsafe/path traversal;
- типы и количество записей;
- размер после распаковки;
- обязательные файлы;
- совпадение `VERSION` с GitHub Release;
- версия нового ARM backend до его активации.

Предыдущая версия сохраняется как:

```text
/mnt/us/koreader/plugins/mailpush.koplugin.previous
```

Если пользователь отказывается от предлагаемого обновления, автоматическое напоминание откладывается на 30 дней. Ручная проверка остаётся доступной.

## Где хранятся настройки

Настройки находятся **вне** каталога плагина:

```text
<KOReader settings>/mailpush/config.json
<KOReader settings>/mailpush/state.json
<KOReader settings>/mailpush/update_state.json
<KOReader settings>/mailpush/last_result.json
```

Поэтому обычное обновление плагина не должно удалять логин/пароль и историю обработанных писем.

Пароль хранится локально в `config.json` с ограниченными правами (`0600`), так как универсального secure keyring для целевого Kindle-окружения нет.

## Как отправить книгу

Просто отправьте вложение на настроенный ящик или добавьте ссылку:

```text
https://example.org/book.epub
```

Можно задать путь:

```text
save to books/book.epub
save to books/
saveto books/book.epub
сохранить в books/book.epub
```

Все пути должны оставаться внутри настроенного `root`, по умолчанию `/mnt/us`.

## Сборка

Рекомендуемый production-вариант:

```sh
sh ./scripts/build.sh
```

Результат:

```text
dist/mailpush.koplugin.zip
dist/mailpush-linux-amd64
```

Локально:

```sh
make test
make vet
make build-arm
make build-host
make docker
```

## Проверки

```sh
go test ./...
go vet ./...
go test -race ./...
```

Lua должен оставаться совместимым с Lua 5.1.

GitHub Actions дополнительно проверяет production ZIP: обязательные updater-файлы, безопасные пути, размеры, ARM-сборку и совпадение версии внутри пакета.

## Релизы

Обычный процесс:

1. изменить `VERSION` на `vX.Y.Z`;
2. проверить тесты;
3. отправить изменение в `main`;
4. Release workflow соберёт и проверит ZIP;
5. создаст тег/релиз;
6. загрузит `mailpush.koplugin-bilingual.zip`.

История изменений: [`CHANGELOG.md`](CHANGELOG.md).

Полное техническое описание для разработчиков и AI-агентов: [`AGENTS.md`](AGENTS.md).
