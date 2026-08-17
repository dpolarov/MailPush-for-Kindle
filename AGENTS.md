# AGENTS.md — MailPush for KOReader maintainer guide

This file is the project memory for maintainers, coding agents and future contributors. Read it before changing runtime behavior, updater code, release packaging, IMAP semantics or Kindle build flags.

The short version is: **MailPush is a KOReader Lua plugin with a small static Go backend. Keep normal mail processing in Go, keep self-update networking/installation in KOReader Lua, and build the Kindle backend conservatively with `GOARM=5`.**

## 1. Project purpose

MailPush for KOReader downloads books and other files from an ordinary IMAP mailbox to a jailbroken Kindle running KOReader.

It is intended as an account-independent/self-hostable alternative to Amazon Send to Kindle. The user sends attachments or HTTP(S) links to a mailbox, then asks MailPush to fetch them from the KOReader menu.

The project was inspired by:

- `Le-Maxime/MailPushRU`
- `guo-yong-zhi/MailPush`

This repository is a new Go + Lua implementation, not a direct Python port.

## 2. Supported/runtime environment

Primary real-device target:

- Kindle Paperwhite 5 / 11th generation (PW5)
- jailbroken Kindle
- KOReader
- 32-bit ARM Linux userspace

The production Kindle binary must be built conservatively:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5
```

### Important Kindle history

Earlier updater attempts used a less conservative ARM target and Go HTTP networking. A real PW5 produced `SIGBUS` during updater activity. The project subsequently moved the self-update network/install path into KOReader Lua and standardized the release ARM target on `GOARM=5`.

**Do not change the production Kindle target back to `GOARM=7` just because it builds on CI.** A change to ARM level or updater networking must be treated as a device-compatibility change and tested on a real Kindle.

## 3. Current verified state

As of `v1.1.5` (2026-08-17):

- the plugin loads on a real Kindle Paperwhite 5;
- the KOReader menu integration works;
- IMAP connection/authentication works;
- book download works;
- the dedicated KOReader-native self-updater works against a real GitHub Release;
- the published `v1.1.5` release ZIP satisfies the new updater package contract;
- an on-device update to `v1.1.5` completed successfully and KOReader restarted with the updated plugin.

The `v1.1.5` release was intentionally produced while validating the updater test branch so that the complete `releases/latest -> asset download -> staging -> validation -> replacement -> restart` path could be exercised against a real GitHub Release. Do not rewrite or move the published `v1.1.5` tag.

## 4. High-level architecture

```text
KOReader
  |
  | Lua UI / settings / update checks
  v
mailpush.koplugin/main.lua
  |                         \
  |                          \ self update
  |                           v
  |                     updater.lua
  |                     - NetworkMgr
  |                     - LuaSocket/socketutil
  |                     - GitHub Releases API
  |                     - ffi/archiver
  |                     - staging/validation
  |
  | normal mail commands
  v
mailpush.koplugin/bin/mailpush
  |
  +-- config
  +-- IMAP client
  +-- MIME/message parser
  +-- HTTP(S) downloader
  +-- safe filesystem resolver
  +-- archive extraction
  +-- processed UID state
