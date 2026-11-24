import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/background_job.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/background_tasks_ui.dart';
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
  testWidgets(
      'CatalogBrowserView stages asynchronously as a background task, locks in-flight items, and updates badges on completion',
      (tester) async {
    setDesktopViewport(tester);

    Map<String, dynamic>? stageRequestBody;
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
                'formats': [
                  {'format': 'epub', 'label': 'EPUB'},
                ],
                'status': 'available',
              },
            ],
            'total_count': 2,
          }),
          200,
        );
      }

      if (request.url.path ==
              '/api/v1/plugins/project-gutenberg/catalog/stage' &&
          request.method == 'POST') {
        stageRequestBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'batch_id': 'pjob-stage-99',
            'async': true,
            'job_id': 'pjob-stage-99',
            'message': 'Staging started in background',
            'job': {
              'id': 'pjob-stage-99',
              'user_id': 'u-1',
              'plugin': 'project-gutenberg',
              'action': 'catalog_stage',
              'title': 'Staging 2 books',
              'target_type': 'catalog',
              'target_id': '1342',
              'external_ids': ['1342', '84'],
              'status': 'running',
              'current': 0,
              'total': 2,
              'message': 'Starting...',
            },
          }),
          202,
        );
      }

      if (request.url.path == '/api/v1/jobs') {
        return http.Response(jsonEncode({'jobs': []}), 200);
      }
      if (request.url.path == '/api/v1/plugins/jobs') {
        return http.Response(jsonEncode({'jobs': []}), 200);
      }

      return http.Response('Not Found', 404);
    });

    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(api),
        queueProvider.overrideWith(_FakeQueueNotifier.new),
        uploadProvider.overrideWith(() => fakeUpload),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const CatalogBrowserView(initialPlugin: 'project-gutenberg'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Select both items
    await tester.tap(find.byKey(const Key('catalog_checkbox_1342')));
    await tester.tap(find.byKey(const Key('catalog_checkbox_84')));
    await tester.pumpAndSettle();
    expect(find.text('Stage Selected (2)'), findsOneWidget);

    // Click Stage Selected -> returns 202 Accepted immediately
    await tester.tap(find.byKey(const Key('stage_selected_catalog_button')));
    await tester.pumpAndSettle();

    expect(stageRequestBody?['async'], isTrue);
    // Selection cleared immediately and both cards locked with "Staging..." badge
    expect(find.text('Stage Selected (0)'), findsOneWidget);
    expect(find.text('Staging...'), findsNWidgets(2));
    expect(find.byKey(const Key('catalog_checkbox_1342')), findsNothing);
    expect(find.text('View Tasks'), findsOneWidget);

    // Job is immediately tracked in backgroundJobsProvider
    expect(container.read(backgroundJobsProvider).activeJobs.length, 1);

    // Simulate SSE job_updated completion (1342 staged, 84 failed)
    container.read(backgroundJobsProvider.notifier).upsertJob(
      const BackgroundJob(
        id: 'pjob-stage-99',
        kind: BackgroundJobKind.plugin,
        userId: 'u-1',
        title: 'Staging 2 books',
        status: 'completed',
        current: 2,
        total: 2,
        itemProgressPct: 100,
        message: 'Staged 1 of 2 books (1 failed)',
        metadata: {
          'plugin': 'project-gutenberg',
          'action': 'catalog_stage',
          'external_ids': ['1342', '84'],
          'result': {
            'status': 'completed',
            'message': 'Staged 1 of 2 books (1 failed)',
            'data': {
              'results': [
                {
                  'external_id': '1342',
                  'title': 'Pride and Prejudice',
                  'upload_job_id': 'up-1342',
                  'status': 'staged',
                },
                {
                  'external_id': '84',
                  'title': 'Frankenstein',
                  'status': 'failed',
                  'error': 'Upstream 503',
                },
              ],
            },
          },
        },
      ),
    );
    await tester.pumpAndSettle();

    // Succeeded item flips to Staged, failed item reverts to selectable checkbox
    expect(find.text('Staging...'), findsNothing);
    expect(find.text('Staged'), findsOneWidget);
    expect(find.byKey(const Key('catalog_checkbox_84')), findsOneWidget);
    expect(fakeUpload.reloadCalls, greaterThanOrEqualTo(1));
  });

  testWidgets(
      'BackgroundTasksPanel renders Review Queue action for completed catalog_stage job and navigates cleanly',
      (tester) async {
    setDesktopViewport(tester);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/jobs') {
        return http.Response(jsonEncode({'jobs': []}), 200);
      }
      return http.Response('Not Found', 404);
    });
    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(api),
        queueProvider.overrideWith(_FakeQueueNotifier.new),
      ],
    );
    addTearDown(container.dispose);

    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: Center(
              child: ElevatedButton(
                key: const Key('open_tasks_btn'),
                onPressed: () => showBackgroundTasksPanel(context),
                child: const Text('Open Tasks'),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/uploads',
          builder: (context, state) => const Scaffold(
            body: Center(child: Text('Uploads Destination')),
          ),
        ),
      ],
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          routerConfig: router,
        ),
      ),
    );
    container.read(backgroundJobsProvider);
    await tester.pumpAndSettle();

    container.read(backgroundJobsProvider.notifier).upsertJob(
      const BackgroundJob(
        id: 'pjob-done-1',
        kind: BackgroundJobKind.plugin,
        title: 'Staging "Frankenstein"',
        status: 'completed',
        current: 1,
        total: 1,
        itemProgressPct: 100,
        message: 'Staged 1 book for review',
        metadata: {
          'plugin': 'project-gutenberg',
          'action': 'catalog_stage',
        },
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('open_tasks_btn')));
    await tester.pumpAndSettle();

    final reviewBtn =
        find.byKey(const Key('background_task_review_queue_pjob-done-1'));
    expect(reviewBtn, findsOneWidget);

    await tester.tap(reviewBtn);
    await tester.pumpAndSettle();

    expect(find.text('Uploads Destination'), findsOneWidget);
  });

  testWidgets('OverDrive-style array card save sends flat list with typed secrets', (tester) async {
    setDesktopViewport(tester);
    Map<String, dynamic>? lastSavedBody;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'GET') {
        return http.Response(jsonEncode({'config': {}, 'secret_fields_set': []}), 200);
      }
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'PUT') {
        lastSavedBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(jsonEncode({'config': lastSavedBody!, 'secret_fields_set': ['libraries[*].password']}), 200);
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });
    final api = ApiService(baseUrl: 'http://localhost:8080', token: 'test-token-123', client: mockClient);
    const summary = PluginSummary(
      name: 'test-overdrive',
      displayName: 'Test OverDrive',
      version: '1.0.0',
      enabled: true,
      status: 'idle',
      capabilities: ['catalog'],
      configSchema: {
        'type': 'object',
        'required': ['libraries'],
        'properties': {
          'libraries': {
            'type': 'array',
            'items': {
              'type': 'object',
              'properties': {
                'subdomain': {'type': 'string', 'title': 'Library subdomain'},
                'username': {'type': 'string', 'title': 'Card number'},
                'password': {'type': 'string', 'title': 'Password', 'x-shelfd-secret': true, 'format': 'password'},
              },
              'required': ['subdomain', 'username', 'password'],
            },
          },
        },
      },
    );
    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(api),
        queueProvider.overrideWith(_FakeQueueNotifier.new),
      ],
      child: MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: const Scaffold(body: PluginConfigDialog(plugin: summary)),
      ),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('array_add_libraries')));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Library subdomain'),
        'jocolibrary');
    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Card number'),
        '025746613');
    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Password'),
        's3cret');
    await tester.tap(find.byKey(const Key('plugin_config_save_button')));
    await tester.pumpAndSettle();

    expect(lastSavedBody, isNotNull);
    expect(lastSavedBody!['libraries'], isA<List>());
    final items = lastSavedBody!['libraries'] as List;
    expect(items.length, 1);
    expect(items.first['subdomain'], 'jocolibrary');
    expect(items.first['username'], '025746613');
    expect(items.first['password'], 's3cret');
    expect(jsonEncode(lastSavedBody).contains(kPluginSecretRedactedSentinel), isFalse);
  });

  testWidgets('Blank array cards are dropped on save', (tester) async {
    setDesktopViewport(tester);
    Map<String, dynamic>? lastSavedBody;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'GET') {
        return http.Response(jsonEncode({'config': {}, 'secret_fields_set': []}), 200);
      }
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'PUT') {
        lastSavedBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(jsonEncode({'config': lastSavedBody!, 'secret_fields_set': []}), 200);
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });
    final api = ApiService(baseUrl: 'http://localhost:8080', token: 'test-token-123', client: mockClient);
    const summary = PluginSummary(
      name: 'test-overdrive',
      displayName: 'Test OverDrive',
      version: '1.0.0',
      enabled: true,
      status: 'idle',
      capabilities: ['catalog'],
      configSchema: {
        'type': 'object',
        'required': ['libraries'],
        'properties': {
          'libraries': {
            'type': 'array',
            'items': {
              'type': 'object',
              'properties': {
                'subdomain': {'type': 'string', 'title': 'Library subdomain'},
                'username': {'type': 'string', 'title': 'Card number'},
                'password': {'type': 'string', 'title': 'Password', 'x-shelfd-secret': true, 'format': 'password'},
              },
              'required': ['subdomain', 'username', 'password'],
            },
          },
        },
      },
    );
    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(api),
        queueProvider.overrideWith(_FakeQueueNotifier.new),
      ],
      child: MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: const Scaffold(body: PluginConfigDialog(plugin: summary)),
      ),
    ));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('array_add_libraries')));
    await tester.tap(find.byKey(const Key('array_add_libraries')));
    await tester.pumpAndSettle();
    // Fill only the first card (finders return first match = index 0).
    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Library subdomain').first,
        'jocolibrary');
    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Card number').first,
        '025746613');
    await tester.enterText(
        find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == 'Enter Password').first,
        's3cret');
    await tester.tap(find.byKey(const Key('plugin_config_save_button')));
    await tester.pumpAndSettle();
    expect(lastSavedBody, isNotNull);
    final items = lastSavedBody!['libraries'] as List;
    expect(items.length, 1);
    for (final item in items) {
      expect(item, isNotEmpty);
    }
  });

  testWidgets('Reopened array card keeps stored secret via sentinel, shows empty obscured field', (tester) async {
    setDesktopViewport(tester);
    Map<String, dynamic>? lastSavedBody;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'GET') {
        return http.Response(jsonEncode({
          'config': {'libraries': [{'subdomain': 'jocolibrary', 'username': '025746613', 'password': kPluginSecretRedactedSentinel}]},
          'secret_fields_set': ['libraries[*].password'],
        }), 200);
      }
      if (request.url.path == '/api/v1/plugins/test-overdrive/config' && request.method == 'PUT') {
        lastSavedBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(jsonEncode({'config': lastSavedBody!, 'secret_fields_set': ['libraries[*].password']}), 200);
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
      }
      return http.Response('Not Found', 404);
    });
    final api = ApiService(baseUrl: 'http://localhost:8080', token: 'test-token-123', client: mockClient);
    const summary = PluginSummary(
      name: 'test-overdrive',
      displayName: 'Test OverDrive',
      version: '1.0.0',
      enabled: true,
      status: 'idle',
      capabilities: ['catalog'],
      configSchema: {
        'type': 'object',
        'required': ['libraries'],
        'properties': {
          'libraries': {
            'type': 'array',
            'items': {
              'type': 'object',
              'properties': {
                'subdomain': {'type': 'string', 'title': 'Library subdomain'},
                'username': {'type': 'string', 'title': 'Card number'},
                'password': {'type': 'string', 'title': 'Password', 'x-shelfd-secret': true, 'format': 'password'},
              },
              'required': ['subdomain', 'username', 'password'],
            },
          },
        },
      },
    );
    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(api),
        queueProvider.overrideWith(_FakeQueueNotifier.new),
      ],
      child: MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: const Scaffold(body: PluginConfigDialog(plugin: summary)),
      ),
    ));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('array_remove_libraries.0')), findsOneWidget);
    expect(find.byKey(const Key('array_remove_libraries.1')), findsNothing);
    expect(find.text(kPluginSecretRedactedSentinel), findsNothing);

    await tester.tap(find.byKey(const Key('plugin_config_save_button')));
    await tester.pumpAndSettle();
    expect(lastSavedBody, isNotNull);
    final items = lastSavedBody!['libraries'] as List;
    expect(items.length, 1);
    expect(items.first['password'], kPluginSecretRedactedSentinel);
    expect(items.first['subdomain'], 'jocolibrary');
    expect(items.first['username'], '025746613');
  });
}
