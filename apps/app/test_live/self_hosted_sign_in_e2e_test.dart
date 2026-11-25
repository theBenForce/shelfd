import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/main.dart';
import 'package:shelf/platform/qr_scanner_service.dart';
import 'package:shelf/ui/core/shelfd_authenticated_image.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/connect/connect_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _LiveQrScannerService extends QrScannerService {
  final String? scanResult;

  _LiveQrScannerService({this.scanResult});

  @override
  Future<String?> scanQrCode() async => scanResult;

  @override
  Future<String> getDeviceModel() async => 'Pixel 9 Live E2E';
}

final Uint8List _oneByOnePng = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
);

int _crc32(List<int> bytes) {
  int crc = 0xFFFFFFFF;
  for (final byte in bytes) {
    crc ^= (byte & 0xFF);
    for (int bit = 0; bit < 8; bit++) {
      if ((crc & 1) != 0) {
        crc = (crc >>> 1) ^ 0xEDB88320;
      } else {
        crc = crc >>> 1;
      }
    }
  }
  return (crc ^ 0xFFFFFFFF) & 0xFFFFFFFF;
}

void _writeUint16LE(BytesBuilder out, int value) {
  out.addByte(value & 0xFF);
  out.addByte((value >>> 8) & 0xFF);
}

void _writeUint32LE(BytesBuilder out, int value) {
  out.addByte(value & 0xFF);
  out.addByte((value >>> 8) & 0xFF);
  out.addByte((value >>> 16) & 0xFF);
  out.addByte((value >>> 24) & 0xFF);
}

Uint8List _buildStoredZip(List<({String path, List<int> data})> entries) {
  final archive = BytesBuilder(copy: false);
  final centralDir = BytesBuilder(copy: false);

  for (final entry in entries) {
    final nameBytes = utf8.encode(entry.path);
    final payload = entry.data;
    final crc = _crc32(payload);
    final localOffset = archive.length;

    _writeUint32LE(archive, 0x04034B50);
    _writeUint16LE(archive, 20);
    _writeUint16LE(archive, 0);
    _writeUint16LE(archive, 0);
    _writeUint16LE(archive, 0);
    _writeUint16LE(archive, 0);
    _writeUint32LE(archive, crc);
    _writeUint32LE(archive, payload.length);
    _writeUint32LE(archive, payload.length);
    _writeUint16LE(archive, nameBytes.length);
    _writeUint16LE(archive, 0);
    archive.add(nameBytes);
    archive.add(payload);

    _writeUint32LE(centralDir, 0x02014B50);
    _writeUint16LE(centralDir, 20);
    _writeUint16LE(centralDir, 20);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint32LE(centralDir, crc);
    _writeUint32LE(centralDir, payload.length);
    _writeUint32LE(centralDir, payload.length);
    _writeUint16LE(centralDir, nameBytes.length);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint16LE(centralDir, 0);
    _writeUint32LE(centralDir, 0);
    _writeUint32LE(centralDir, localOffset);
    centralDir.add(nameBytes);
  }

  final centralOffset = archive.length;
  final centralBytes = centralDir.takeBytes();
  archive.add(centralBytes);

  _writeUint32LE(archive, 0x06054B50);
  _writeUint16LE(archive, 0);
  _writeUint16LE(archive, 0);
  _writeUint16LE(archive, entries.length);
  _writeUint16LE(archive, entries.length);
  _writeUint32LE(archive, centralBytes.length);
  _writeUint32LE(archive, centralOffset);
  _writeUint16LE(archive, 0);

  return archive.takeBytes();
}