```

### Responsibility boundary

**Lua/KOReader owns:**

- menu/UI;
- editing and persisting user-facing settings;
- invoking the backend;
- displaying results/errors;
- update check UX and 30-day snooze;
- GitHub update metadata/download networking;
- update ZIP staging/validation/activation;
- KOReader restart prompt.

**Go owns:**

- IMAP protocol;
- authentication;
- UID search/fetch/mark-seen operations;
- MIME parsing;
- attachment processing;
- extraction of URLs and save directives;
- normal HTTP(S) file downloads from messages;
- safe path resolution;
- normal downloaded archive extraction;
- persistent processed-message UID state;
- JSON command result output.

Do not move the updater back into Go without a very strong reason and real-device testing.

## 5. Repository map

```text
.
├── .github/workflows/
│   ├── test.yml              # Go, Lua and package-contract CI
│   └── release.yml           # production release build/publish workflow
├── cmd/mailpush/
│   ├── main.go               # Go CLI entry point
│   └── main_test.go
├── internal/
│   ├── archive/              # safe normal-file archive extraction
│   ├── config/               # config loading/default validation
│   ├── download/             # HTTP(S) downloads referenced by mail
│   ├── imapclient/           # low-level IMAP implementation
│   ├── message/              # MIME parsing, URLs, save directives
│   ├── safefs/               # root sandbox/path safety/atomic writes
│   ├── state/                # UIDVALIDITY + processed UID persistence
│   └── updater/              # legacy Go updater implementation; see tech debt
├── mailpush.koplugin/
│   ├── _meta.lua             # KOReader plugin metadata
│   ├── main.lua              # KOReader UI and normal backend bridge
│   ├── updater.lua           # active self-updater implementation
│   ├── updater_download.lua  # v1.1.x compatibility bridge, still packaged
│   └── config.default.json   # packaged default configuration
├── scripts/build.sh          # Docker package entry point
├── Dockerfile                # ARM + host build and release ZIP packaging
├── docker-compose.yml
├── Makefile
├── VERSION                   # source/release version
├── README.md
├── CHANGELOG.md
└── AGENTS.md
```

The generated release ZIP additionally contains:

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

`cacert.pem`, packaged `VERSION` and `bin/mailpush` are generated/copied during the Docker packaging stage; they do not all exist as source files under `mailpush.koplugin/`.

## 6. Runtime paths on Kindle

Typical installed plugin path:

```text
/mnt/us/koreader/plugins/mailpush.koplugin/
```

KOReader settings are stored outside the plugin directory using `DataStorage:getSettingsDir()`:

```text
<KOReader settings>/mailpush/config.json
<KOReader settings>/mailpush/state.json
<KOReader settings>/mailpush/update_state.json
<KOReader settings>/mailpush/last_result.json
```

This separation is intentional: replacing the plugin during an update must not wipe IMAP credentials or processed-message history.

The updater uses this backup directory:

```text
/mnt/us/koreader/plugins/mailpush.koplugin.previous
```

The `.previous` directory is a recovery copy of the version that was active immediately before the last successful replacement.

## 7. Configuration schema

Source defaults are in `mailpush.koplugin/config.default.json`.

| Key | Default | Meaning |
|---|---:|---|
| `host` | `""` | IMAP server hostname |
| `port` | `993` | IMAP port |
| `user` | `""` | IMAP username/email |
| `password` | `""` | IMAP password/app password |
| `mailbox` | `INBOX` | mailbox selected after login |
| `tls` | `true` | use TLS |
| `ca_file` | `""` | optional additional/private CA file |
| `download_dir` | `/mnt/us/documents/downloads` | default destination |
| `root` | `/mnt/us` | filesystem sandbox root |
| `max_age_days` | `7` | search age limit |
| `max_messages` | `20` | max messages processed per fetch |
| `fetch_unread_only` | `true` | search only unread messages |
| `mark_seen` | `true` | mark completely processed mail read |
| `fetch_on_start` | `false` | perform one fetch after KOReader startup |
| `auto_unpack` | `true` | unpack supported downloaded archives |
| `max_file_bytes` | `104857600` | per-file download/write limit (100 MiB) |
| `max_message_bytes` | `157286400` | fetched raw-message limit (150 MiB) |
| `max_archive_bytes` | `314572800` | normal archive expanded-byte limit (300 MiB) |
| `max_archive_files` | `500` | normal archive entry limit |
| `connect_timeout_seconds` | `15` | connection timeout |
| `io_timeout_seconds` | `45` | IMAP I/O timeout |
| `http_timeout_seconds` | `60` | mail-referenced HTTP(S) download timeout |

The updater has separate, intentionally smaller hard safety limits for the plugin release package:

```text
compressed ZIP <= 32 MiB
expanded content <= 64 MiB
archive entries <= 256
```

Do not casually unify the normal-book archive limits with the updater limits; they protect different attack surfaces.

## 8. Configuration persistence and secrets

`main.lua` creates the MailPush settings directory with mode `0700` and atomically writes config files with mode `0600`.

The IMAP password is stored in `config.json`. There is no general secure keyring available to this plugin on the target Kindle environment.

Rules:

- never print the password in normal output;
- never add config contents to debug logs without redaction;
- never move persistent config into the plugin directory;
- preserve UTF-8 BOM input compatibility in Go config loading;
- continue writing clean UTF-8 without BOM.

## 9. Backend command protocol

`main.lua` executes the static backend and expects one JSON object on stdout.

Relevant Go commands currently include:

```text
mailpush fetch
mailpush test
mailpush version
```

Legacy updater commands also still exist in Go (`check-update` / `install-update`), but the active KOReader UI updater no longer uses them. See Known issues / technical debt.

The Lua bridge passes paths roughly as:

```text
bin/mailpush \
  --config <settings>/mailpush/config.json \
  --state <settings>/mailpush/state.json \
  --ca-bundle <plugin>/cacert.pem \
  <command>
