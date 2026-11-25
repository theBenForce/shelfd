import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/queue_status.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeQueueNotifier extends QueueNotifier {
  final QueueState _initial;
  FakeQueueNotifier([this._initial = const QueueState()]);

  @override
  QueueState build() => _initial;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late SharedPreferences prefs;

  setUp(() async {
    SharedPreferences.setMockInitialValues({
      'shelfd_saved_username': 'test_admin',
      'shelfd_server_url': 'http://localhost:8080',
      'shelfd_auth_token': 'test-token-123',
    });
    prefs = await SharedPreferences.getInstance();
  });

  void setLargeViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(1200, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
  }

  testWidgets('SettingsView renders simplified sections and expandable change password form',
      (tester) async {
    setLargeViewport(tester);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Settings'), findsWidgets);
    expect(find.text('Appearance & Theme'), findsOneWidget);
    expect(find.text('Plugins & Storefronts'), findsOneWidget);
    expect(find.text('Server Connection'), findsOneWidget);
    expect(find.text('Rescan Library'), findsOneWidget);
    expect(find.text('Account & Security'), findsOneWidget);
    expect(find.text('Default Username'), findsOneWidget);
    expect(find.text('Change Password'), findsOneWidget);
    expect(find.text('Disconnect / Sign Out'), findsOneWidget);

    // Password form is collapsed by default for progressive disclosure
    expect(find.text('Current Password'), findsNothing);

    await tester.tap(find.byKey(const Key('toggle_change_password_button')));
    await tester.pumpAndSettle();

    expect(find.text('Current Password'), findsOneWidget);
    expect(find.text('New Password'), findsOneWidget);
    expect(find.text('Confirm New Password'), findsOneWidget);
    expect(find.text('Update Password'), findsOneWidget);
  });

  testWidgets('SettingsView validates empty fields and mismatched passwords', (tester) async {
    setLargeViewport(tester);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('toggle_change_password_button')));
    await tester.pumpAndSettle();

    // Tap Update Password with empty inputs
    await tester.tap(find.text('Update Password'));
    await tester.pumpAndSettle();

    expect(find.text('Current password required'), findsOneWidget);
    expect(find.text('New password required'), findsOneWidget);

    // Enter short password
    await tester.enterText(find.widgetWithText(TextFormField, 'Current Password'), 'currentPass');
    await tester.enterText(find.widgetWithText(TextFormField, 'New Password'), '123');
    await tester.enterText(find.widgetWithText(TextFormField, 'Confirm New Password'), '123');
    await tester.tap(find.text('Update Password'));
    await tester.pumpAndSettle();

    expect(find.text('Password must be at least 6 characters'), findsOneWidget);

    // Enter mismatched passwords
    await tester.enterText(find.widgetWithText(TextFormField, 'New Password'), 'password123');
    await tester.enterText(find.widgetWithText(TextFormField, 'Confirm New Password'), 'different123');
    await tester.tap(find.text('Update Password'));
    await tester.pumpAndSettle();

    expect(find.text('Passwords do not match'), findsOneWidget);
  });

  testWidgets('SettingsView successfully changes password and clears form', (tester) async {
    setLargeViewport(tester);

    bool apiCalled = false;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/auth/change-password') {
        apiCalled = true;
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['current_password'], 'validCurrentPass');
        expect(body['new_password'], 'brandNewPass123');
        return http.Response(jsonEncode({'message': 'Password updated successfully'}), 200);
      }
      return http.Response('Not Found', 404);
    });

    final testApiService = ApiService(baseUrl: 'http://localhost:8080', token: 'test-token-123', client: mockClient);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
          apiServiceProvider.overrideWithValue(testApiService),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('toggle_change_password_button')));
    await tester.pumpAndSettle();

    await tester.enterText(find.widgetWithText(TextFormField, 'Current Password'), 'validCurrentPass');
    await tester.enterText(find.widgetWithText(TextFormField, 'New Password'), 'brandNewPass123');
    await tester.enterText(find.widgetWithText(TextFormField, 'Confirm New Password'), 'brandNewPass123');

    await tester.tap(find.text('Update Password'));
    await tester.pumpAndSettle();

    expect(apiCalled, isTrue);
    expect(find.text('Password updated successfully!'), findsWidgets);
  });

  testWidgets('SettingsView displays error message on API failure', (tester) async {
    setLargeViewport(tester);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/auth/change-password') {
        return http.Response(jsonEncode({'error': 'Current password is incorrect'}), 401);
      }
      return http.Response('Not Found', 404);
    });

    final testApiService = ApiService(baseUrl: 'http://localhost:8080', token: 'test-token-123', client: mockClient);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
          apiServiceProvider.overrideWithValue(testApiService),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('toggle_change_password_button')));
    await tester.pumpAndSettle();

    await tester.enterText(find.widgetWithText(TextFormField, 'Current Password'), 'wrongCurrentPass');
    await tester.enterText(find.widgetWithText(TextFormField, 'New Password'), 'brandNewPass123');
    await tester.enterText(find.widgetWithText(TextFormField, 'Confirm New Password'), 'brandNewPass123');

    await tester.tap(find.text('Update Password'));
    await tester.pumpAndSettle();

    expect(find.text('Current password is incorrect'), findsOneWidget);
  });

  testWidgets('SettingsView updates default username in storage', (tester) async {
    setLargeViewport(tester);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Default username input has initial value
    expect(find.text('test_admin'), findsOneWidget);

    await tester.enterText(find.widgetWithText(TextFormField, 'e.g. admin or your username'), 'custom_user_42');
    await tester.tap(find.text('Save'));
    await tester.pumpAndSettle();

    expect(prefs.getString('shelfd_saved_username'), 'custom_user_42');
    expect(find.text('Default username updated to "custom_user_42"'), findsOneWidget);
  });

  testWidgets('SettingsView toggles application theme via segmented button', (tester) async {
    setLargeViewport(tester);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Appearance & Theme'), findsOneWidget);
    expect(find.text('Dark'), findsOneWidget);

    await tester.tap(find.text('Dark'));
    await tester.pumpAndSettle();

    expect(prefs.getString('shelfd_theme_mode'), 'dark');
  });

  testWidgets('SettingsView renders AI Indexing status and Host URL in Dark theme', (tester) async {
    setLargeViewport(tester);

    const queueStatus = QueueState(
      status: QueueStatus(
        totalChapters: 20,
        indexedChapters: 15,
        pendingChapters: 5,
        progressPercent: 75.0,
        currentBook: 'Dune',
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier(queueStatus)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('AI Indexing & Vectors'), findsOneWidget);
    expect(find.text('75%'), findsWidgets);
    expect(find.textContaining('Host:'), findsOneWidget);
  });

  testWidgets('SettingsView Web Pair Mobile Device modal warns on localhost and renders QR when LAN IP is entered', (tester) async {
    setLargeViewport(tester);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/auth/pair/begin') {
        return http.Response(
          jsonEncode({
            'code': '0123456789abcdef0123456789abcdef',
            'expires_at': DateTime.now().toUtc().add(const Duration(seconds: 90)).toIso8601String(),
          }),
          201,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/auth/tokens') {
        return http.Response(jsonEncode({'tokens': []}), 200);
      }
      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
          apiServiceProvider.overrideWithValue(api),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(isWebOverride: true),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('pair_mobile_device_button')));
    await tester.pumpAndSettle();

    expect(find.textContaining('opened via localhost'), findsOneWidget);
    expect(find.byKey(const Key('pairing_qr_image')), findsNothing);

    await tester.enterText(
      find.byKey(const Key('pairing_reachable_url_field')),
      'http://192.168.1.50:8080',
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('pairing_qr_image')), findsOneWidget);
    expect(find.byKey(const Key('copy_pairing_link_button')), findsOneWidget);
  });

  testWidgets('SettingsView lists Active Devices & API Tokens and revokes selected device', (tester) async {
    setLargeViewport(tester);

    String? deletedTokenId;
    var tokens = <Map<String, dynamic>>[
      {
        'id': 'tok-self',
        'name': 'Shelfd Android (Pixel 9)',
        'created_at': '2026-11-24T12:00:00Z',
      },
      {
        'id': 'tok-tablet',
        'name': 'Shelfd Android (Galaxy Tab)',
        'created_at': '2026-11-24T13:00:00Z',
      },
    ];

    SharedPreferences.setMockInitialValues({
      'shelfd_saved_username': 'test_admin',
      'shelfd_server_url': 'https://shelfd.example.com',
      'shelfd_auth_token': 'shelfd_self_tok',
      'shelfd_auth_token_id': 'tok-self',
    });
    final localPrefs = await SharedPreferences.getInstance();

    final mockClient = MockClient((request) async {
      if (request.method == 'GET' && request.url.path == '/api/v1/auth/tokens') {
        return http.Response(jsonEncode({'tokens': tokens}), 200);
      }
      if (request.method == 'DELETE' && request.url.path == '/api/v1/auth/tokens/tok-tablet') {
        deletedTokenId = 'tok-tablet';
        tokens = tokens.where((t) => t['id'] != 'tok-tablet').toList();
        return http.Response('', 204);
      }
      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'https://shelfd.example.com',
      token: 'shelfd_self_tok',
      client: mockClient,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(localPrefs),
          queueProvider.overrideWith(() => FakeQueueNotifier()),
          apiServiceProvider.overrideWithValue(api),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Active Devices & API Tokens'), findsOneWidget);
    expect(find.text('Shelfd Android (Pixel 9)'), findsOneWidget);
    expect(find.text('This device'), findsOneWidget);
    expect(find.text('Shelfd Android (Galaxy Tab)'), findsOneWidget);

    await tester.tap(find.byKey(const Key('revoke_token_tok-tablet')));
    await tester.pumpAndSettle();

    expect(deletedTokenId, 'tok-tablet');
    expect(find.text('Shelfd Android (Galaxy Tab)'), findsNothing);
  });
}
