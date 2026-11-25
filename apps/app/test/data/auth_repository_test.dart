import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/repositories/auth_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/data/utils/connect_diagnostics.dart';
import 'package:shelf/data/utils/url_normalizer.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('StorageService pre-hydration and Keystore recovery', () {
    test('migrates legacy SharedPreferences token into SecureTokenStorage and removes plaintext copy', () async {
      SharedPreferences.setMockInitialValues({
        'shelfd_server_url': 'https://shelfd.example.com',
        'shelfd_auth_token': 'shelfd_legacy_secret',
        'shelfd_auth_token_id': 'legacy-id-1',
      });
      final prefs = await SharedPreferences.getInstance();
      final secure = InMemorySecureTokenStorage();
      final storage = StorageService(prefs, secureStorage: secure);

      await storage.init();

      expect(storage.getAuthToken(), 'shelfd_legacy_secret');
      expect(storage.getAuthTokenId(), 'legacy-id-1');
      expect(secure.entries['shelfd_auth_token'], 'shelfd_legacy_secret');
      expect(secure.entries['shelfd_auth_token_id'], 'legacy-id-1');
      expect(prefs.getString('shelfd_auth_token'), isNull);
      expect(prefs.getString('shelfd_auth_token_id'), isNull);
      expect(storage.getServerUrl(), 'https://shelfd.example.com');
    });

    test('recovers cleanly via deleteAll when Android Keystore throws PlatformException on read', () async {
      SharedPreferences.setMockInitialValues({
        'shelfd_server_url': 'https://shelfd.example.com',
      });
      final prefs = await SharedPreferences.getInstance();
      final secure = InMemorySecureTokenStorage();
      await secure.write('shelfd_auth_token', 'corrupted-ciphertext');
      secure.throwPlatformExceptionOnRead = true;

      final storage = StorageService(prefs, secureStorage: secure);
      await storage.init();

      expect(storage.getAuthToken(), isNull);
      expect(secure.entries, isEmpty);
      expect(storage.getServerUrl(), 'https://shelfd.example.com');
    });
  });

  group('AuthRepository self-hosted sign-in, pairing, and offline retention', () {
    late SharedPreferences prefs;
    late InMemorySecureTokenStorage secure;
    late StorageService storage;

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      prefs = await SharedPreferences.getInstance();
      secure = InMemorySecureTokenStorage();
      storage = StorageService(prefs, secureStorage: secure);
      await storage.init();
    });

    test('checkInitialAuth retains server URL and token on network timeout or SocketException', () async {
      await storage.saveServerConnection(
        'https://shelfd.tailnet.ts.net',
        'shelfd_offline_token',
        username: 'ben',
        tokenId: 'tok-99',
      );

      final timeoutClient = MockClient((_) async {
        throw TimeoutException('Timed out');
      });
      final api = ApiService(
        baseUrl: 'https://shelfd.tailnet.ts.net',
        token: 'shelfd_offline_token',
        client: timeoutClient,
      );
      final repo = AuthRepository(apiService: api, storageService: storage);

      final isAuthed = await repo.checkInitialAuth();
      expect(isAuthed, isTrue);
      expect(storage.getServerUrl(), 'https://shelfd.tailnet.ts.net');
      expect(storage.getAuthToken(), 'shelfd_offline_token');

      final socketClient = MockClient((_) async {
        throw const SocketException('Network is unreachable');
      });
      final socketRepo = AuthRepository(
        apiService: ApiService(
          baseUrl: 'https://shelfd.tailnet.ts.net',
          token: 'shelfd_offline_token',
          client: socketClient,
        ),
        storageService: storage,
      );
      expect(await socketRepo.checkInitialAuth(), isTrue);
      expect(storage.getAuthToken(), 'shelfd_offline_token');
    });

    test('checkInitialAuth clears revoked token on 401 while keeping shelfd_server_url', () async {
      await storage.saveServerConnection(
        'https://shelfd.example.com',
        'shelfd_revoked_token',
        username: 'ben',
        tokenId: 'tok-revoked',
      );

      final client = MockClient((_) async {
        return http.Response(jsonEncode({'error': 'Invalid or expired token'}), 401);
      });
      final api = ApiService(
        baseUrl: 'https://shelfd.example.com',
        token: 'shelfd_revoked_token',
        client: client,
      );
      final repo = AuthRepository(apiService: api, storageService: storage);

      expect(await repo.checkInitialAuth(), isFalse);
      expect(storage.getAuthToken(), isNull);
      expect(storage.getAuthTokenId(), isNull);
      expect(storage.getServerUrl(), 'https://shelfd.example.com');
    });

    test('pairWithUri exchanges single-use code, normalizes URL, and persists device token', () async {
      String? sentCode;
      String? sentDevice;
      final client = MockClient((request) async {
        expect(request.url.toString(), 'https://books.example.com/api/v1/auth/pair/exchange');
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        sentCode = body['code'] as String?;
        sentDevice = body['device_name'] as String?;
        return http.Response(
          jsonEncode({
            'id': 'pair-tok-1',
            'name': sentDevice,
            'token': 'shelfd_paired_secret_abc',
            'user': {'id': 'u-1', 'username': 'reader', 'role': 'admin'},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final api = ApiService(baseUrl: '', client: client);
      final repo = AuthRepository(apiService: api, storageService: storage);

      final pairUri = UrlNormalizer.buildPairingUri(
        serverUrl: 'https://books.example.com/',
        code: 'deadbeef01234567',
      );
      final user = await repo.pairWithUri(
        pairUri,
        deviceName: 'Shelfd Android (Pixel 9)',
      );

      expect(user.username, 'reader');
      expect(sentCode, 'deadbeef01234567');
      expect(sentDevice, 'Shelfd Android (Pixel 9)');
      expect(storage.getServerUrl(), 'https://books.example.com');
      expect(storage.getAuthToken(), 'shelfd_paired_secret_abc');
      expect(storage.getAuthTokenId(), 'pair-tok-1');
    });

    test('ApiService invokes centralized onUnauthorized callback on authenticated 401 but not on login 401', () async {
      int unauthorizedCount = 0;
      final client = MockClient((request) async {
        return http.Response(jsonEncode({'error': 'Unauthorized'}), 401);
      });

      final api = ApiService(
        baseUrl: 'https://books.example.com',
        token: 'shelfd_active_token',
        client: client,
        onUnauthorized: () {
          unauthorizedCount++;
        },
      );

      await expectLater(api.login('admin', 'wrong'), throwsA(isA<ApiException>()));
      expect(unauthorizedCount, 0);

      await expectLater(api.getBooks(), throwsA(isA<ApiException>()));
      expect(unauthorizedCount, 1);
    });

    test('login probes connect-info before sending credentials and aborts on non-Shelfd 404', () async {
      bool loginEndpointCalled = false;
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/server/connect-info') {
          return http.Response('Not Found', 404);
        }
        if (request.url.path == '/api/v1/auth/login') {
          loginEndpointCalled = true;
          return http.Response('{}', 200);
        }
        return http.Response('Not Found', 404);
      });
      final api = ApiService(baseUrl: '', client: client);
      final repo = AuthRepository(apiService: api, storageService: storage);

      await expectLater(
        repo.login('https://wrong-host.example.com', 'admin', 'secret123'),
        throwsA(isA<ApiException>()),
      );
      expect(loginEndpointCalled, isFalse);
    });
  });

  group('ConnectDiagnostics formatting and token redaction', () {
    test('maps TLS, Tailscale timeout, and 502 errors cleanly and redacts tokens in clipboard summary', () {
      final tlsDiag = ConnectDiagnostics.fromError(
        const HandshakeException('CERTIFICATE_VERIFY_FAILED: self signed certificate'),
        serverUrl: 'https://homelab.local:8443/api/v1?token=shelfd_supersecret123',
      );
      expect(tlsDiag.message, contains('TLS certificate verification failed'));
      expect(tlsDiag.clipboardDetails, isNot(contains('shelfd_supersecret123')));
      expect(tlsDiag.clipboardDetails, contains('[REDACTED]'));

      final tailscaleTimeout = ConnectDiagnostics.fromError(
        TimeoutException('Timed out'),
        serverUrl: 'https://nas.tailnet.ts.net',
      );
      expect(tailscaleTimeout.message, contains('Tailscale VPN'));

      final badGateway = ConnectDiagnostics.fromError(
        const ApiException(502, 'Bad Gateway'),
        serverUrl: 'https://shelfd.example.com',
      );
      expect(badGateway.message, contains('HTTP 502'));
    });
  });
}