```

Backend output is stored in `last_result.json` for troubleshooting.

## 10. IMAP semantics — invariants

These behaviors are deliberate and should not be changed casually.

### Use UID, not sequence numbers

The client tracks messages using IMAP UIDs plus `UIDVALIDITY`.

Sequence numbers are mailbox-order dependent and unstable. Do not replace UID operations with sequence-number operations.

### Fetch without changing Seen state

Message data is retrieved using:

```text
BODY.PEEK[]
```

Do not replace this with `BODY[]` unless deliberately changing user-visible read/unread behavior.

### Mark read only after complete success

If `mark_seen` is enabled, `\\Seen` is applied separately after the message has been processed successfully.

A failed/partial message should remain eligible for retry instead of being silently consumed.

### UIDVALIDITY reset

If server `UIDVALIDITY` changes, stored UIDs belong to a different mailbox identity and must not be reused. The state layer resets the old processed UID set.

## 11. Message processing

A message can contain:

- one attachment;
- multiple attachments;
- HTTP(S) URLs in subject/plain-text body;
- destination directives.

Supported destination forms include:

```text
save to books/book.epub
save to books/one.epub|books/two.pdf
save to books/
saveto books/book.epub
сохранить в books/book.epub
```

A trailing slash denotes a destination directory and preserves source filenames for multiple items.

MIME handling includes encoded filenames (RFC2047/RFC2231 cases are covered by tests).

## 12. Normal HTTP(S) downloads

URLs received from mail are untrusted.

Rules:

- allow `http://` and `https://` only;
- reject `file://`, `ftp://` and other schemes;
- do not allow a redirect chain to escape to an unsafe scheme;
- enforce configured byte limits and timeouts;
- resolve the final destination through `safefs`.

This is separate from the self-updater's GitHub download path. Do not assume one implementation can safely replace the other without testing Kindle behavior.

## 13. Filesystem safety invariants

All message-requested output paths must remain inside configured `root`.

The project protects against:

- `../` traversal;
- absolute path escape;
- symlink traversal through existing filesystem components;
- overwrite races through atomic/unique writes where applicable.

When changing `internal/safefs`, treat every path from mail as attacker-controlled.

Do not solve path problems with string-prefix checks alone; path normalization and filesystem/symlink behavior matter.

## 14. Normal archive extraction safety

The backend can automatically unpack supported downloaded archives:

- ZIP
- TAR
- TAR.GZ / TGZ
- TAR.BZ2 / TBZ2

Archive entries are untrusted.

The normal archive implementation must continue rejecting:

- paths escaping the extraction destination;
- symlinks;
- hardlinks;
- excessive expanded bytes;
- excessive entry counts.

Never add a new archive type without the same traversal/link/limit protections.

## 15. Self-updater architecture (active path)

The active updater is `mailpush.koplugin/updater.lua`.

### Update check

1. `main.lua` calls `Updater.check(self, force)`.
2. Automatic checks happen after a mail fetch unless the user snoozed updates.
3. Manual **Check for updates** uses `force=true`.
4. KOReader `NetworkMgr` gates work on connectivity.
5. LuaSocket/socket utilities fetch:

```text
https://api.github.com/repos/dpolarov/MailPush-for-Kindle/releases/latest
```

6. The updater looks for the exact release asset:

```text
mailpush.koplugin-bilingual.zip
```

7. Versions are compared numerically after stripping leading `v` and prerelease/build suffixes.

### Declining an update

If the user chooses **Not now**, `update_state.json` records a `snooze_until` timestamp 30 days in the future.

Manual forced checks are not blocked by snooze.

### Update installation transaction