Uint8List _buildMinimalFixedLayoutEpub() {
  const containerXml = '''<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>''';

  const contentOpf = '''<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:11111111-2222-3333-4444-555555555555</dc:identifier>
    <dc:title>Android Live Sign-In Specimen</dc:title>
    <dc:creator>Ada Lovelace</dc:creator>
    <dc:language>en</dc:language>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="rendition:spread">landscape</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="p1" href="xhtml/page01.xhtml" media-type="application/xhtml+xml"/>
    <item id="illo" href="images/illo.png" media-type="image/png"/>
  </manifest>
  <spine>
    <itemref idref="p1"/>
  </spine>
</package>''';

  const navXhtml = '''<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Navigation</title></head>
<body><nav epub:type="toc"><ol><li><a href="xhtml/page01.xhtml">Page 1</a></li></ol></nav></body>
</html>''';

  const pageOneXhtml = '''<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><meta name="viewport" content="width=600, height=800"/><title>Page 1</title></head>
<body>
  <img src="../images/illo.png" alt="Specimen"/>
  <span id="w1">Analytical</span>
  <span id="w2">Engine</span>
</body>
</html>''';

  return _buildStoredZip([
    (path: 'mimetype', data: ascii.encode('application/epub+zip')),
    (path: 'META-INF/container.xml', data: utf8.encode(containerXml)),
    (path: 'OEBPS/content.opf', data: utf8.encode(contentOpf)),
    (path: 'OEBPS/nav.xhtml', data: utf8.encode(navXhtml)),
    (path: 'OEBPS/xhtml/page01.xhtml', data: utf8.encode(pageOneXhtml)),
    (path: 'OEBPS/images/illo.png', data: _oneByOnePng),
  ]);
}

Future<int> _reserveFreePort() async {
  final socket = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
  final port = socket.port;
  await socket.close();
  return port;
}

File _resolveShelfdBinary() {
  final candidates = [
    File('../server/dist/shelfd'),
    File('apps/server/dist/shelfd'),
  ];
  for (final file in candidates) {
    if (file.existsSync()) {
      return file;
    }
  }
  throw StateError(
    'Standalone Shelfd server binary not found at ../server/dist/shelfd. '
    'Run `pnpm --filter @shelfd/server build` before running live E2E tests.',
  );
}

