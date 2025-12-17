# Changelog

## [0.14.1](https://github.com/openkcm/session-manager/compare/v0.14.0...v0.14.1) (2025-12-17)


### Bug Fixes

* syntax errors in integration tests ([#184](https://github.com/openkcm/session-manager/issues/184)) ([3b4ed57](https://github.com/openkcm/session-manager/commit/3b4ed57ff9ebc11b6af46b446e501b768c309661))

## [0.14.0](https://github.com/openkcm/session-manager/compare/v0.13.4...v0.14.0) (2025-12-17)


### Features

* use tenant specific cookies ([#180](https://github.com/openkcm/session-manager/issues/180)) ([0c47910](https://github.com/openkcm/session-manager/commit/0c47910645d55971280ec0bcb7763361d9b59ffb))


### Bug Fixes

* token introspection: deny if invalid ([#182](https://github.com/openkcm/session-manager/issues/182)) ([60db523](https://github.com/openkcm/session-manager/commit/60db52309469248fc8ef389e1a4891f8f728873b))

## [0.13.4](https://github.com/openkcm/session-manager/compare/v0.13.3...v0.13.4) (2025-12-16)


### Bug Fixes

* token refresh not using WKOC endpoint ([#179](https://github.com/openkcm/session-manager/issues/179)) ([4f7939e](https://github.com/openkcm/session-manager/commit/4f7939e582d799a60fe03dbbd658ee02d6f22bda))

## [0.13.3](https://github.com/openkcm/session-manager/compare/v0.13.2...v0.13.3) (2025-12-15)


### Bug Fixes

* use the default http client for the WKOC endpoint ([#176](https://github.com/openkcm/session-manager/issues/176)) ([4571e47](https://github.com/openkcm/session-manager/commit/4571e4719439e9871aa88bc1f0df1cfb7df5e5c3))

## [0.13.2](https://github.com/openkcm/session-manager/compare/v0.13.1...v0.13.2) (2025-12-15)


### Bug Fixes

* the round tripper to actually add the client_id ([#174](https://github.com/openkcm/session-manager/issues/174)) ([de06e4f](https://github.com/openkcm/session-manager/commit/de06e4fc7b55f0fdacbb90c7cfb3b9ca466cad3d))

## [0.13.1](https://github.com/openkcm/session-manager/compare/v0.13.0...v0.13.1) (2025-12-08)


### Bug Fixes

* client ID handling ([#166](https://github.com/openkcm/session-manager/issues/166)) ([bcde51b](https://github.com/openkcm/session-manager/commit/bcde51b0a93b23c2ebfb0aaa4993284d463dd915))

## [0.13.0](https://github.com/openkcm/session-manager/compare/v0.12.3...v0.13.0) (2025-12-08)


### Features

* cleanup idle sessions ([#156](https://github.com/openkcm/session-manager/issues/156)) ([6e125e0](https://github.com/openkcm/session-manager/commit/6e125e03d794d135a3d45062268de5581509bad9))


### Bug Fixes

* deny session on fingerprint mismatch ([#154](https://github.com/openkcm/session-manager/issues/154)) ([b24d4e5](https://github.com/openkcm/session-manager/commit/b24d4e57cb3e51f163647c19b9fdf9aa9c3b935f))
* Remove invalid golangci-lint config ([#161](https://github.com/openkcm/session-manager/issues/161)) ([4b4bc3f](https://github.com/openkcm/session-manager/commit/4b4bc3f05920eaeed36a520a519f1ef88ede5a66))
* token introspection ([#155](https://github.com/openkcm/session-manager/issues/155)) ([c4a32a6](https://github.com/openkcm/session-manager/commit/c4a32a6c7f57b829904acdfc5a20ab70561162f1))

## [0.12.3](https://github.com/openkcm/session-manager/compare/v0.12.2...v0.12.3) (2025-12-01)


### Bug Fixes

* add missing `sid` claim as provider session ID ([#151](https://github.com/openkcm/session-manager/issues/151)) ([e39b88f](https://github.com/openkcm/session-manager/commit/e39b88fb1c868c7524b167bf441995bbab6c2cd3))

## [0.12.2](https://github.com/openkcm/session-manager/compare/v0.12.1...v0.12.2) (2025-11-28)


### Bug Fixes

* disable token introspection ([#148](https://github.com/openkcm/session-manager/issues/148)) ([03492dc](https://github.com/openkcm/session-manager/commit/03492dc01b09a19049312b132a8855dc256f9790))

## [0.12.1](https://github.com/openkcm/session-manager/compare/v0.12.0...v0.12.1) (2025-11-28)


### Bug Fixes

* debug fingerprinting ([#145](https://github.com/openkcm/session-manager/issues/145)) ([42adbba](https://github.com/openkcm/session-manager/commit/42adbba45a748c3e0e86b0df3325ac500ab68949))

## [0.12.0](https://github.com/openkcm/session-manager/compare/v0.11.0...v0.12.0) (2025-11-27)


### Features

* implement remove ODIC mapping ([61c088a](https://github.com/openkcm/session-manager/commit/61c088a3ee228a4b923bf2cbf903d316337946b9))
* implement remove OIDC mapping ([#140](https://github.com/openkcm/session-manager/issues/140)) ([61c088a](https://github.com/openkcm/session-manager/commit/61c088a3ee228a4b923bf2cbf903d316337946b9))


### Bug Fixes

* configmap hook weight ([#141](https://github.com/openkcm/session-manager/issues/141)) ([55a06ef](https://github.com/openkcm/session-manager/commit/55a06efd477c4f6da29b458fa3a1fb55f381a792))
* migration script ([#142](https://github.com/openkcm/session-manager/issues/142)) ([c775ebb](https://github.com/openkcm/session-manager/commit/c775ebb5c90d93741cb7fc52e709358f26d0d813))

## [0.11.0](https://github.com/openkcm/session-manager/compare/v0.10.1...v0.11.0) (2025-11-27)


### Features

* implement the gRPC session service ([#131](https://github.com/openkcm/session-manager/issues/131)) ([4361b20](https://github.com/openkcm/session-manager/commit/4361b20c8ad489f259f51051e0d23ab28c66ad73))

## [0.10.1](https://github.com/openkcm/session-manager/compare/v0.10.0...v0.10.1) (2025-11-25)


### Bug Fixes

* cookie configuration and config loading ([#132](https://github.com/openkcm/session-manager/issues/132)) ([0b34e08](https://github.com/openkcm/session-manager/commit/0b34e08a538d244ac6effc5e2f80b7d08e1ddd12))

## [0.10.0](https://github.com/openkcm/session-manager/compare/v0.9.10...v0.10.0) (2025-11-25)


### Features

* make cookies configurable ([#124](https://github.com/openkcm/session-manager/issues/124)) ([7705585](https://github.com/openkcm/session-manager/commit/77055855073549ad4aa249ad4dfd3ce7db4eacc2))

## [0.9.10](https://github.com/openkcm/session-manager/compare/v0.9.9...v0.9.10) (2025-11-24)


### Bug Fixes

* include missing claims ([#125](https://github.com/openkcm/session-manager/issues/125)) ([c89d76e](https://github.com/openkcm/session-manager/commit/c89d76e383ab6fff6524b115f3e96e82c2e441e8))

## [0.9.9](https://github.com/openkcm/session-manager/compare/v0.9.8...v0.9.9) (2025-11-24)


### Bug Fixes

* define the CSRF cookie without the `__Host-` prefix ([#121](https://github.com/openkcm/session-manager/issues/121)) ([2dc708e](https://github.com/openkcm/session-manager/commit/2dc708ef3c913890093f4558819334c8ae5e10a0))

## [0.9.8](https://github.com/openkcm/session-manager/compare/v0.9.7...v0.9.8) (2025-11-21)


### Bug Fixes

* reintroduce the redirectURL as deprecated ([#119](https://github.com/openkcm/session-manager/issues/119)) ([7c16c5b](https://github.com/openkcm/session-manager/commit/7c16c5bfe3406f9ee376ae2cb2fd23cad41ffadf))

## [0.9.7](https://github.com/openkcm/session-manager/compare/v0.9.6...v0.9.7) (2025-11-21)


### Bug Fixes

* set the cookies for the correct domain ([#117](https://github.com/openkcm/session-manager/issues/117)) ([8cd8f5c](https://github.com/openkcm/session-manager/commit/8cd8f5c59c9d93124edd7fe6ea9aee538a447f99))

## [0.9.6](https://github.com/openkcm/session-manager/compare/v0.9.5...v0.9.6) (2025-11-21)


### Bug Fixes

* set the cookies for the right domain ([#115](https://github.com/openkcm/session-manager/issues/115)) ([b7b0112](https://github.com/openkcm/session-manager/commit/b7b01124745887ed1f4456304ca262d3ce6fbc23))

## [0.9.5](https://github.com/openkcm/session-manager/compare/v0.9.4...v0.9.5) (2025-11-18)


### Bug Fixes

* create cookies with SameSite: None ([#112](https://github.com/openkcm/session-manager/issues/112)) ([e55f39a](https://github.com/openkcm/session-manager/commit/e55f39a80feb83baac4acd9719d1f3e9f85bb0ed))

## [0.9.4](https://github.com/openkcm/session-manager/compare/v0.9.3...v0.9.4) (2025-11-17)


### Bug Fixes

* redirect get parameter ([#110](https://github.com/openkcm/session-manager/issues/110)) ([d6c003a](https://github.com/openkcm/session-manager/commit/d6c003abd7cee03d76a4fb439126a09ea7f634d3))

## [0.9.3](https://github.com/openkcm/session-manager/compare/v0.9.2...v0.9.3) (2025-11-17)


### Bug Fixes

* apply OIDC mapping response ([0342419](https://github.com/openkcm/session-manager/commit/03424194884165803eb64dc7633fb5458b4b0d33)), closes [#107](https://github.com/openkcm/session-manager/issues/107)
* separate callback and redirect to properly set cookies ([#108](https://github.com/openkcm/session-manager/issues/108)) ([40742e5](https://github.com/openkcm/session-manager/commit/40742e58fbe682bed18c9177a496740b40305cc4))

## [0.9.2](https://github.com/openkcm/session-manager/compare/v0.9.1...v0.9.2) (2025-11-17)


### Bug Fixes

* cookie handling with OpenAPI ([#104](https://github.com/openkcm/session-manager/issues/104)) ([2996a3b](https://github.com/openkcm/session-manager/commit/2996a3b9b4b1c8e3a0a9c8a535596ea0689b384e))

## [0.9.1](https://github.com/openkcm/session-manager/compare/v0.9.0...v0.9.1) (2025-11-14)


### Bug Fixes

* set the cookies for the correct domain ([#101](https://github.com/openkcm/session-manager/issues/101)) ([df8f896](https://github.com/openkcm/session-manager/commit/df8f89626860f10a47bd6741c47e4b4e3a276c1e))

## [0.9.0](https://github.com/openkcm/session-manager/compare/v0.8.0...v0.9.0) (2025-11-11)


### Features

* define auth context in session ([#96](https://github.com/openkcm/session-manager/issues/96)) ([8a79b01](https://github.com/openkcm/session-manager/commit/8a79b0101a7d0d8151ef58c9f0d96a77af5f3078))
* make oidc mapping removal idempotent ([5906e27](https://github.com/openkcm/session-manager/commit/5906e277b9da176c36b500fb0e36a4e60ddf4a36)), closes [#95](https://github.com/openkcm/session-manager/issues/95)

## [0.8.0](https://github.com/openkcm/session-manager/compare/v0.7.0...v0.8.0) (2025-11-06)


### Features

* add custom parameters to Provider ([#88](https://github.com/openkcm/session-manager/issues/88)) ([fbf8a40](https://github.com/openkcm/session-manager/commit/fbf8a400b4d30a32a7f206089d74fa2e1e6c2154))
* add valkey implementation ([#65](https://github.com/openkcm/session-manager/issues/65)) ([0486736](https://github.com/openkcm/session-manager/commit/04867366aa5442bf9ef3d20ad7440334279e73da))

## [0.7.0](https://github.com/openkcm/session-manager/compare/v0.6.0...v0.7.0) (2025-11-05)


### Features

* configure the clientID as plain string ([#84](https://github.com/openkcm/session-manager/issues/84)) ([be8eca3](https://github.com/openkcm/session-manager/commit/be8eca371ab597bc2d18212d42404486b65bceda))
* store raw JSON claims from the ID token in the session ([#82](https://github.com/openkcm/session-manager/issues/82)) ([3acf92e](https://github.com/openkcm/session-manager/commit/3acf92ede3d3505ee4805d1a7bcf192e6fc985af))


### Bug Fixes

* Handle null values for optional OIDC columns ([#83](https://github.com/openkcm/session-manager/issues/83)) ([e15fec9](https://github.com/openkcm/session-manager/commit/e15fec948702089ee9807a8d57370edfd0fbb932))

## [0.6.0](https://github.com/openkcm/session-manager/compare/v0.5.0...v0.6.0) (2025-11-03)


### Features

* add block/unblock OIDC mapping endpoints  ([23050ec](https://github.com/openkcm/session-manager/commit/23050ecd2617bc5ed1685640a7d369ebfa7780d5))


### Bug Fixes

* scopes URL encoding ([#79](https://github.com/openkcm/session-manager/issues/79)) ([8807451](https://github.com/openkcm/session-manager/commit/88074514157a1b70ef8a448507e824f4c1738afd))

## [0.5.0](https://github.com/openkcm/session-manager/compare/v0.4.0...v0.5.0) (2025-10-30)


### Features

* support client authentication ([#73](https://github.com/openkcm/session-manager/issues/73)) ([8209b0b](https://github.com/openkcm/session-manager/commit/8209b0bd543faff965c39b197436fe834e0367c0))


### Bug Fixes

* closed valkey connection ([#70](https://github.com/openkcm/session-manager/issues/70)) ([70aba4b](https://github.com/openkcm/session-manager/commit/70aba4b2c69b15249625288dd5bff8cb0bf5eca8))

## [0.4.0](https://github.com/openkcm/session-manager/compare/v0.3.0...v0.4.0) (2025-10-29)


### Features

* support configurable gRPC reflection ([#62](https://github.com/openkcm/session-manager/issues/62)) ([77f0715](https://github.com/openkcm/session-manager/commit/77f0715dba3309e1234e1f3b77c49ee119aa9bd7))


### Bug Fixes

* /sm http endpoint mapping ([#66](https://github.com/openkcm/session-manager/issues/66)) ([21d6f5c](https://github.com/openkcm/session-manager/commit/21d6f5c2db5004686833adc2e3b3a4cc59ea5041))

## [0.3.0](https://github.com/openkcm/session-manager/compare/v0.2.1...v0.3.0) (2025-10-23)


### Features

* valkey mTLS authentication ([#57](https://github.com/openkcm/session-manager/issues/57)) ([d4fb525](https://github.com/openkcm/session-manager/commit/d4fb525f3b0844d890bd418ca4e890fb296ff674))


### Bug Fixes

* **charts:** expose grpc port ([#51](https://github.com/openkcm/session-manager/issues/51)) ([4147699](https://github.com/openkcm/session-manager/commit/41476995bd28054204573147a4ced88af8cb442a))
* CSRF token ([#56](https://github.com/openkcm/session-manager/issues/56)) ([c9e466e](https://github.com/openkcm/session-manager/commit/c9e466e8a24ca42b5353bca4076573dc420fe91a))

## [0.2.1](https://github.com/openkcm/session-manager/compare/v0.2.0...v0.2.1) (2025-10-21)


### Bug Fixes

* **charts:** render issues ([#48](https://github.com/openkcm/session-manager/issues/48)) ([1ab8109](https://github.com/openkcm/session-manager/commit/1ab81095c2e6704a8c1184a51b89bc636380b223))

## [0.2.0](https://github.com/openkcm/session-manager/compare/v1.0.1...v0.2.0) (2025-10-17)


### ⚠ BREAKING CHANGES

* single CLI ([#43](https://github.com/openkcm/session-manager/issues/43))

### Features

* add audit logger ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* add openAPI base ([#4](https://github.com/openkcm/session-manager/issues/4)) ([33bd921](https://github.com/openkcm/session-manager/commit/33bd9215b7f83d7a94691aa39f9164a4ab45b43a))
* add sql example integration ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* add sql example integration ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* add sql example integration ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* add sql example integration ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* add sql integration ([#6](https://github.com/openkcm/session-manager/issues/6)) ([9cc706c](https://github.com/openkcm/session-manager/commit/9cc706c14f61e47fbc9982b60fadb90dfb4271c6))
* add stub auth endpoint implementation ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* add stub auth endpoint implementation ([#5](https://github.com/openkcm/session-manager/issues/5)) ([02c9a87](https://github.com/openkcm/session-manager/commit/02c9a87ff5043e89d4c1f950c3272052d4674530))
* add stubs for gRPC services ([#25](https://github.com/openkcm/session-manager/issues/25)) ([cb782aa](https://github.com/openkcm/session-manager/commit/cb782aa099c4c7ca39c7fdae0d44260f0112d6cc))
* audit logger ([#10](https://github.com/openkcm/session-manager/issues/10)) ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* charts and makefile ([#7](https://github.com/openkcm/session-manager/issues/7)) ([bcf0528](https://github.com/openkcm/session-manager/commit/bcf05286410e3e18a78c1f5429a9d5cd1b653a93))
* expose the session package ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* goose integration ([#28](https://github.com/openkcm/session-manager/issues/28)) ([f07cdf6](https://github.com/openkcm/session-manager/commit/f07cdf690f3b5be24560be91f7c95481506041c5))
* grpc server ([#33](https://github.com/openkcm/session-manager/issues/33)) ([9055c31](https://github.com/openkcm/session-manager/commit/9055c31b0557c46acff427f4fd0701192c01950d))
* valkey integration ([#8](https://github.com/openkcm/session-manager/issues/8)) ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* verify id token ([#38](https://github.com/openkcm/session-manager/issues/38)) ([c661dd7](https://github.com/openkcm/session-manager/commit/c661dd76b760ecd675d96734c966e40be708da1e))


### Bug Fixes

* bump chart version ([#45](https://github.com/openkcm/session-manager/issues/45)) ([079b9a7](https://github.com/openkcm/session-manager/commit/079b9a73bedd41b18209025aeaf31e2cd743e863))
* bump github.com/openkcm/common-sdk from 1.4.3 to 1.4.5 ([#27](https://github.com/openkcm/session-manager/issues/27)) ([4daeb9e](https://github.com/openkcm/session-manager/commit/4daeb9e655428207da75dfa995ec8b679943be4f))
* **chart:** http port binding ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* **config:** fix config loading ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* **deps:** bump github.com/envoyproxy/go-control-plane/envoy ([#26](https://github.com/openkcm/session-manager/issues/26)) ([05eb019](https://github.com/openkcm/session-manager/commit/05eb019a5f3acf6197aec5ac96cfd8f20b587f23))
* general fixes ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* general fixes ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* get rid of embedded build version, use ldflags instead ([#11](https://github.com/openkcm/session-manager/issues/11)) ([e01df28](https://github.com/openkcm/session-manager/commit/e01df285954e60d0e04a9ad2325125f63ab4c77b))
* imports ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* imports ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* imports ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* imports ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* rebase artifacts ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* rebase errors ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* rebase errors ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* Session Model ([#20](https://github.com/openkcm/session-manager/issues/20)) ([0de6f3a](https://github.com/openkcm/session-manager/commit/0de6f3a3f295255bd36c2d676b090250051b59c7))
* session.stateID -&gt; sessionID ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* test ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* the test tags ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* update the time ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* use common-sdk for grpc server ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* use common-sdk for grpc server ([f8f6093](https://github.com/openkcm/session-manager/commit/f8f60938d8c353a70ebebc1999f9325f63ec1ea5))
* use common-sdk for grpc server ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* use common-sdk for grpc server ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))
* valkey tests ([1296663](https://github.com/openkcm/session-manager/commit/1296663b74d49f499e13cb4db90acf23b982ce2e))


### Miscellaneous Chores

* release 0.1.1 ([f357645](https://github.com/openkcm/session-manager/commit/f35764522c21808a3901b3d0e1bf4d5d8fb7ab5c))
* reset version to 0.2.0 ([0cbe7cb](https://github.com/openkcm/session-manager/commit/0cbe7cbe4ee5ade9da6cdd6ee4cf0a6aeb382c9a))


### Code Refactoring

* single CLI ([#43](https://github.com/openkcm/session-manager/issues/43)) ([b4f70ee](https://github.com/openkcm/session-manager/commit/b4f70eee9eefe856844b6f4dc0960bbee3502879))

## [1.0.1](https://github.com/openkcm/session-manager/compare/v1.0.0...v1.0.1) (2025-10-17)


### Bug Fixes

* bump chart version ([#45](https://github.com/openkcm/session-manager/issues/45)) ([079b9a7](https://github.com/openkcm/session-manager/commit/079b9a73bedd41b18209025aeaf31e2cd743e863))

## [1.0.0](https://github.com/openkcm/session-manager/compare/v0.1.1...v1.0.0) (2025-10-17)


### ⚠ BREAKING CHANGES

* single CLI ([#43](https://github.com/openkcm/session-manager/issues/43))

### Code Refactoring

* single CLI ([#43](https://github.com/openkcm/session-manager/issues/43)) ([b4f70ee](https://github.com/openkcm/session-manager/commit/b4f70eee9eefe856844b6f4dc0960bbee3502879))

## [0.1.1](https://github.com/openkcm/session-manager/compare/v0.1.0...v0.1.1) (2025-10-16)


### Miscellaneous Chores

* release 0.1.1 ([f357645](https://github.com/openkcm/session-manager/commit/f35764522c21808a3901b3d0e1bf4d5d8fb7ab5c))

## [0.1.0](https://github.com/openkcm/session-manager/compare/v0.0.1...v0.1.0) (2025-10-16)


### Features

* add stubs for gRPC services ([#25](https://github.com/openkcm/session-manager/issues/25)) ([cb782aa](https://github.com/openkcm/session-manager/commit/cb782aa099c4c7ca39c7fdae0d44260f0112d6cc))
* goose integration ([#28](https://github.com/openkcm/session-manager/issues/28)) ([f07cdf6](https://github.com/openkcm/session-manager/commit/f07cdf690f3b5be24560be91f7c95481506041c5))
* grpc server ([#33](https://github.com/openkcm/session-manager/issues/33)) ([9055c31](https://github.com/openkcm/session-manager/commit/9055c31b0557c46acff427f4fd0701192c01950d))
* verify id token ([#38](https://github.com/openkcm/session-manager/issues/38)) ([c661dd7](https://github.com/openkcm/session-manager/commit/c661dd76b760ecd675d96734c966e40be708da1e))


### Bug Fixes

* bump github.com/openkcm/common-sdk from 1.4.3 to 1.4.5 ([#27](https://github.com/openkcm/session-manager/issues/27)) ([4daeb9e](https://github.com/openkcm/session-manager/commit/4daeb9e655428207da75dfa995ec8b679943be4f))
* **deps:** bump github.com/envoyproxy/go-control-plane/envoy ([#26](https://github.com/openkcm/session-manager/issues/26)) ([05eb019](https://github.com/openkcm/session-manager/commit/05eb019a5f3acf6197aec5ac96cfd8f20b587f23))
* Session Model ([#20](https://github.com/openkcm/session-manager/issues/20)) ([0de6f3a](https://github.com/openkcm/session-manager/commit/0de6f3a3f295255bd36c2d676b090250051b59c7))