The updater intentionally does not unpack directly over the live plugin.

Flow:

1. create a work/staging directory on the same plugin filesystem;
2. download the release ZIP there;
3. verify downloaded size;
4. open ZIP with KOReader `ffi/archiver`;
5. validate every archive path/type/count/expanded size;
6. extract into a staged `mailpush.koplugin` directory;
7. verify required files;
8. verify packaged `VERSION` equals the GitHub release version;
9. chmod the staged backend executable;
10. execute the **staged** ARM backend with `version` and verify its embedded version;
11. only after all checks pass, move current plugin to `.previous`;
12. atomically rename the staged plugin into the active path;
13. clear update snooze state;
14. ask the user to restart KOReader.

The installed plugin must not be touched before staging verification succeeds.

### Required update package files

The updater currently requires:

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

Changing this list requires coordinated changes to:

- `updater.lua`;
- `Dockerfile` packaging;
- `.github/workflows/test.yml` archive-contract validation;
- `.github/workflows/release.yml` archive-contract validation;
- README/AGENTS documentation;
- preferably a real-device update test.

## 16. Updater failures already encountered

Do not repeat these experiments without understanding why they failed.

### Attempt: update entirely in Go

Symptoms included Kindle/PW5 `SIGBUS` during updater/network work.

Mitigations attempted included conservative transport settings, disabling HTTP keep-alive/HTTP2 and changing ARM build flags. The project ultimately moved the active updater networking/install logic to KOReader Lua.

### Attempt: split Lua updater across many mechanisms

An intermediate design combined:

- Go release check;
- Lua downloader;
- `ffi/archiver` validation;
- `Device:unpackArchive` extraction;
- directory replacement in `main.lua`.

This created too many device-dependent boundaries and several small fixes were required (LuaSocket response handling, package inclusion, archive-root validation).

`v1.1.5` consolidates the active updater in `updater.lua` and stages the whole plugin before activation.

### Failure: updater required `VERSION` but latest release did not contain it

During real-device testing, the new updater downloaded the then-current `v1.1.4` release and correctly rejected it with:

```text
Update archive is missing required file: VERSION
```

Root cause: the updater/package contract had changed, but `releases/latest` still pointed to an older package.

Fix: publish `v1.1.5` using the new package contract and verify the actual GitHub Release asset, not only a local/CI artifact.

**Lesson:** whenever updater package requirements change, test against the exact published Release asset that a real Kindle will download.

## 17. Version sources

There are three version representations that must agree in a production package:

1. repository root `VERSION`;
2. generated `mailpush.koplugin/VERSION` inside the ZIP;
3. Go `main.version` embedded by linker flag:

```text
-X main.version=<version>
```

CI verifies the packaged file and host backend version against the expected release version. The on-device updater additionally executes the staged ARM backend and checks its reported version before activation.

Do not hand-edit only one of these representations in a release artifact.

## 18. Build system

Go version declared by the project:

```text
go 1.23
```

The module currently has no third-party Go modules, so absence of `go.sum` is normal.

### Recommended release/package build

```sh
sh ./scripts/build.sh
```

This uses Docker and produces:

```text
dist/mailpush.koplugin.zip
dist/mailpush-linux-amd64
```

The release workflow renames the plugin ZIP to:

```text
mailpush.koplugin-bilingual.zip
```

### Local targets

```sh
make test
make vet
make build-arm
make build-host
make docker
```

`make build-arm` must remain aligned with the production `GOARM=5` target.

## 19. Test commands

Minimum before pushing runtime/backend changes:

```sh
go test ./...
go vet ./...
```

Recommended on a development host where race testing is applicable:

```sh
go test -race ./...
```

Lua syntax must be valid for Lua 5.1:

```sh
luac5.1 -p mailpush.koplugin/main.lua
luac5.1 -p mailpush.koplugin/updater.lua
luac5.1 -p mailpush.koplugin/updater_download.lua
```

The CI workflow also builds the Docker package and validates its contents.

## 20. CI expectations

`.github/workflows/test.yml` should validate at least:

- Go tests;
- Go vet;
- conservative ARM build;
- Lua 5.1 syntax;
- Docker package build;
- embedded backend version;
- ZIP integrity;
- updater package required files;
- archive root/path safety assumptions;
- updater entry-count and expanded-size limits.

