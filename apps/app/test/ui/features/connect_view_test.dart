import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/platform/qr_scanner_service.dart';
import 'package:shelf/ui/core/shelfd_authenticated_image.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/connect/connect_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeQrScannerService extends QrScannerService {
  final String? scanResult;
  final bool throwUnavailable;

  _FakeQrScannerService({
    this.scanResult,
    this.throwUnavailable = false,
  });

  @override
  Future<String?> scanQrCode() async {
    if (throwUnavailable) {
      throw const QrScannerUnavailableException();
    }
    return scanResult;
  }

  @override
  Future<String> getDeviceModel() async => 'Pixel 9';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('ConnectView displays QR option, empty mobile URL input, manual inputs, and privacy badge',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const ConnectView(),
        ),
      ),
    );

    expect(find.text('Connect to your Library'), findsOneWidget);
    expect(find.text('Scan Server QR Code'), findsOneWidget);
    expect(find.text('Paste Pairing Link'), findsOneWidget);
    expect(find.text('Or connect manually'), findsOneWidget);

    final urlField = tester.widget<TextFormField>(
      find.widgetWithText(TextFormField, 'Server URL'),
    );
    expect(urlField.controller?.text, isEmpty);
    expect(find.widgetWithText(TextField, 'Username'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Password'), findsOneWidget);
    expect(find.text('Connect & Sync Library'), findsOneWidget);
    expect(find.textContaining('Direct connection to your private server'), findsOneWidget);
  });

  test('resolveDefaultServerUrl returns empty on mobile and localhost on desktop', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    expect(resolveDefaultServerUrl(), '');
    debugDefaultTargetPlatformOverride = TargetPlatform.linux;
    expect(resolveDefaultServerUrl(), 'http://localhost:8080');
    debugDefaultTargetPlatformOverride = null;
  });

  test('ShelfdAuthenticatedImage attaches Authorization strictly to same-origin Shelfd URLs', () {
    final sameOrigin = ShelfdAuthenticatedImage.buildOriginScopedHeaders(
      targetUrl: 'https://shelfd.example.com/api/v1/books/b1/cover',
      serverBaseUrl: 'https://shelfd.example.com',
      authToken: 'shelfd_tok_123',
      isWeb: false,
    );
    expect(sameOrigin, {'Authorization': 'Bearer shelfd_tok_123'});

    final externalOrigin = ShelfdAuthenticatedImage.buildOriginScopedHeaders(
      targetUrl: 'https://covers.openlibrary.org/b/id/99-L.jpg',
      serverBaseUrl: 'https://shelfd.example.com',
      authToken: 'shelfd_tok_123',
      isWeb: false,
    );
    expect(externalOrigin, isNull);
  });

  testWidgets('ConnectView pre-fills with saved server URL from preferences', (tester) async {
    SharedPreferences.setMockInitialValues({
      'shelfd_server_url': 'https://shelfd.thebenforce.com',
      'shelfd_saved_username': 'customuser',
    });
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const ConnectView(),
        ),
      ),
    );

    expect(find.text('https://shelfd.thebenforce.com'), findsOneWidget);
    expect(find.text('customuser'), findsOneWidget);
  });

  testWidgets('ConnectView pairs directly when native QR scanner returns shelfd://pair link', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storage = StorageService(prefs, secureStorage: InMemorySecureTokenStorage());
    await storage.init();

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/auth/pair/exchange') {
        return http.Response(
          jsonEncode({
            'id': 'tok-scan-1',
            'name': 'Shelfd Android (Pixel 9)',
            'token': 'shelfd_direct_scan_token',
            'user': {'id': 'u1', 'username': 'admin', 'role': 'admin'},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storage),
          apiServiceProvider.overrideWithValue(
            ApiService(baseUrl: '', client: mockClient),
          ),
          qrScannerServiceProvider.overrideWithValue(
            _FakeQrScannerService(
              scanResult: 'shelfd://pair?u=https%3A%2F%2Fshelfd.example.com&c=scan123456',
            ),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: const ConnectView(),
        ),
      ),
    );

    await tester.tap(find.text('Scan Server QR Code'));
    await tester.pumpAndSettle();

    expect(storage.getServerUrl(), 'https://shelfd.example.com');
    expect(storage.getAuthToken(), 'shelfd_direct_scan_token');
  });

  testWidgets('ConnectView falls back to Paste Pairing Link dialog when native QR scanner is unavailable', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() {
      debugDefaultTargetPlatformOverride = null;
    });
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storage = StorageService(prefs, secureStorage: InMemorySecureTokenStorage());
    await storage.init();

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/auth/pair/exchange') {
        return http.Response(
          jsonEncode({
            'id': 'tok-pair-1',
            'name': 'Shelfd Android (Pixel 9)',
            'token': 'shelfd_qr_token_xyz',
            'user': {'id': 'u1', 'username': 'admin', 'role': 'admin'},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storage),
          apiServiceProvider.overrideWithValue(
            ApiService(baseUrl: '', client: mockClient),
          ),
          qrScannerServiceProvider.overrideWithValue(
            _FakeQrScannerService(throwUnavailable: true),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: const ConnectView(),
        ),
      ),
    );

    await tester.tap(find.text('Scan Server QR Code'));
    await tester.pumpAndSettle();

    expect(find.text('Pair with Server Link'), findsOneWidget);
    await tester.enterText(
      find.byKey(const Key('pairing_link_input')),
      'shelfd://pair?u=https%3A%2F%2Fshelfd.example.com&c=abcdef123456',
    );
    await tester.tap(find.byKey(const Key('pair_device_submit_button')));
    await tester.pumpAndSettle();

    expect(storage.getServerUrl(), 'https://shelfd.example.com');
    expect(storage.getAuthToken(), 'shelfd_qr_token_xyz');
  });

  testWidgets('ConnectView prompts before cleartext HTTP sign-in to non-private host', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const ConnectView(),
        ),
      ),
    );

    await tester.enterText(
      find.widgetWithText(TextFormField, 'Server URL'),
      'http://books.example.com',
    );
    await tester.enterText(
      find.widgetWithText(TextFormField, 'Password'),
      'secret123',
    );

    await tester.ensureVisible(find.text('Connect & Sync Library'));
    await tester.tap(find.text('Connect & Sync Library'));
    await tester.pumpAndSettle();

    expect(find.text('Unencrypted HTTP Connection'), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.text('Unencrypted HTTP Connection'), findsNothing);
  });
}
