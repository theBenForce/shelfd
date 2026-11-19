import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/plugins/catalog_browser_view.dart';
import 'package:shelf/ui/features/settings/plugin_config_dialog.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeQueueNotifier extends QueueNotifier {
  @override
  QueueState build() => const QueueState();

  @override
  Future<void> refresh() async {}
}

class _FakeUploadNotifier extends UploadNotifier {
  int reloadCalls = 0;

  @override
  UploadState build() => const UploadState(stagedJobs: []);

  @override
  Future<void> loadStagedJobs() async {
    reloadCalls++;
  }
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

  void setDesktopViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(1280, 1200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
  }

  testWidgets(
      'CatalogBrowserView renders items, provenance badges, and stages selected items',
      (tester) async {
    setDesktopViewport(tester);

    List<dynamic>? stagedRequestItems;
    final fakeUpload = _FakeUploadNotifier();

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(
          jsonEncode({
            'plugins': [
              {
                'name': 'project-gutenberg',
                'display_name': 'Project Gutenberg',
                'version': '0.1.0',
                'enabled': true,
                'status': 'idle',
                'capabilities': ['catalog'],
                'catalog_modes': ['browse', 'search'],
                'configured': true,
              },
            ],
          }),
          200,
        );
      }

      if (request.url.path == '/api/v1/plugins/project-gutenberg/catalog') {
        return http.Response(
          jsonEncode({
            'items': [
              {
                'external_id': '1342',
                'title': 'Pride and Prejudice',
                'authors': ['Jane Austen'],
                'formats': [
                  {'format': 'epub', 'label': 'EPUB'},
                ],
                'status': 'available',
              },
              {
                'external_id': '84',
                'title': 'Frankenstein',
                'authors': ['Mary Shelley'],
                'status': 'in_library',
                'book_id': 'b-84',
              },
            ],
            'groups': [
              {'id': 'Adventure', 'label': 'Adventure'},
            ],
            'total_count': 2,
          }),
          200,
        );
      }

      if (request.url.path ==
              '/api/v1/plugins/project-gutenberg/catalog/stage' &&
          request.method == 'POST') {
        final decoded = jsonDecode(request.body) as Map<String, dynamic>;
        stagedRequestItems = decoded['items'] as List<dynamic>?;
        return http.Response(
          jsonEncode({
            'async': false,
            'results': [
              {
                'external_id': '1342',
                'title': 'Pride and Prejudice',
                'upload_job_id': 'job-1342',
                'status': 'staged',
              },
            ],
          }),
          200,
        );
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
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
          uploadProvider.overrideWith(() => fakeUpload),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const CatalogBrowserView(initialPlugin: 'project-gutenberg'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Storefront Catalog'), findsOneWidget);
    expect(find.text('Pride and Prejudice'), findsOneWidget);
    expect(find.text('Frankenstein'), findsOneWidget);
    expect(find.text('In Library'), findsOneWidget);
    expect(find.text('Stage Selected (0)'), findsOneWidget);

    // Select Pride and Prejudice
    final checkboxFinder = find.byKey(const Key('catalog_checkbox_1342'));
    await tester.ensureVisible(checkboxFinder);
    await tester.pumpAndSettle();
    await tester.tap(checkboxFinder);
    await tester.pumpAndSettle();

    expect(find.text('Stage Selected (1)'), findsOneWidget);

    // Trigger staging
    final stageBtn = find.byKey(const Key('stage_selected_catalog_button'));
    await tester.ensureVisible(stageBtn);
    await tester.pumpAndSettle();
    await tester.tap(stageBtn);
    await tester.pumpAndSettle();

    expect(stagedRequestItems, isNotNull);
    expect(stagedRequestItems!.single['external_id'], '1342');
    expect(find.text('Staged'), findsOneWidget);
    expect(fakeUpload.reloadCalls, greaterThanOrEqualTo(1));
  });

  testWidgets(
      'PluginConfigDialog preserves redacted secret sentinel unless replaced',
      (tester) async {
    setDesktopViewport(tester);

    Map<String, dynamic>? lastSavedBody;

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/humble-bundle/config' &&
          request.method == 'GET') {
        return http.Response(
          jsonEncode({
            'config': {
              'session_cookie': kPluginSecretRedactedSentinel,
              'preferred_format': 'epub',
            },
            'secret_fields_set': ['session_cookie'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/humble-bundle/config' &&
          request.method == 'PUT') {
        lastSavedBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'config': {
              'session_cookie': kPluginSecretRedactedSentinel,
              'preferred_format': lastSavedBody!['preferred_format'],
            },
            'secret_fields_set': ['session_cookie'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    const summary = PluginSummary(
      name: 'humble-bundle',
      displayName: 'Humble Bundle',
      version: '1.0.0',
      enabled: true,
      status: 'idle',
      capabilities: ['catalog'],
      permissions: PluginPermissions(networkDomains: ['www.humblebundle.com']),
      configSchema: {
        'type': 'object',
        'required': ['session_cookie'],
        'properties': {
          'session_cookie': {
            'type': 'string',
            'title': 'Session Cookie',
            'x-shelfd-secret': true,
            'x-shelfd-curl-paste': true,
          },
          'preferred_format': {
            'type': 'string',
            'title': 'Preferred Format',
            'enum': ['epub', 'pdf'],
            'default': 'epub',
          },
        },
      },
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: PluginConfigDialog(plugin: summary),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Configure Humble Bundle'), findsOneWidget);
    expect(find.text('Configured'), findsOneWidget);
    expect(find.text('Paste cURL'), findsOneWidget);

    // Save without touching secret -> preserves __SHELFD_REDACTED__
    await tester.tap(find.byKey(const Key('plugin_config_save_button')));
    await tester.pumpAndSettle();

    expect(lastSavedBody, isNotNull);
    expect(lastSavedBody!['session_cookie'], kPluginSecretRedactedSentinel);
    expect(lastSavedBody!['preferred_format'], 'epub');
  });

  testWidgets('SettingsView renders Plugins & Storefronts card and toggles plugin',
      (tester) async {
    tester.view.physicalSize = const Size(1280, 2800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    bool enableCalled = false;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(
          jsonEncode({
            'plugins': [
              {
                'name': 'project-gutenberg',
                'display_name': 'Project Gutenberg',
                'version': '0.1.0',
                'description': 'Public domain ebooks via Gutendex',
                'enabled': false,
                'status': 'disabled',
                'capabilities': ['catalog'],
              },
            ],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/project-gutenberg/enable' &&
          request.method == 'POST') {
        enableCalled = true;
        return http.Response(
          jsonEncode({
            'name': 'project-gutenberg',
            'display_name': 'Project Gutenberg',
            'version': '0.1.0',
            'enabled': true,
            'status': 'idle',
            'capabilities': ['catalog'],
          }),
          200,
        );
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
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final toggleFinder = find.byKey(const Key('plugin_toggle_project-gutenberg'));
    await tester.scrollUntilVisible(
      toggleFinder,
      300,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pumpAndSettle();

    expect(find.text('Plugins & Storefronts'), findsOneWidget);
    expect(find.text('Project Gutenberg v0.1.0'), findsOneWidget);

    await tester.tap(toggleFinder);
    await tester.pumpAndSettle();

    expect(enableCalled, isTrue);
  });

  testWidgets('PluginConfigDialog renders Account Linking section when plugin declares auth',
      (tester) async {
    setDesktopViewport(tester);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/yoto/auth/status') {
        return http.Response(
          jsonEncode({
            'linked': false,
            'client_id': 'yoto-dev-client',
            'methods': ['oauth_pkce', 'token'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    const summary = PluginSummary(
      name: 'yoto',
      displayName: 'Yoto Player',
      version: '0.1.0',
      enabled: true,
      status: 'idle',
      capabilities: ['actions'],
      auth: PluginAuthSpec(
        methods: ['oauth_pkce', 'token'],
        title: 'Yoto Account',
        description: 'Link your Yoto account via OAuth 2.0 PKCE.',
        fields: [
          PluginAuthFieldSpec(
            key: 'client_id',
            title: 'Yoto Client ID',
          ),
        ],
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: PluginConfigDialog(plugin: summary),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('plugin_auth_section_yoto')), findsOneWidget);
    expect(find.text('Not Linked'), findsOneWidget);
    expect(find.byKey(const Key('plugin_auth_oauth_begin_button')), findsOneWidget);
  });

  testWidgets('PluginConfigDialog hides x-shelfd-hidden config_schema properties',
      (tester) async {
    setDesktopViewport(tester);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/yoto/config') {
        return http.Response(
          jsonEncode({
            'config': {
              'client_id': 'yoto-client-123',
              'api_base_url': 'https://api.yotoplay.com',
              'access_token': '__SHELFD_REDACTED__',
            },
            'secret_fields_set': ['access_token'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/status') {
        return http.Response(
          jsonEncode({
            'linked': true,
            'client_id': 'yoto-client-123',
            'methods': ['oauth_pkce', 'token'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    const summary = PluginSummary(
      name: 'yoto',
      displayName: 'Yoto Player',
      version: '0.1.0',
      enabled: true,
      status: 'idle',
      capabilities: ['actions'],
      configSchema: {
        'type': 'object',
        'properties': {
          'client_id': {
            'type': 'string',
            'title': 'Yoto OAuth Client ID',
            'x-shelfd-hidden': true,
          },
          'api_base_url': {
            'type': 'string',
            'title': 'Yoto API Base URL',
            'default': 'https://api.yotoplay.com',
            'x-shelfd-hidden': true,
          },
          'access_token': {
            'type': 'string',
            'title': 'Yoto Access Token',
            'x-shelfd-secret': true,
            'x-shelfd-hidden': true,
          },
        },
      },
      auth: PluginAuthSpec(
        methods: ['oauth_pkce', 'token'],
        title: 'Yoto Account',
        description: 'Link your Yoto account via OAuth 2.0 PKCE.',
        fields: [
          PluginAuthFieldSpec(
            key: 'client_id',
            title: 'Yoto Client ID',
          ),
        ],
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: PluginConfigDialog(plugin: summary),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('plugin_auth_section_yoto')), findsOneWidget);
    expect(find.text('Yoto OAuth Client ID'), findsNothing);
    expect(find.text('Yoto API Base URL'), findsNothing);
    expect(find.text('Yoto Access Token'), findsNothing);
    expect(find.byKey(const Key('plugin_config_save_button')), findsNothing);
  });
}