Future<void> _pumpLiveUntil(
  WidgetTester tester,
  bool Function() predicate, {
  Duration timeout = const Duration(seconds: 10),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (!predicate()) {
    if (DateTime.now().isAfter(deadline)) {
      fail('Timed out after ${timeout.inSeconds}s waiting for live server state');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 25)),
    );
    await tester.pump(const Duration(milliseconds: 25));
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides.global = null;

  late String liveBaseUrl;
  late String adminUsername;
  late String adminPassword;
  Process? spawnedServer;
  Directory? spawnedTempDir;

  setUpAll(() async {
    adminUsername =
        (Platform.environment['SHELFD_ADMIN_USERNAME'] ?? '').trim().isNotEmpty
            ? Platform.environment['SHELFD_ADMIN_USERNAME']!.trim()
            : 'admin';
    adminPassword =
        (Platform.environment['SHELFD_ADMIN_PASSWORD'] ?? '').isNotEmpty
            ? Platform.environment['SHELFD_ADMIN_PASSWORD']!
            : 'live-e2e-pass-123';

    final configuredUrl =
        (Platform.environment['SHELFD_LIVE_URL'] ?? '').trim();
    if (configuredUrl.isNotEmpty) {
      liveBaseUrl = configuredUrl.endsWith('/')
          ? configuredUrl.substring(0, configuredUrl.length - 1)
          : configuredUrl;
      return;
    }

    final binary = _resolveShelfdBinary();
    final port = await _reserveFreePort();
    final tempDir = Directory.systemTemp.createTempSync('shelfd-live-e2e-');
    spawnedTempDir = tempDir;

    final dataDir = Directory('${tempDir.path}/data')..createSync();
    final libraryDir = Directory('${tempDir.path}/library')..createSync();
    final pluginsDir = Directory('${tempDir.path}/plugins')..createSync();

    liveBaseUrl = 'http://127.0.0.1:$port';
    spawnedServer = await Process.start(
      binary.path,
      const [],
      environment: {
        ...Platform.environment,
        'SHELFD_SERVER_HOST': '127.0.0.1',
        'SHELFD_SERVER_PORT': '$port',
        'SHELFD_STORAGE_DATA_DIR': dataDir.path,
        'SHELFD_SQLITE_PATH': '${dataDir.path}/sqlite.db',
        'SHELFD_STORAGE_LIBRARY_DIR': libraryDir.path,
        'SHELFD_PLUGINS_DIR': pluginsDir.path,
        'SHELFD_CAST_ENABLED': 'false',
        'SHELFD_JWT_SECRET': 'live-e2e-jwt-secret-key-0123456789',
        'SHELFD_ADMIN_USERNAME': adminUsername,
        'SHELFD_ADMIN_PASSWORD': adminPassword,
      },
    );

    final healthUri = Uri.parse('$liveBaseUrl/health');
    final client = http.Client();
    try {
      bool ready = false;
      for (int attempt = 0; attempt < 60; attempt++) {
        try {
          final resp = await client
              .get(healthUri)
              .timeout(const Duration(milliseconds: 500));
          if (resp.statusCode == 200) {
            ready = true;
            break;
          }
        } catch (_) {}
        await Future<void>.delayed(const Duration(milliseconds: 200));
      }
      if (!ready) {
        throw StateError('Spawned Shelfd server failed health check at $healthUri');
      }
    } finally {
      client.close();
    }
  });

  tearDownAll(() async {
    spawnedServer?.kill(ProcessSignal.sigterm);
    if (spawnedServer != null) {
      await spawnedServer!.exitCode.timeout(
        const Duration(seconds: 5),
        onTimeout: () {
          spawnedServer?.kill(ProcessSignal.sigkill);
          return -1;
        },
      );
    }
    if (spawnedTempDir != null && spawnedTempDir!.existsSync()) {
      spawnedTempDir!.deleteSync(recursive: true);
    }
  });

  testWidgets(
      'manual URL normalization and ConnectView login issue revocable shelfd_ device token and unwrap getMe',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });

    SharedPreferences.setMockInitialValues({});
    final prefs = await tester.runAsync(SharedPreferences.getInstance);
    final storage = StorageService(
      prefs!,
      secureStorage: InMemorySecureTokenStorage(),
    );
    await tester.runAsync(storage.init);

    late WidgetRef capturedRef;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storage),
          qrScannerServiceProvider.overrideWithValue(_LiveQrScannerService()),
        ],
        child: Consumer(
          builder: (context, ref, _) {
            capturedRef = ref;
            return MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ConnectView(),
            );
          },
        ),
      ),
    );

    final parsedLive = Uri.parse(liveBaseUrl);
    final unnormalizedInput = '${parsedLive.host}:${parsedLive.port}/api/v1/';

    await tester.enterText(
      find.widgetWithText(TextFormField, 'Server URL'),
      unnormalizedInput,
    );
    await tester.enterText(
      find.widgetWithText(TextFormField, 'Username'),
      adminUsername,
    );
    await tester.enterText(
      find.widgetWithText(TextFormField, 'Password'),
      adminPassword,
    );

    await tester.ensureVisible(find.text('Connect & Sync Library'));
    await tester.tap(find.text('Connect & Sync Library'));
    await tester.pump();

    await _pumpLiveUntil(
      tester,
      () => capturedRef.read(authProvider).isAuthenticated,
    );

    expect(storage.getServerUrl(), liveBaseUrl);
    final issuedToken = storage.getAuthToken();
    expect(issuedToken, isNotNull);
    expect(issuedToken, startsWith('shelfd_'));
    expect(storage.getAuthTokenId(), isNotEmpty);

    final me = await tester.runAsync(
      () => capturedRef.read(apiServiceProvider).getMe(),
    );
    expect(me!.id, isNotEmpty);
    expect(me.username, adminUsername);

    final tokens = await tester.runAsync(
      () => capturedRef.read(apiServiceProvider).listApiTokens(),
    );
    expect(
      tokens!.any((t) => t.id == storage.getAuthTokenId()),
      isTrue,
      reason: 'Issued device token must appear in GET /api/v1/auth/tokens',
    );

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(seconds: 16));
  });

  testWidgets(
      'beginPairing rejects accidental localhost QR scan in UI, exchanges shelfd://pair for single-use device token, and rejects replay with 401',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });

    final adminClient = http.Client();
    addTearDown(adminClient.close);

    final adminApi = ApiService(baseUrl: liveBaseUrl, client: adminClient);
    final loginRes = await tester.runAsync(
      () => adminApi.loginWithDevice(adminUsername, adminPassword),
    );
    adminApi.updateConnection(newToken: loginRes!.token);

    final pairing = await tester.runAsync(adminApi.beginPairing);
    expect(pairing!.code, isNotEmpty);
    expect(
      pairing.expiresAt.isAfter(DateTime.now().toUtc()),
      isTrue,
    );

    final pairUri =
        'shelfd://pair?u=${Uri.encodeComponent(liveBaseUrl)}&c=${Uri.encodeComponent(pairing.code)}';

    SharedPreferences.setMockInitialValues({});
    final prefs = await tester.runAsync(SharedPreferences.getInstance);
    final mobileStorage = StorageService(
      prefs!,
      secureStorage: InMemorySecureTokenStorage(),
    );
    await tester.runAsync(mobileStorage.init);

    late WidgetRef capturedRef;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(mobileStorage),
          qrScannerServiceProvider.overrideWithValue(
            _LiveQrScannerService(scanResult: pairUri),
          ),
        ],
        child: Consumer(
          builder: (context, ref, _) {
            capturedRef = ref;
            return MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.dark),
              home: const ConnectView(),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('Scan Server QR Code'));
    await tester.pump();
    await _pumpLiveUntil(
      tester,
      () => (capturedRef.read(authProvider).error ?? '').contains('localhost'),
    );
    expect(
      find.textContaining('This QR code uses localhost'),
      findsOneWidget,
    );

    final pairedUser = await tester.runAsync(
      () => capturedRef.read(authRepositoryProvider).pairWithUri(
            pairUri,
            deviceName: 'Shelfd Android (Pixel 9 Live E2E)',
            rejectLoopback: false,
          ),
    );
    expect(pairedUser!.username, adminUsername);
    expect(mobileStorage.getServerUrl(), liveBaseUrl);
    expect(mobileStorage.getAuthToken(), startsWith('shelfd_'));
    expect(mobileStorage.getAuthTokenId(), isNotEmpty);

    ApiException? replayError;
    await tester.runAsync(() async {
      try {
        await adminApi.exchangePairing(
          pairing.code,
          deviceName: 'Replay Attacker',
        );
      } on ApiException catch (e) {
        replayError = e;
      }
    });
    expect(replayError, isNotNull);
    expect(replayError!.statusCode, 401);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(seconds: 16));
  });

  test(
      'ShelfdAuthenticatedImage cover headers and book-scoped media JWT enforce origin and route boundaries',
      () async {
    final rawClient = http.Client();
    addTearDown(rawClient.close);

    final api = ApiService(
      baseUrl: liveBaseUrl,
      client: rawClient,
      isWebOverride: false,
    );
    final loginRes = await api.loginWithDevice(
      adminUsername,
      adminPassword,
      deviceName: 'Shelfd Android (Media Test)',
    );
    expect(loginRes.token, startsWith('shelfd_'));
    api.updateConnection(newToken: loginRes.token);

    final staged = await api.stageUploadBook(
      filename: 'specimen.epub',
      bytes: _buildMinimalFixedLayoutEpub(),
      coverFilename: 'cover.png',
      coverBytes: _oneByOnePng,
    );
    final book = await api.commitUploadJob(staged.jobId, staged.metadata);
    expect(book.id, isNotEmpty);

    final coverUrl = '$liveBaseUrl/api/v1/books/${book.id}/cover';
    final unauthCoverResp = await rawClient.get(Uri.parse(coverUrl));
    expect(unauthCoverResp.statusCode, 401);

    final imageHeaders = ShelfdAuthenticatedImage.buildOriginScopedHeaders(
      targetUrl: coverUrl,
      serverBaseUrl: liveBaseUrl,
      authToken: loginRes.token,
      isWeb: false,
    );
    expect(imageHeaders, {'Authorization': 'Bearer ${loginRes.token}'});

    final authCoverResp = await rawClient.get(
      Uri.parse(coverUrl),
      headers: imageHeaders,
    );
    expect(authCoverResp.statusCode, 200);
    expect(authCoverResp.bodyBytes.take(4).toList(), [0x89, 0x50, 0x4E, 0x47]);

    final mediaToken = await api.mintBookMediaToken(book.id);
    expect(mediaToken.token, isNotEmpty);
    expect(
      mediaToken.expiresAt.isAfter(
        DateTime.now().toUtc().add(const Duration(minutes: 100)),
      ),
      isTrue,
    );

    final cookieHeaders = {'Cookie': 'shelfd_token=${mediaToken.token}'};

    final chapterGet = await rawClient.get(
      Uri.parse('$liveBaseUrl/api/v1/books/${book.id}/chapters/1/html'),
      headers: cookieHeaders,
    );
    expect(chapterGet.statusCode, 200);
    expect(chapterGet.body, contains('ShelfdBridge'));

    final chapterHead = await rawClient.head(
      Uri.parse('$liveBaseUrl/api/v1/books/${book.id}/chapters/1/html'),
      headers: cookieHeaders,
    );
    expect(chapterHead.statusCode, 200);

    final assetGet = await rawClient.get(
      Uri.parse(
        '$liveBaseUrl/api/v1/books/${book.id}/assets/OEBPS/images/illo.png',
      ),
      headers: cookieHeaders,
    );
    expect(assetGet.statusCode, 200);

    final assetHead = await rawClient.head(
      Uri.parse(
        '$liveBaseUrl/api/v1/books/${book.id}/assets/OEBPS/images/illo.png',
      ),
      headers: cookieHeaders,
    );
    expect(assetHead.statusCode, 200);

    final coverHead = await rawClient.head(
      Uri.parse(coverUrl),
      headers: cookieHeaders,
    );
    expect(coverHead.statusCode, 200);

    final forbiddenBooksCookie = await rawClient.get(
      Uri.parse('$liveBaseUrl/api/v1/books'),
      headers: cookieHeaders,
    );
    expect(forbiddenBooksCookie.statusCode, 403);

    final forbiddenBooksBearer = await rawClient.get(
      Uri.parse('$liveBaseUrl/api/v1/books'),
      headers: {'Authorization': 'Bearer ${mediaToken.token}'},
    );
    expect(forbiddenBooksBearer.statusCode, 403);
  });

  test(
      'offline connection failure in checkInitialAuth retains stored server URL and device token',
      () async {
    final closedPort = await _reserveFreePort();
    final unreachableUrl = 'http://127.0.0.1:$closedPort';

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storage = StorageService(
      prefs,
      secureStorage: InMemorySecureTokenStorage(),
    );
    await storage.init();
    await storage.saveServerConnection(
      unreachableUrl,
      'shelfd_offline_keep_token',
      tokenId: 'tok-offline-1',
    );

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        storageServiceProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);

    await container.read(authProvider.notifier).checkAuth();

    expect(
      storage.getServerUrl(),
      unreachableUrl,
      reason: 'Offline startup must keep configured server URL',
    );
    expect(
      storage.getAuthToken(),
      'shelfd_offline_keep_token',
      reason: 'Offline startup must never wipe stored device token',
    );
    expect(
      container.read(authProvider).isAuthenticated,
      isTrue,
      reason: 'Offline startup keeps cached session active until server explicitly returns 401',
    );
  });

  testWidgets(
      'revoking active device token returns 401 on subsequent request, clears token, preserves server URL, and redirects GoRouter to /connect',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });

    final adminClient = http.Client();
    addTearDown(adminClient.close);
    final adminApi = ApiService(baseUrl: liveBaseUrl, client: adminClient);
    final adminLogin = await tester.runAsync(
      () => adminApi.loginWithDevice(
        adminUsername,
        adminPassword,
        deviceName: 'Shelfd Web Admin',
      ),
    );
    adminApi.updateConnection(newToken: adminLogin!.token);

    final victimBootstrap = ApiService(
      baseUrl: liveBaseUrl,
      client: adminClient,
    );
    final victimLogin = await tester.runAsync(
      () => victimBootstrap.loginWithDevice(
        adminUsername,
        adminPassword,
        deviceName: 'Shelfd Android Revocable',
      ),
    );
    expect(victimLogin!.tokenId, isNotEmpty);

    SharedPreferences.setMockInitialValues({});
    final prefs = await tester.runAsync(SharedPreferences.getInstance);
    final victimStorage = StorageService(
      prefs!,
      secureStorage: InMemorySecureTokenStorage(),
    );
    await tester.runAsync(victimStorage.init);
    await tester.runAsync(
      () => victimStorage.saveServerConnection(
        liveBaseUrl,
        victimLogin.token,
        tokenId: victimLogin.tokenId,
      ),
    );

    late WidgetRef capturedRef;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(victimStorage),
          qrScannerServiceProvider.overrideWithValue(_LiveQrScannerService()),
        ],
        child: Consumer(
          builder: (context, ref, _) {
            capturedRef = ref;
            return const ShelfApp(initialRoute: '/books');
          },
        ),
      ),
    );

    await _pumpLiveUntil(
      tester,
      () =>
          capturedRef.read(authProvider).isAuthenticated &&
          !capturedRef.read(authProvider).isLoading,
    );

    await tester.runAsync(() => adminApi.deleteApiToken(victimLogin.tokenId!));

    ApiException? revokedError;
    await tester.runAsync(() async {
      try {
        await capturedRef.read(apiServiceProvider).getBooks();
      } on ApiException catch (e) {
        revokedError = e;
      }
    });
    expect(revokedError, isNotNull);
    expect(revokedError!.statusCode, 401);

    await _pumpLiveUntil(
      tester,
      () =>
          victimStorage.getAuthToken() == null &&
          find.byType(ConnectView).evaluate().isNotEmpty,
    );

    expect(victimStorage.getAuthToken(), isNull);
    expect(victimStorage.getServerUrl(), liveBaseUrl);
    expect(find.text('Connect to your Library'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 50)),
    );
    await tester.pump(const Duration(seconds: 16));
  });

  test(
      'cross-origin HTTP redirect strips Authorization header from ApiService and image requests',
      () async {
    String? downstreamAuthHeader;
    final targetServer = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => targetServer.close(force: true));
    targetServer.listen((HttpRequest req) {
      downstreamAuthHeader = req.headers.value(HttpHeaders.authorizationHeader);
      req.response
        ..statusCode = 200
        ..headers.contentType = ContentType.json
        ..write(jsonEncode({'books': const [], 'total': 0}))
        ..close();
    });

    final originServer = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => originServer.close(force: true));
    originServer.listen((HttpRequest req) {
      req.response
        ..statusCode = 302
        ..headers.set(
          HttpHeaders.locationHeader,
          'http://127.0.0.1:${targetServer.port}${req.uri.path}',
        )
        ..close();
    });

    final redirectClient = http.Client();
    addTearDown(redirectClient.close);
    final api = ApiService(
      baseUrl: 'http://127.0.0.1:${originServer.port}',
      token: 'shelfd_secret_do_not_leak',
      client: redirectClient,
    );

    await api.getBooks();
    expect(
      downstreamAuthHeader,
      isNull,
      reason: 'Cross-origin redirect must never forward Authorization header',
    );
  });
}

