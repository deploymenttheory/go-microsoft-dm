# Changelog

## 1.0.0 (2026-09-09)


### ⚠ BREAKING CHANGES

* **mdm:** certificate authenticators receive verified chains, and Basic credentials use CredentialHash instead of plaintext fields. SQL startup migrates legacy Basic credentials.

### Features

* bootstrap the two-module workspace, tier placeholders and layout tests ([3e1f49f](https://github.com/deploymenttheory/go-microsoft-dm/commit/3e1f49f6009ee9721698fc6a2706b9ceea35fff1))
* bootstrap the workspace and foundation packages (Phases 0 and 1) ([dc004b9](https://github.com/deploymenttheory/go-microsoft-dm/commit/dc004b9e009feeed81c8b445a27629820b705e72))
* desktop conformance and isolate reenrollment sessions ([6ca0e5f](https://github.com/deploymenttheory/go-microsoft-dm/commit/6ca0e5ffcf00502fa1f60ec0cf5fb4baae671e2e))
* **enroll:** MS-MDE2 discovery, policy and enrollment service ([a121b08](https://github.com/deploymenttheory/go-microsoft-dm/commit/a121b086460234dad635762cfdd0c080c89983fc))
* foundation packages clock, paging, secrets, telemetry, state, ratelimit and testpki ([57e86b3](https://github.com/deploymenttheory/go-microsoft-dm/commit/57e86b3f87e7f5f7d79a0d5c7dea07be6565ca9a))
* implement WNS push and polling fallback ([e303dfd](https://github.com/deploymenttheory/go-microsoft-dm/commit/e303dfdd68ad4b2ffaa817a42516674e6f6f12b5))
* implement WNS push and polling fallback observability ([bb4a08c](https://github.com/deploymenttheory/go-microsoft-dm/commit/bb4a08c550dda493f9bc50d791f79eaf0cd6f784))
* **mdm:** diff desired vs acknowledged configuration helper ([5071c9f](https://github.com/deploymenttheory/go-microsoft-dm/commit/5071c9f78d7a6fbba130e2db0eb456ed9e86696d))
* **mdm:** MS-MDM management session engine over SyncML ([5e72f95](https://github.com/deploymenttheory/go-microsoft-dm/commit/5e72f952f79920bee782f26913d14a5f2affba50))
* **mdm:** unenroll command builders ([6f358be](https://github.com/deploymenttheory/go-microsoft-dm/commit/6f358be3c08bcf26503ce707ab9810ceb9d6e6d0))
* Phase 4 enrollment with MS-MDE2, XCEP and WSTEP ([e33eeeb](https://github.com/deploymenttheory/go-microsoft-dm/commit/e33eeebf95a5f4ce4a0d03bb2952bc97ea00e326))
* Phase 5 MS-MDM management session engine ([799d49d](https://github.com/deploymenttheory/go-microsoft-dm/commit/799d49da2df27d8e27e90303c0aad2ac60dea5c6))
* pin the DDF v2 bundle and the specifications, add the Makefile and scripts ([c906357](https://github.com/deploymenttheory/go-microsoft-dm/commit/c9063576c28ae642bb68ab0682de2427717ce92d))
* **pki:** CA issuer, Windows-tolerant CSR parser and XCEP policy ([bb501ad](https://github.com/deploymenttheory/go-microsoft-dm/commit/bb501adbc7968f21225468b7cb84a34860365e2f))
* **schema:** CSP schema generated from DDF v2 (Phase 3) ([715ee3c](https://github.com/deploymenttheory/go-microsoft-dm/commit/715ee3c867f8d83adebb255bae6ccd88dba72b97))
* **schemagen:** DDF v2 parser, deterministic generator, verify and drop diff ([f208c2a](https://github.com/deploymenttheory/go-microsoft-dm/commit/f208c2ac3146264152161b862b2af2f75e8a0a90))
* **schema:** generated CSP and Policy area packages from DDFv2Feb2026 ([4824142](https://github.com/deploymenttheory/go-microsoft-dm/commit/4824142e1129af68119330946cd6832763efe9fb))
* **schema:** runtime CSP model, registry lookup, build applicability and command validation ([4bc1497](https://github.com/deploymenttheory/go-microsoft-dm/commit/4bc14974d0e4ffc6bf8187b3900fed7545af11fc))
* **server:** Phase 6 — reference server, SQL storage, dmserver and dmctl ([cb326cf](https://github.com/deploymenttheory/go-microsoft-dm/commit/cb326cf18a8d52e03f8d0b55c04cce05d2f397ea))
* **server:** reference server with SQL storage, dmserver and dmctl ([7d73aa8](https://github.com/deploymenttheory/go-microsoft-dm/commit/7d73aa89834f987403925bd026aa5df790d0c836))
* **simulator:** client-side chunked upload of large Get results ([98f5368](https://github.com/deploymenttheory/go-microsoft-dm/commit/98f5368dfbd97995aac92103cbf43ad4dfba8f88))
* **simulator:** on-premise enrollment client ([1b4ceca](https://github.com/deploymenttheory/go-microsoft-dm/commit/1b4ceca1fc19f4f4503d655a6c738f6b6c150b66))
* **simulator:** software OMA DM client that runs management sessions ([ef84542](https://github.com/deploymenttheory/go-microsoft-dm/commit/ef845424f9c1c9fd4682cc2e8c223d4281b7996c))
* **soap:** SOAP envelope, WS-Security header and MS-MDE2 faults ([e175835](https://github.com/deploymenttheory/go-microsoft-dm/commit/e175835311850f26612bfb1f4a3fbad39d0b3b20))
* **storage:** enrollment and certificate contracts with in-memory backend ([b5e780b](https://github.com/deploymenttheory/go-microsoft-dm/commit/b5e780b7c18b0422f75800be39c63c962f830d97))
* **storage:** OMA DM command queue, credential store and session authenticator ([a01f074](https://github.com/deploymenttheory/go-microsoft-dm/commit/a01f0743816681c19575fb4e75db88f5989e434a))
* **storagetest:** add credential store contract suite ([8aad608](https://github.com/deploymenttheory/go-microsoft-dm/commit/8aad608aeae8da32304d99a26e81d47612f6e767))
* **syncml:** SyncML wire model (Phase 2) ([ba89b94](https://github.com/deploymenttheory/go-microsoft-dm/commit/ba89b94be43d8a3b1c59b101518be34cb983b0bf))
* **syncml:** typed, DTD-ordered SyncML 1.2 codec for the Windows OMA DM subset ([3ec6c20](https://github.com/deploymenttheory/go-microsoft-dm/commit/3ec6c2040cfc7ae3cbc731d528a8a1df542e8bff))
* **wapprov:** provisioning document model and w7 builders ([d40c092](https://github.com/deploymenttheory/go-microsoft-dm/commit/d40c092860b7a77f3b130608812980b5fcca03a2))


### Bug Fixes

* complete Phase 1-5 deliverables (diff helper, chunked upload, unenroll) ([bd965fd](https://github.com/deploymenttheory/go-microsoft-dm/commit/bd965fd2e5f6edd55196b36a980a3cbd8bbc7f95))
* make native Windows enrollment provisioning compatible ([c2d8736](https://github.com/deploymenttheory/go-microsoft-dm/commit/c2d8736be3cf1fbb406218ce3197c5bb9cdf3ede))
* **mdm:** verify certificate identity and hash Basic credentials ([3b44c68](https://github.com/deploymenttheory/go-microsoft-dm/commit/3b44c685ce19272908d85affc751b7e9db75f3f8))
* **mdm:** verify certificate identity and hash Basic credentials ([bdaef0f](https://github.com/deploymenttheory/go-microsoft-dm/commit/bdaef0ffde44bb24cf5876c8c3d4fb6396c51f17))
* resolve native Windows enrollment provisioning failures ([1b0dc75](https://github.com/deploymenttheory/go-microsoft-dm/commit/1b0dc759d375fea2b44e9244e0bad6e6105b5354))
* **server:** gapless per-device sequence and MySQL-compatible indexes ([3941653](https://github.com/deploymenttheory/go-microsoft-dm/commit/3941653285fb76f662463bcfbd80c1cde3b4c260))
* **server:** retry transactions on transient deadlocks ([cd3d6ca](https://github.com/deploymenttheory/go-microsoft-dm/commit/cd3d6ca32f68e8d8125c8736c5d74aa98d773e21))
* **server:** use BIGINT and VARCHAR for portable SQL types ([0c91f2a](https://github.com/deploymenttheory/go-microsoft-dm/commit/0c91f2a1a9c4d375a2ed2acbd0b8be629c0e589f))


### Documentation

* add the Windows MDM research store ([2ce1a67](https://github.com/deploymenttheory/go-microsoft-dm/commit/2ce1a6759e41796dd59983bb2f021900109f347c))
* add the Windows MDM research store ([c53f957](https://github.com/deploymenttheory/go-microsoft-dm/commit/c53f957aa349226034dadc9d4c2126fc9701ffaa))
* added implementation plan ([7b0159d](https://github.com/deploymenttheory/go-microsoft-dm/commit/7b0159dc01f82229007b353e88394ce3b002a7fe))
* added implementation plan ([ef6cdb9](https://github.com/deploymenttheory/go-microsoft-dm/commit/ef6cdb9969db2c8782fee8919818fcb6d08e84df))
* amend records 0012 and 0013 for the diff helper and SyncType gate ([2657c0b](https://github.com/deploymenttheory/go-microsoft-dm/commit/2657c0b55977892ba6da43191a3fab5312eaf2d5))
* decision record 0005 for the SyncML message model; mark Phase 2 done ([0b8ebc6](https://github.com/deploymenttheory/go-microsoft-dm/commit/0b8ebc65e3765a606fa300fd2e08b704d0e65f34))
* decision records 0006 and 0007; mark Phase 3 done ([0ebc4d0](https://github.com/deploymenttheory/go-microsoft-dm/commit/0ebc4d06e8be37cfb26cade46024cde455f166bd))
* decision records 0008 to 0010; mark Phase 4 done ([3f78088](https://github.com/deploymenttheory/go-microsoft-dm/commit/3f780884a2a71409c7c520e558dbeb95dacce64c))
* decision records 0011 to 0013; mark Phase 5 done ([16a2b86](https://github.com/deploymenttheory/go-microsoft-dm/commit/16a2b860ff937603154f3d50ae6a9f246f2ff37b))
* governance for go-microsoft-dm and decision records 0001 to 0004 ([08a8a9a](https://github.com/deploymenttheory/go-microsoft-dm/commit/08a8a9a4e2fb9962aef9425019b3400e30604319))
* mark Phase 6 complete and document the reference server ([5e187dd](https://github.com/deploymenttheory/go-microsoft-dm/commit/5e187dd2c8cb37cc95624b37a8f7519c96e534c5))
* **research:** guestweave builds Windows media with go-sdk-winmediafoundry ([50ba4aa](https://github.com/deploymenttheory/go-microsoft-dm/commit/50ba4aa7bd2075b49fd7a7fbeebe3ceb28e93afb))
* **research:** guestweave-macos ships a vTPM via go-sdk-vtpm2 ([0b68402](https://github.com/deploymenttheory/go-microsoft-dm/commit/0b684027f966c55c6d399a747fa4621ad1ab15a9))
* **research:** use guestweave for Windows 11 test environments ([a0b3885](https://github.com/deploymenttheory/go-microsoft-dm/commit/a0b388541a64db00794c6a90352f9b35b36c67f5))
* **research:** use guestweave for Windows 11 test environments ([29933c8](https://github.com/deploymenttheory/go-microsoft-dm/commit/29933c880bb9231ab4073e8dd13d0c6e0ac926fa))

## Changelog

Release-please generates this file from Conventional Commits on `main`. Nothing has been
released yet.