A green unit-test job is not enough for updater changes. The package-contract job matters because updater failures have previously been caused by release ZIP layout rather than source logic.

## 21. Release workflow

Production asset name is a public contract:

```text
mailpush.koplugin-bilingual.zip
```

The updater searches for that exact name in `releases/latest`.

Normal intended release process:

1. update root `VERSION` to a valid `vX.Y.Z` value;
2. ensure tests and package checks pass;
3. merge/push the version change to `main`;
4. `.github/workflows/release.yml` builds the package;
5. the workflow creates the tag if needed;
6. the workflow creates/updates the GitHub Release;
7. it uploads the exact fixed-name ZIP;
8. verify `releases/latest` and the published asset before asking for device testing.

The workflow can also be run manually with a tag input.

### Important release rule

If a tag already exists at a different commit, the release workflow intentionally treats that as a conflict. Do not force-move published version tags to make CI green.

Because `v1.1.5` was created during the real updater validation branch, take care when merging that branch into `main`: do not attempt to recreate or move `v1.1.5`. The next production change should use a new version.

## 22. Real-device test checklist for updater changes

For any meaningful updater/package change, CI is necessary but not sufficient.

Recommended test:

1. install a known older/dev build on a real Kindle;
2. confirm **Check for updates** sees the intended new version;
3. confirm the source is GitHub `releases/latest`;
4. confirm download succeeds over normal Kindle Wi-Fi;
5. confirm staged package validation succeeds;
6. confirm backend staged self-test succeeds;
7. confirm update reports success;
8. restart KOReader;
9. confirm MailPush appears in the Network menu;
10. run **Test connection**;
11. fetch a real test message/book;
12. confirm settings and processed history survived the update;
13. inspect `.previous` behavior/recovery if relevant.

When reporting success, state the Kindle model and KOReader context.

## 23. Debugging

Useful user-visible/runtime files:

```text
<settings>/mailpush/config.json
<settings>/mailpush/state.json
<settings>/mailpush/update_state.json
<settings>/mailpush/last_result.json
```

Common categories:

### Authentication failure

- wrong username/password;
- provider requires app password;
- IMAP disabled by provider.

### TLS/certificate failure

- Kindle date/time incorrect;
- private CA not configured;
- broken/custom server certificate chain.

### Connection timeout/refused

- Wi-Fi not actually online;
- wrong host/port;
- server/network blocking IMAP.

### Path/root error

- requested `save to` path escapes configured `root`;
- existing symlink crosses the sandbox boundary.

### Size/archive error

- message/file/archive exceeds configured safety limits;
- updater ZIP exceeds hard updater limits.

### Update package error

Verify the actual latest GitHub Release and inspect the ZIP root/files. Do not assume the branch artifact and published release asset are identical until checked.

## 24. Known issues / technical debt

These are known as of `v1.1.5`.

### 24.1 Legacy Go updater code remains

`internal/updater` and Go CLI updater commands still exist, but the active KOReader UI updater is Lua.

Why it remains:

- it came from the earlier updater generations;
- removing it was not required to stabilize the device path;
- tests/history around it may still be useful during cleanup.

Future cleanup can remove it, but do so as a separate change after confirming no scripts/users depend on the CLI updater commands.

### 24.2 `updater_download.lua` is currently a compatibility/package bridge

The dedicated `updater.lua` performs its own download. `updater_download.lua` is no longer the active downloader but remains in the package and required-file contract so the transition from earlier v1.1.x updater layouts is explicit/stable.

Do not remove it from only one place. Removal requires coordinated package-contract changes and should happen only after the supported upgrade floor no longer needs it.

### 24.3 GitHub asset digest is not currently verified by the Lua updater

The GitHub Releases API may expose an asset `digest`, and the updater reads it into release metadata, but the current Lua install path does not calculate/compare SHA-256 itself.

Current protections are:

- HTTPS;
- fixed GitHub repository/API endpoint;
- fixed expected asset name;
- compressed/expanded size limits;
- archive path/type validation;
- required-file validation;
- packaged `VERSION` match;
- staged ARM backend version self-test before activation.

