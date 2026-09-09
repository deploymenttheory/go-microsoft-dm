# Changelog

## 1.0.0 (2026-09-09)


### ⚠ BREAKING CHANGES

* **mdm:** certificate authenticators receive verified chains, and Basic credentials use CredentialHash instead of plaintext fields. SQL startup migrates legacy Basic credentials.

### Features

* bootstrap the two-module workspace, tier placeholders and layout tests ([3e1f49f](https://github.com/deploymenttheory/go-microsoft-dm/commit/3e1f49f6009ee9721698fc6a2706b9ceea35fff1))
* bootstrap the workspace and foundation packages (Phases 0 and 1) ([dc004b9](https://github.com/deploymenttheory/go-microsoft-dm/commit/dc004b9e009feeed81c8b445a27629820b705e72))
* desktop conformance and isolate reenrollment sessions ([6ca0e5f](https://github.com/deploymenttheory/go-microsoft-dm/commit/6ca0e5ffcf00502fa1f60ec0cf5fb4baae671e2e))
* implement WNS push and polling fallback ([e303dfd](https://github.com/deploymenttheory/go-microsoft-dm/commit/e303dfdd68ad4b2ffaa817a42516674e6f6f12b5))
* implement WNS push and polling fallback observability ([bb4a08c](https://github.com/deploymenttheory/go-microsoft-dm/commit/bb4a08c550dda493f9bc50d791f79eaf0cd6f784))
* **server:** Phase 6 — reference server, SQL storage, dmserver and dmctl ([cb326cf](https://github.com/deploymenttheory/go-microsoft-dm/commit/cb326cf18a8d52e03f8d0b55c04cce05d2f397ea))
* **server:** reference server with SQL storage, dmserver and dmctl ([7d73aa8](https://github.com/deploymenttheory/go-microsoft-dm/commit/7d73aa89834f987403925bd026aa5df790d0c836))


### Bug Fixes

* make native Windows enrollment provisioning compatible ([c2d8736](https://github.com/deploymenttheory/go-microsoft-dm/commit/c2d8736be3cf1fbb406218ce3197c5bb9cdf3ede))
* **mdm:** verify certificate identity and hash Basic credentials ([3b44c68](https://github.com/deploymenttheory/go-microsoft-dm/commit/3b44c685ce19272908d85affc751b7e9db75f3f8))
* **mdm:** verify certificate identity and hash Basic credentials ([bdaef0f](https://github.com/deploymenttheory/go-microsoft-dm/commit/bdaef0ffde44bb24cf5876c8c3d4fb6396c51f17))
* resolve native Windows enrollment provisioning failures ([1b0dc75](https://github.com/deploymenttheory/go-microsoft-dm/commit/1b0dc759d375fea2b44e9244e0bad6e6105b5354))
* **server:** gapless per-device sequence and MySQL-compatible indexes ([3941653](https://github.com/deploymenttheory/go-microsoft-dm/commit/3941653285fb76f662463bcfbd80c1cde3b4c260))
* **server:** retry transactions on transient deadlocks ([cd3d6ca](https://github.com/deploymenttheory/go-microsoft-dm/commit/cd3d6ca32f68e8d8125c8736c5d74aa98d773e21))
* **server:** use BIGINT and VARCHAR for portable SQL types ([0c91f2a](https://github.com/deploymenttheory/go-microsoft-dm/commit/0c91f2a1a9c4d375a2ed2acbd0b8be629c0e589f))
