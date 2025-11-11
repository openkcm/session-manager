# Changelog

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
