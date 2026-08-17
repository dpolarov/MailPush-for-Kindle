# Changelog

All notable changes to MailPush for KOReader are documented here.

The project uses release tags in the form `vMAJOR.MINOR.PATCH`. Dates below use ISO `YYYY-MM-DD` format.

## [Unreleased]

### Documentation

- Reworked the project README to match the current runtime, updater, build and release behavior.
- Added `AGENTS.md` with architecture, invariants, troubleshooting notes and known technical debt for future maintainers and coding agents.

## [v1.1.5] - 2026-08-17

This is the first release with a self-update path successfully exercised on a real Kindle Paperwhite 5 from an older installed release to the new GitHub release.

### Changed

- Reworked self-update around the KOReader runtime instead of using the Go backend for updater networking and installation.
- GitHub release metadata and ZIP downloads now use KOReader's LuaSocket/socket utilities.
- Updates are downloaded to a staging area and fully validated before the installed plugin directory is touched.
- The installed plugin is replaced only after the staged package passes validation and backend self-test.
- The previous plugin directory is retained as `mailpush.koplugin.previous` for manual rollback/recovery.
- Added a packaged `mailpush.koplugin/VERSION` file so normal version checks do not need to execute the Go backend.
- Kindle release builds use the conservative `GOARM=5` target.
- Local `make build-arm` now matches the release target.

### Added

- Dedicated `mailpush.koplugin/updater.lua`.
- Release-package checks for required files, archive root, unsafe paths, entry count and unpacked-size limits.
- Staged backend version self-test before activation.
- Lua 5.1 syntax validation in CI.
- CI verification that the packaged `VERSION` matches the embedded Go backend version.
- CI verification of the exact updater archive contract expected on-device.

### Fixed

- Fixed the failed update path where a new updater downloaded an older release ZIP that did not contain the newly required `VERSION` file.
- Fixed the release side of that problem by publishing `v1.1.5` with the complete new package contract.
- Removed updater dependence on the Go HTTP stack that had produced Kindle-specific `SIGBUS` failures during earlier attempts.
- Avoided replacing the active plugin before extraction and validation have completed.

### Verified

- `go test ./...` and `go vet ./...` pass.
- ARM `GOARM=5` build passes.
- Lua 5.1 syntax checks pass.
- Docker release package validation passes.
- GitHub Release `v1.1.5` contains `mailpush.koplugin-bilingual.zip` with `VERSION`, `updater.lua`, CA bundle and ARM backend.
- On-device self-update to `v1.1.5` was successfully completed on a Kindle Paperwhite 5 running KOReader.

## [v1.1.4] - 2026-08-17

### Changed

- Version/release rebuild only; no functional source-code difference from `v1.1.3` other than the `VERSION` value.

## [v1.1.3] - 2026-08-17

### Changed

- Moved update download and installation work from the Go updater path into the KOReader runtime.
- Added a KOReader-native Lua updater downloader.
- Added archive validation before update extraction.
- Added staged plugin replacement and backup/restore behavior.

### Fixed

- Corrected LuaSocket update-download response handling.
- Corrected the expected update archive root/layout validation.
- Included the updater downloader in packaged releases.

### Notes

- This generation still split updater responsibilities across `main.lua`, `updater_download.lua`, `Device:unpackArchive`, `ffi/archiver` and legacy Go updater code. `v1.1.5` replaces that experimental path with the dedicated updater module.

## [v1.1.2] - 2026-08-17

### Fixed

- Changed the Kindle binary to the more conservative ARM target to avoid a `SIGBUS` observed on Kindle Paperwhite 5 during update-related networking.
- Disabled persistent HTTP connections and HTTP/2 in the legacy Go updater transport to reduce exposure to Kindle userspace/network-stack issues.

## [v1.1.1] - 2026-08-17

### Changed

- Version/release rebuild only; no functional source-code changes from `v1.1.0` other than the `VERSION` value.

## [v1.1.0] - 2026-08-17

### Added

- Initial GitHub Release self-update implementation.
- `VERSION`-based release versioning and automatic release packaging.
- Update checks after mail fetch.
- 30-day update reminder snooze after a user declines an offered update.
- Update ZIP verification and rollback-oriented installation logic.
- Embedded Go backend version information.
- CI-built installable plugin artifacts.

### Changed

- Release workflow builds the plugin package through the same Docker packaging path used for production releases.

## Earlier development

Before the `v1.1.x` updater work, the project established the current KOReader/Go architecture:

- native KOReader `.koplugin` UI;
- statically linked Go backend without Python or KUAL;
- IMAP UID/UIDVALIDITY state tracking;
- `BODY.PEEK[]` message retrieval and optional separate `\\Seen` marking;
- attachment and HTTP(S) link downloads;
- `save to` / `saveto` / `сохранить в` destination directives;
- safe filesystem sandboxing and archive extraction;
- configurable message, file, archive and timeout limits;
- bundled CA certificates with optional custom CA support.

[v1.1.5]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.5
[v1.1.4]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.4
[v1.1.3]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.3
[v1.1.2]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.2
[v1.1.1]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.1
[v1.1.0]: https://github.com/dpolarov/MailPush-for-Kindle/releases/tag/v1.1.0
