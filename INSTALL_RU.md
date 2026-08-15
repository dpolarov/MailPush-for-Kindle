# Установка MailPush на Kindle Paperwhite 5 (11th generation)

## Что требуется

- Kindle Paperwhite 5 / 11th generation с jailbreak.
- Уже установленный и запускающийся KOReader.
- Docker на компьютере для сборки.

Python и KUAL самому MailPush-плагину не нужны.

## Сборка

Из корня проекта:

```bash
./scripts/build.sh
```

или через Docker Compose:

```bash
mkdir -p dist
docker compose run --rm build
```

После сборки появится:

```text
dist/mailpush.koplugin.zip
```

Внутри находится статический ARMv7 Go-бинарник для Kindle PW5.

## Копирование на Kindle

Распакуйте `mailpush.koplugin.zip` в:

```text
/mnt/us/koreader/plugins/
```

Должно получиться именно так:

```text
/mnt/us/koreader/plugins/mailpush.koplugin/_meta.lua
/mnt/us/koreader/plugins/mailpush.koplugin/main.lua
/mnt/us/koreader/plugins/mailpush.koplugin/config.default.json
/mnt/us/koreader/plugins/mailpush.koplugin/cacert.pem
/mnt/us/koreader/plugins/mailpush.koplugin/bin/mailpush
```

Если перенос файлов сбросил executable bit:

```bash
chmod 755 /mnt/us/koreader/plugins/mailpush.koplugin/bin/mailpush
```

Перезапустите KOReader.

## Настройка

Откройте **Settings → Network → MailPush** в KOReader и заполните:

- `IMAP host` — например `imap.gmail.com`;
- `IMAP port` — обычно `993`;
- `Username`;
- `Password` — обычный или app password в зависимости от почтового сервиса;
- `Mailbox` — обычно `INBOX`;
- `Download directory` — по умолчанию `/mnt/us/documents/downloads`;
- `Allowed root directory` — по умолчанию `/mnt/us`.

После этого нажмите **Test connection**. Все сообщения об ошибках и подсказки в интерфейсе специально выводятся на английском.

## Режим запуска

Фонового опроса нет.

MailPush работает только:

- по команде **Fetch mail now**;
- один раз при запуске KOReader, если включить **Fetch once when KOReader starts**.

## Прочитано / непрочитано

Письмо сначала читается IMAP-командой `BODY.PEEK[]`, поэтому само получение письма не выставляет флаг `Seen`.

После успешной обработки, если включена настройка **Mark successfully processed mail as read**, выполняется отдельная команда `UID STORE ... +FLAGS.SILENT (\\Seen)`.

## Повторная загрузка

Плагин хранит `UIDVALIDITY` и обработанные UID локально. Поэтому уже обработанное письмо повторно не скачивается даже если его снова пометить непрочитанным.

Для принудительной повторной обработки используйте **Reset processed-message history**.

## TLS и сертификаты

Проверка TLS включена. Плагин использует системные CA, доступные Go на Kindle, и дополнительно собственный `cacert.pem`, который кладётся в пакет при Docker-сборке из `ca-certificates` Debian.

Для собственного IMAP-сервера можно указать дополнительный сертификат через **Custom CA file**.

Отключение проверки сертификата в интерфейс специально не добавлено.

## Формат писем

Обычное вложение скачивается без специальных команд.

Можно передать HTTPS-ссылку:

```text
https://example.com/book.epub
```

Поддерживаются команды сохранения:

```text
save to books/book.epub
save to books/
save to books/one.epub|books/two.pdf
сохранить в books/book.epub
```

Если указано только `save to books/`, все файлы этого письма сохраняются в этот каталог с исходными именами.

`file://` и `ftp://` не поддерживаются намеренно.
