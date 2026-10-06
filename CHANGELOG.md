# [1.11.0](https://github.com/theBenForce/shelfd/compare/v1.10.1...v1.11.0) (2026-10-06)


### Features

* **utilities:** support multi-format book merge with companion file retention ([#48](https://github.com/theBenForce/shelfd/issues/48)) ([7b323f0](https://github.com/theBenForce/shelfd/commit/7b323f0727f938571acae1ca6383550d40ee2a70))

## [1.10.1](https://github.com/theBenForce/shelfd/compare/v1.10.0...v1.10.1) (2026-10-04)


### Bug Fixes

* **scanner,repo:** normalize author names and fix multi-file audiobook title resolution ([#47](https://github.com/theBenForce/shelfd/issues/47)) ([88f0ba4](https://github.com/theBenForce/shelfd/commit/88f0ba4f58c71afc2b9f4c5dc6242268f3220287))

# [1.10.0](https://github.com/theBenForce/shelfd/compare/v1.9.0...v1.10.0) (2026-10-04)


### Bug Fixes

* **app:** replace non-rendering rounded icon variants in audiobook player with standard Material icons ([#44](https://github.com/theBenForce/shelfd/issues/44)) ([ea85c9f](https://github.com/theBenForce/shelfd/commit/ea85c9fbfc7a0180f664f98b5b9b64323b2136b9))
* **test:** mock audio player engine in audiobook ui widget tests ([#46](https://github.com/theBenForce/shelfd/issues/46)) ([daf9ed6](https://github.com/theBenForce/shelfd/commit/daf9ed6c76944bf6715878e4261ea36a7713bde0))


### Features

* **audio:** support multi-file audiobooks with directory track aggregation ([#45](https://github.com/theBenForce/shelfd/issues/45)) ([ce435d7](https://github.com/theBenForce/shelfd/commit/ce435d7cb1f6c739060916d1c42062dcb1ce5393))
* **upload:** confirm duplicate book uploads and sync edited metadata ([#43](https://github.com/theBenForce/shelfd/issues/43)) ([de37f53](https://github.com/theBenForce/shelfd/commit/de37f53f23930d330f6912bda0bd2c969d6a6e5f))

# [1.9.0](https://github.com/theBenForce/shelfd/compare/v1.8.0...v1.9.0) (2026-10-03)


### Features

* **app:** add audiobook player core state and audio repository ([#41](https://github.com/theBenForce/shelfd/issues/41)) ([1b7cc3a](https://github.com/theBenForce/shelfd/commit/1b7cc3a2289635c90fc7121b25ac120b9fc7a045))
* **app:** add audiobook player ui and associated files section ([#42](https://github.com/theBenForce/shelfd/issues/42)) ([739f667](https://github.com/theBenForce/shelfd/commit/739f66786bd7472c96bfa329f2d294019e5218aa))
* **server:** add media format schema and audio metadata scanner ([#38](https://github.com/theBenForce/shelfd/issues/38)) ([221ae27](https://github.com/theBenForce/shelfd/commit/221ae27eb3edcc5e64a2406413d6db9af56bdce6))
* **server:** implement audio streaming and progress sync APIs ([#39](https://github.com/theBenForce/shelfd/issues/39)) ([2dc2e82](https://github.com/theBenForce/shelfd/commit/2dc2e82071cb4a8fde3f994ef0a572d6dc681c56))

# [1.8.0](https://github.com/theBenForce/shelfd/compare/v1.7.0...v1.8.0) (2026-10-02)


### Bug Fixes

* **epub:** parse toc ncx and nav documents for chapter titles ([#31](https://github.com/theBenForce/shelfd/issues/31)) ([fb7760d](https://github.com/theBenForce/shelfd/commit/fb7760dffc172138dd191a8dc8d53caa16657190))


### Features

* **api:** add CLI OAuth flow support with title and ISBN search ([#33](https://github.com/theBenForce/shelfd/issues/33)) ([3567b53](https://github.com/theBenForce/shelfd/commit/3567b531a982e21fb6c6ef6f5726df8d8046a1e0))
* **utilities:** add utilities page with duplicate books detection and merge ([#32](https://github.com/theBenForce/shelfd/issues/32)) ([ac8d4f2](https://github.com/theBenForce/shelfd/commit/ac8d4f20d75b4700d4f6cbe2d83dcb4f7fb50647))

# [1.7.0](https://github.com/theBenForce/shelfd/compare/v1.6.0...v1.7.0) (2026-09-30)


### Features

* book metadata editor with EPUB in-place sync and database updates ([#30](https://github.com/theBenForce/shelfd/issues/30)) ([028ab5c](https://github.com/theBenForce/shelfd/commit/028ab5c6f5f3c70e60c55ecd4c791ba24c3ca54e))

# [1.6.0](https://github.com/theBenForce/shelfd/compare/v1.5.2...v1.6.0) (2026-09-30)


### Features

* **book-detail:** add EPUB download button on book details view ([#29](https://github.com/theBenForce/shelfd/issues/29)) ([7281cb9](https://github.com/theBenForce/shelfd/commit/7281cb92b42dcb4253f6af0f8cc351a63d75e00d))

## [1.5.2](https://github.com/theBenForce/shelfd/compare/v1.5.1...v1.5.2) (2026-09-21)


### Bug Fixes

* **database:** disable socket read timeout on postgres connections ([#28](https://github.com/theBenForce/shelfd/issues/28)) ([0a439a4](https://github.com/theBenForce/shelfd/commit/0a439a4d8de151ec47f9cb78c89912bf486b5b55))

## [1.5.1](https://github.com/theBenForce/shelfd/compare/v1.5.0...v1.5.1) (2026-09-21)


### Performance Improvements

* **server:** optimize unindexed paragraph queries and add worker backoff ([#27](https://github.com/theBenForce/shelfd/issues/27)) ([d2f56c3](https://github.com/theBenForce/shelfd/commit/d2f56c37933913337d2289dd319adff9888192b5))

# [1.5.0](https://github.com/theBenForce/shelfd/compare/v1.4.0...v1.5.0) (2026-09-12)


### Features

* authenticate media requests via cookie, allow cover replacement, and fix web folder picker ([#15](https://github.com/theBenForce/shelfd/issues/15)) ([d33388f](https://github.com/theBenForce/shelfd/commit/d33388fe50f4131b19ed14c6224d2551f8145da9))

# [1.4.0](https://github.com/theBenForce/shelfd/compare/v1.3.1...v1.4.0) (2026-09-11)


### Bug Fixes

* **epub:** permit root directory entry and leading slash in zip slip validation ([#7](https://github.com/theBenForce/shelfd/issues/7)) ([#10](https://github.com/theBenForce/shelfd/issues/10)) ([86fa69f](https://github.com/theBenForce/shelfd/commit/86fa69fcf5ecf704fdd58a60be6854bdc906e2a4))
* **epub:** recover from malformed line-1 xml declarations in container and opf ([#8](https://github.com/theBenForce/shelfd/issues/8)) ([#11](https://github.com/theBenForce/shelfd/issues/11)) ([53abd53](https://github.com/theBenForce/shelfd/commit/53abd535223217a032c078b39915e4c1b93dc970))


### Features

* revamp library view with live search, tag autocomplete, and genre/topic navigation ([#14](https://github.com/theBenForce/shelfd/issues/14)) ([bcc0a97](https://github.com/theBenForce/shelfd/commit/bcc0a976df64fd19c4fdff05dd43d75741408143))
* staged uploads review queue, recursive folder drop, and metadata inspector ([#13](https://github.com/theBenForce/shelfd/issues/13)) ([51c4b61](https://github.com/theBenForce/shelfd/commit/51c4b61d07418c98ab29fc9d2075b314e88c91cc))

## [1.3.1](https://github.com/theBenForce/shelfd/compare/v1.3.0...v1.3.1) (2026-09-11)


### Bug Fixes

* **mcp:** improve search tool usability, taxonomy filtering, and topics architecture ([#6](https://github.com/theBenForce/shelfd/issues/6)) ([3235aee](https://github.com/theBenForce/shelfd/commit/3235aeec6f5ea74664aee118faa27c095663f20d)), closes [#5](https://github.com/theBenForce/shelfd/issues/5)

# [1.3.0](https://github.com/theBenForce/shelfd/compare/v1.2.2...v1.3.0) (2026-09-10)


### Features

* **mcp:** support streamable http transport and rfc 9728 protected resource metadata ([#4](https://github.com/theBenForce/shelfd/issues/4)) ([3a41a4e](https://github.com/theBenForce/shelfd/commit/3a41a4ee9f8c0430573b5e9d969844042404e326))

## [1.2.2](https://github.com/theBenForce/shelfd/compare/v1.2.1...v1.2.2) (2026-09-10)


### Bug Fixes

* **oauth:** fix client lookup query alias and update docker image name ([#3](https://github.com/theBenForce/shelfd/issues/3)) ([913f3fa](https://github.com/theBenForce/shelfd/commit/913f3fadcc27be244464e7aaf6e555ddff86ece3))

## [1.2.1](https://github.com/theBenForce/shelfd/compare/v1.2.0...v1.2.1) (2026-09-10)


### Bug Fixes

* **oauth:** resolve postgres table mapping and enable cors on discovery ([#2](https://github.com/theBenForce/shelfd/issues/2)) ([196ccb0](https://github.com/theBenForce/shelfd/commit/196ccb0289b72e47742a74fd4494fa079d4736b2))

# [1.2.0](https://github.com/theBenForce/shelved/compare/v1.1.1...v1.2.0) (2026-09-10)


### Features

* pipeline library scanning and stream real-time book ingestion events ([309b345](https://github.com/theBenForce/shelved/commit/309b3452a2e7c17245ec82f9be606ecb96cbcda2))
* realtime library scanning ([9bfaac1](https://github.com/theBenForce/shelved/commit/9bfaac18dfc926a697ad9b2d6e3e8b035ff6cdfa))

## [1.1.1](https://github.com/theBenForce/shelved/compare/v1.1.0...v1.1.1) (2026-09-10)


### Bug Fixes

* auto-detect deployed domain origin and make server url editable in web client ([d3d2bf4](https://github.com/theBenForce/shelved/commit/d3d2bf4520a03bbf29af5e8be8ce12f59f0e614c))

# [1.1.0](https://github.com/theBenForce/shelved/compare/v1.0.0...v1.1.0) (2026-09-10)


### Bug Fixes

* **queue:** track indexing progress from paragraph vectors instead of chapter summaries ([df53f3b](https://github.com/theBenForce/shelved/commit/df53f3b62e80026f18d41a883fffcab29482e0e1))
* support per_page/page pagination and load complete catalog on shelf ([95af430](https://github.com/theBenForce/shelved/commit/95af430b93cf74648053a269d19915ee10d5dcae))


### Features

* add epub drag-and-drop upload with metadata editor and opf package rewriter ([53c8515](https://github.com/theBenForce/shelved/commit/53c8515f0c13b7d06f494eed94169d2f9ba75ee9))
* adopt log/slog structured logging and http request logger middleware ([b81fd6d](https://github.com/theBenForce/shelved/commit/b81fd6d8a12687dce0873900a6f31647db95ff46))
* **app:** add homelab settings view, user password management, and reader selection improvements ([7ff3cd5](https://github.com/theBenForce/shelved/commit/7ff3cd584658dda8ff32d8792511f1ce98aac48d))
* **app:** implement infinite scroll pagination in library view ([525d738](https://github.com/theBenForce/shelved/commit/525d738f22051b6164cd3b5d5f25ab77c567ca53))
* **app:** retain desktop sidebar navigation on series, author, and book detail pages ([3359e0c](https://github.com/theBenForce/shelved/commit/3359e0c3dbb4490c2afee1abec08885634977a42))
* **app:** standardize plural routes, collapse series in library, and add library accordion nav ([bd4daf0](https://github.com/theBenForce/shelved/commit/bd4daf0107ec52f1c676595e9f3e5cd0c2984ae7))
* **monorepo:** add turbo dev scripts for @shelfd/server and @shelfd/app ([ea63fba](https://github.com/theBenForce/shelved/commit/ea63fbaa76adbc43953de38e38b2f2118d042f61))
* **reader:** add multi-paragraph highlight support and offline cross-device sync ([9216c45](https://github.com/theBenForce/shelved/commit/9216c45b12fdbff64c4837aa6a1d3f795f64db0e))
* **reader:** support fixed-layout EPUBs with responsive two-page spreads and auto-fit scaling ([d429d85](https://github.com/theBenForce/shelved/commit/d429d856d4e15eb23e1d909a18270d0468c33e08))
* **router:** use ShellRoute and nested subroutes for book, series, and author ([637494d](https://github.com/theBenForce/shelved/commit/637494d3fff216a5d4438021e13a46088c0d6103))
* **scanner:** add incremental library scanning and preserve vector embeddings ([e50b72b](https://github.com/theBenForce/shelved/commit/e50b72be59501a1a28496828b6649c14f3857fd9))
* **server:** add air live reload config, series normalization, and author directory fallback ([0b9113c](https://github.com/theBenForce/shelved/commit/0b9113c48f1842d1e2a04e688a81889a3a99ddf9))
* **server:** add dev script with watch mode and .env configuration ([f3dfc1e](https://github.com/theBenForce/shelved/commit/f3dfc1ec1d832aac5f4f0326510d7dd3a2fa9b9a))

# 1.0.0 (2026-09-09)


### Bug Fixes

* **app:** parse content_plain in Chapter.fromJson and bundle CanvasKit locally without CDN ([01ad690](https://github.com/theBenForce/shelved/commit/01ad690b7c1654dfcf00138c0826300b930b2db2))
* **deploy:** update dockerfile to go 1.25 and set embedding dimensions to 256 ([fca8119](https://github.com/theBenForce/shelved/commit/fca811988521891918de9688bd2ac03ca0694c40))
* **server:** resolve sqlite-vec shadow table conflict and eliminate hardcoded passwords ([5c0cc8f](https://github.com/theBenForce/shelved/commit/5c0cc8f8b79f1c0628fd1aa163534ffe98ea89da))


### Features

* add book details page with annotations and in-book rag chat ([f569745](https://github.com/theBenForce/shelved/commit/f56974585f98c2e60406fed3d6bf6467c335daf4))
* add Google Gemini AI provider and unified batch vector ingestion ([87e5377](https://github.com/theBenForce/shelved/commit/87e537762d44471e382b63c1db2e7142f29e063a))
* **agents:** define team personas, rules, and stitch designer manual ([6cb8dc8](https://github.com/theBenForce/shelved/commit/6cb8dc899cd7b7423d21736aa778e4258d734944))
* **ai:** implement semantic ingestion pipeline and hybrid vector search (milestone 3) ([5a3711b](https://github.com/theBenForce/shelved/commit/5a3711b9d7d59758b8f12974104335e49c29af81))
* **api:** implement rest api, jwt authentication, and mobile qr pairing (milestone 5) ([89ed381](https://github.com/theBenForce/shelved/commit/89ed381d31226a7eb3b0f8bb7aa5cb6c133f5c09))
* **app:** add desktop side navigation, Stitch logo SVG and converted favicon PNG ([6eed980](https://github.com/theBenForce/shelved/commit/6eed980b62d1c6ea98eff84a548e0cca2145d92d))
* **app:** add real-time queue status card in desktop sidebar and settings ([53399af](https://github.com/theBenForce/shelved/commit/53399afa623649c11bc1da6c8321d982c07d2e85))
* **app:** implement flutter reader client with stitch ui prototypes and dry shared widgets (milestone 6) ([e004e89](https://github.com/theBenForce/shelved/commit/e004e892f4f2df85dd01adbd8fbc806a0a2271bb))
* **app:** render markdown in book chat messages ([1a93dfc](https://github.com/theBenForce/shelved/commit/1a93dfc84375f268641f64f1f41107c8ea386e2a))
* **app:** render rich typography, headings, blockquotes, and inline footnotes in reader ([5d07404](https://github.com/theBenForce/shelved/commit/5d07404893d5fe9d028d4f3e5dcca35fcf73c622))
* **app:** strip and format html synopsis in book detail view ([109ce81](https://github.com/theBenForce/shelved/commit/109ce813ab7c348efa295e320ff174e8ba6e2b55))
* **app:** subscribe to real-time SSE queue events stream ([e7633c0](https://github.com/theBenForce/shelved/commit/e7633c0b8074221f333120479ecc10663c86b435))
* **app:** support spine manifest, dynamic chapter titles, and ULID navigation in reader ([f7b3e34](https://github.com/theBenForce/shelved/commit/f7b3e345e93123dc34f05ac8f77faff55fee769b))
* **ci:** add manual release pipeline with semantic-release and GHCR publishing ([af8be58](https://github.com/theBenForce/shelved/commit/af8be5802458ecafc787c946152a5a349829835a))
* **core:** implement config loader, migrations, and sqlite-vec repository (milestone 1) ([3eff212](https://github.com/theBenForce/shelved/commit/3eff21240686a103eb652e0e06ae479b7b53a016))
* **deploy:** add OAuth 2.0 dynamic registration, Traefik integration, and Turborepo pipeline ([9ace77a](https://github.com/theBenForce/shelved/commit/9ace77af2a818bf86e022d625fc66bfaa687aeaa))
* **docker:** implement multi-stage dockerfile, compose, and puid/pgid homelab packaging (milestone 7) ([c82bda4](https://github.com/theBenForce/shelved/commit/c82bda4525f059672fb2e6ebca0817106cbcf44a))
* **docker:** verify live homelab deployment with lm studio and chrome devtools ([d93cb59](https://github.com/theBenForce/shelved/commit/d93cb59a264606bafd35ecd84ce683517e44a99e))
* **epub:** implement safe epub parser, metadata extractor, and abs scanner (milestone 2) ([06cadab](https://github.com/theBenForce/shelved/commit/06cadab5dce256fab70509e669b07bfd674d368a))
* **mcp:** implement model context protocol server over http/sse (milestone 4) ([d4960d3](https://github.com/theBenForce/shelved/commit/d4960d320645c4c85a878f647586f8488d6a3926))
* **reader:** in-reader text selection highlighting, Kindle 4-color palette, notes, and location tracking ([2387ad5](https://github.com/theBenForce/shelved/commit/2387ad5a32990eb2ffdce8c3787b1e44c670c896))
* **scanner:** prioritize sibling cover and save extracted cover alongside epub ([fc5e46e](https://github.com/theBenForce/shelved/commit/fc5e46eeb67d4419d4dc1d43c608bf0f3eaeb0fd))
* **server:** add Bun and PostgreSQL compatibility to database layer ([1a3dc24](https://github.com/theBenForce/shelved/commit/1a3dc24eb850488beaed4a812f38b41347893f8e))
* **server:** add monotonic ULID generator ([b814295](https://github.com/theBenForce/shelved/commit/b814295c99aca00bf71b809314c9521834e5eeaf))
* **server:** add real-time queue status and SSE events endpoints ([9b64ecd](https://github.com/theBenForce/shelved/commit/9b64ecdac499c76176bf857c87a4fada93463afb))
* **server:** add SpineItem model, GetBookSpine query, and ULID chapter assignment ([1f32f57](https://github.com/theBenForce/shelved/commit/1f32f571f0807a303f736706508fc3873bd534a4))
* **server:** auto-backfill paragraphs on startup and scan trigger ([b57e22e](https://github.com/theBenForce/shelved/commit/b57e22e204ca7e99cdf1cb0cc22ad234959358a2))
* **server:** expose spine manifest, direct chapter route, and ULID chapter resolution ([58f853f](https://github.com/theBenForce/shelved/commit/58f853f1bfa1b5acead050c119e051041927d7b7))
* **server:** migrate to 256d per-paragraph vector embeddings and fts5 retrieval ([aaca837](https://github.com/theBenForce/shelved/commit/aaca83794d864de44e152b32b68d87efffaf4a0b))
* **server:** preserve EPUB formatting in Markdown and add book chapter reparse ([34dd317](https://github.com/theBenForce/shelved/commit/34dd31770915faa2952b4f18c9116ae5d0276d79))
* **storage:** implement BunStorageEngine for dual sqlite and postgres support with pgvector ([6d8dd67](https://github.com/theBenForce/shelved/commit/6d8dd67ef45503bd5ee804bbf7a247bf3e200b13))
* **upload:** implement asynchronous upload processing queue and status endpoints ([ddb7ce5](https://github.com/theBenForce/shelved/commit/ddb7ce57e180b5340a39cb25a095fc593c227ed2))
* **web:** bundle self-hosted webapp with host origin locking and library loading fixes ([667d32d](https://github.com/theBenForce/shelved/commit/667d32dac398899ca5c030efd82493b1bfd06f3f))
* **worker:** resolve and broadcast book title during paragraph indexing ([08884ae](https://github.com/theBenForce/shelved/commit/08884ae473722cb0d534bb91c97a2018ee4537dc))