A future improvement could add SHA-256 verification if a reliable KOReader-provided hash implementation is available across target Kindle builds. Do not add a shell dependency such as `sha256sum` without checking device availability.

### 24.4 Update checks are unauthenticated GitHub API requests

There is no GitHub token on the Kindle. Very frequent manual/fetch-triggered checks can eventually encounter GitHub's unauthenticated API rate limits.

The current 30-day snooze applies after the user declines an update; it is not a general check-rate cache.

Potential future improvement: persist a normal last-check timestamp and avoid checking more than once per configurable interval while still allowing forced manual checks.

### 24.5 `.previous` rollback is manual

The updater retains the previous plugin directory, but there is no KOReader menu action that restores it automatically.

A future improvement could add a safe **Restore previous version** action, but it must carefully handle a running plugin replacing itself and should probably require restart.

### 24.6 Credentials are stored on-device

The IMAP password is stored in a mode-`0600` JSON config because no portable secure credential store is available in the target environment.

This is an accepted design limitation, not something to hide in logs or documentation.

### 24.7 OAuth is not implemented

Providers that do not allow password/app-password IMAP cannot currently be used.

OAuth would substantially increase UI, token-storage, refresh and browser/device-flow complexity.

### 24.8 Only PW5 is the primary verified Kindle target

Other Kindle models may work, especially with the conservative ARM build, but do not claim broad hardware compatibility without real testing.

### 24.9 No background polling

MailPush intentionally checks on explicit fetch and optionally once at KOReader startup. There is no daemon/timer polling mailbox in the background.

This is currently a design choice that keeps battery/network behavior predictable.

### 24.10 Documentation duplication can drift

`README.md` is the main public documentation. `INSTALL_RU.md` is a separate Russian installation document and can become stale when packaging/updater behavior changes.

When changing installation paths, required package files, build target or release/update flow, search both documents.

## 25. Changes that require extra caution

Treat the following as high-risk changes:

- `GOARM` / cross-compile target;
- updater networking libraries;
- `ffi/archiver` behavior;
- plugin directory replacement/backup logic;
- required release ZIP layout;
- release asset filename;
- version comparison;
- IMAP UID/UIDVALIDITY behavior;
- `BODY.PEEK[]` vs `BODY[]`;
- when a message becomes processed/Seen;
- safe path/root logic;
- archive traversal/link checks;
- credential logging/persistence.

For these areas, preserve the existing invariant unless the task explicitly requires changing it.

## 26. Coding conventions

### Go

- prefer standard library; currently there are no external modules;
- return contextual errors;
- keep network/file limits explicit;
- keep untrusted-input handling defensive;
- add tests for parsing/path/archive/protocol edge cases;
- keep the backend JSON output stable for Lua consumers.

### Lua

- target KOReader's Lua 5.1 environment;
- do not use newer Lua syntax accidentally;
- use KOReader-provided runtime modules when device compatibility matters;
- wrap optional/device-sensitive behavior with `pcall` where failure must not crash KOReader;
- avoid long synchronous UI freezes when network work can be scheduled with `UIManager:scheduleIn`/`nextTick`;
- preserve clear user-facing error messages.

## 27. Definition of done

For normal Go changes:

```text
go test ./... passes
go vet ./... passes
relevant new tests added
```

For Lua/UI changes:

```text
Lua 5.1 syntax passes
KOReader module assumptions checked
normal backend flow still works
```

For build/package changes:

```text
Docker package builds
ARM target is GOARM=5
ZIP contract passes
VERSION and backend version agree
```

For updater/release changes:

```text
all of the above
actual GitHub Release asset inspected
releases/latest verified
real Kindle test strongly preferred / required before declaring solved
```

## 28. Do not declare an updater fix complete too early

This is the most important process lesson from the v1.1.x work.

A self-update change is **not verified** merely because:

- source compiles;
- unit tests pass;
- a local ZIP looks right;
- a GitHub Actions artifact looks right.

The real device downloads the asset attached to `releases/latest`. The final verification chain is:

```text
source
 -> CI
 -> production release workflow
 -> published GitHub Release asset
 -> Kindle network download
 -> KOReader extraction/staging
 -> version/self-test
 -> plugin activation
 -> KOReader restart
 -> functional mail fetch
```

Keep that chain intact.
