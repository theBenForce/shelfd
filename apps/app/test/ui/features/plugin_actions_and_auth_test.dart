import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/background_job.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/plugins/plugin_action_dialog.dart';
import 'package:shelf/ui/features/plugins/plugin_action_runner.dart';
import 'package:shelf/ui/features/settings/plugin_config_dialog.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeQueueNotifier extends QueueNotifier {
  @override
  QueueState build() => const QueueState();

  @override
  Future<void> refresh() async {}
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

  const yotoPlugin = PluginSummary(
    name: 'yoto',
    displayName: 'Yoto Player',
    version: '0.1.0',
    enabled: true,
    status: 'idle',
    capabilities: ['actions', 'tools', 'events'],
    auth: PluginAuthSpec(
      methods: ['oauth_pkce', 'token'],
      title: 'Yoto Account',
      description: 'Link your Yoto account via OAuth 2.0 PKCE or access token.',
      fields: [
        PluginAuthFieldSpec(
          key: 'client_id',
          title: 'Yoto Client ID',
          placeholder: 'Optional custom OAuth Client ID',
        ),
        PluginAuthFieldSpec(
          key: 'access_token',
          title: 'Access Token',
          secret: true,
        ),
        PluginAuthFieldSpec(
          key: 'refresh_token',
          title: 'Refresh Token (Optional)',
          secret: true,
        ),
      ],
    ),
    actions: [
      PluginActionDef(
        name: 'send_to_yoto',
        title: 'Send to Yoto',
        icon: 'speaker',
        targets: ['book', 'series'],
        mediaTypes: ['audiobook'],
        mode: 'form',
        execution: 'job',
      ),
    ],
  );

  testWidgets(
      'PluginActionDialog renders conditional fields, remote_select, asset_picker, per-entry overrides, and submits invoke payload',
      (tester) async {
    setDesktopViewport(tester);

    Map<String, dynamic>? invokeBody;

    final mockClient = MockClient((request) async {
      if (request.url.path ==
          '/api/v1/plugins/yoto/actions/send_to_yoto/prepare') {
        return http.Response(
          jsonEncode({
            'title': 'Send "The Hobbit" to Yoto',
            'submit_label': 'Send to Yoto',
            'auth_required': false,
            'entries': [
              {'id': '0', 'index': 0, 'title': 'An Unexpected Party'},
              {'id': '1', 'index': 1, 'title': 'Roast Mutton'},
            ],
            'fields': [
              {
                'key': 'playlist_mode',
                'kind': 'segmented',
                'title': 'Destination Card',
                'default_value': 'create',
                'options': [
                  {'value': 'create', 'label': 'Create New Card'},
                  {'value': 'append', 'label': 'Add to Existing'},
                  {'value': 'replace', 'label': 'Replace Existing'},
                ],
              },
              {
                'key': 'playlist_title',
                'kind': 'text',
                'title': 'New Card Title',
                'default_value': 'The Hobbit',
                'visible_when': {
                  'field': 'playlist_mode',
                  'equals': 'create',
                },
              },
              {
                'key': 'card_id',
                'kind': 'remote_select',
                'title': 'Target MYO Playlist',
                'options_source': 'playlists',
                'searchable': true,
                'visible_when': {
                  'field': 'playlist_mode',
                  'in': ['append', 'replace'],
                },
              },
              {
                'key': 'include_cover',
                'kind': 'boolean',
                'title': 'Include Book Cover Art',
                'default_value': true,
              },
              {
                'key': 'default_icon',
                'kind': 'asset_picker',
                'title': 'Default 16x16 Chapter Icon',
                'default_value': 'yoto:#default-book',
                'options_source': 'icons',
                'asset_shape': 'square_pixel',
                'allow_upload': true,
                'tabs': [
                  {'id': 'public', 'label': 'Public Icons', 'searchable': true},
                  {'id': 'custom', 'label': 'My Icons', 'searchable': true},
                ],
                'per_entry_override': true,
                'override_key': 'chapter_icons',
                'override_label': 'Customize Per-Chapter Icons',
              },
            ],
          }),
          200,
        );
      }

      if (request.url.path ==
              '/api/v1/plugins/yoto/actions/send_to_yoto/options' &&
          request.url.queryParameters['source'] == 'playlists') {
        return http.Response(
          jsonEncode({
            'items': [
              {
                'value': 'card-bedtime',
                'label': 'Bedtime Stories',
                'subtitle': '4 chapters',
                'count': 4,
              },
              {
                'value': 'card-roadtrip',
                'label': 'Road Trip Adventures',
                'subtitle': '2 chapters',
                'count': 2,
              },
            ],
          }),
          200,
        );
      }

      if (request.url.path ==
              '/api/v1/plugins/yoto/actions/send_to_yoto/options' &&
          request.url.queryParameters['source'] == 'icons') {
        final tab = request.url.queryParameters['tab'];
        if (tab == 'custom') {
          return http.Response(
            jsonEncode({
              'items': [
                {
                  'value': 'yoto:#custom-star',
                  'label': 'Custom Star',
                  'icon_uri': 'yoto:#custom-star',
                },
              ],
            }),
            200,
          );
        }
        return http.Response(
          jsonEncode({
            'items': [
              {
                'value': 'yoto:#dragon-icon',
                'label': 'Dragon',
                'icon_uri': 'yoto:#dragon-icon',
              },
            ],
          }),
          200,
        );
      }

      if (request.url.path ==
          '/api/v1/plugins/yoto/actions/send_to_yoto/invoke') {
        invokeBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'status': 'job_started',
            'job_id': 'pjob-hobbit-1',
            'message': 'Started Yoto upload job',
            'job': {
              'id': 'pjob-hobbit-1',
              'plugin': 'yoto',
              'action': 'send_to_yoto',
              'title': 'The Hobbit',
              'status': 'running',
              'current': 0,
              'total': 2,
              'message': 'Starting...',
            },
          }),
          200,
        );
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

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: PluginActionDialog(
              plugin: yotoPlugin,
              action: yotoPlugin.actions.first,
              target: const PluginActionTarget(
                type: 'book',
                id: 'book-hobbit',
                title: 'The Hobbit',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Send "The Hobbit" to Yoto'), findsOneWidget);
    // Initially in 'create' mode: New Card Title is visible, Target MYO Playlist is hidden
    expect(
      find.byKey(const Key('plugin_action_field_playlist_title')),
      findsOneWidget,
    );
    expect(
      find.byKey(const Key('plugin_action_field_card_id')),
      findsNothing,
    );

    // Switch to 'Add to Existing' ('append')
    await tester.tap(find.text('Add to Existing'));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('plugin_action_field_playlist_title')),
      findsNothing,
    );
    expect(
      find.byKey(const Key('plugin_action_field_card_id')),
      findsOneWidget,
    );
    expect(find.text('Bedtime Stories'), findsOneWidget);
    expect(find.text('4 chapters'), findsOneWidget);
    expect(find.text('Road Trip Adventures'), findsOneWidget);

    // Select Road Trip Adventures
    await tester.tap(
      find.byKey(const Key('plugin_action_option_card_id_card-roadtrip')),
    );
    await tester.pumpAndSettle();

    // Open per-entry overrides tile and pick an icon for Chapter 1
    final overridesTile = find.byKey(
      const Key('plugin_action_overrides_tile_default_icon'),
    );
    await tester.ensureVisible(overridesTile);
    await tester.pumpAndSettle();
    await tester.tap(overridesTile);
    await tester.pumpAndSettle();

    expect(find.text('1. An Unexpected Party'), findsOneWidget);
    expect(find.text('2. Roast Mutton'), findsOneWidget);

    final entryAssetBtn = find.byKey(
      const Key('plugin_action_entry_asset_default_icon_0'),
    );
    await tester.ensureVisible(entryAssetBtn);
    await tester.pumpAndSettle();
    await tester.tap(entryAssetBtn);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('plugin_asset_picker_dialog')), findsOneWidget);
    expect(find.text('Dragon'), findsOneWidget);

    // Switch to My Icons tab
    await tester.tap(find.text('My Icons'));
    await tester.pumpAndSettle();
    expect(find.text('Custom Star'), findsOneWidget);

    await tester.tap(
      find.byKey(const Key('plugin_asset_tile_yoto:#custom-star')),
    );
    await tester.pumpAndSettle();

    // Submit the form
    await tester.tap(find.byKey(const Key('plugin_action_submit_button')));
    await tester.pumpAndSettle();

    expect(invokeBody, isNotNull);
    final vals = invokeBody!['values'] as Map<String, dynamic>;
    expect(vals['playlist_mode'], 'append');
    expect(vals['card_id'], 'card-roadtrip');
    final ov = invokeBody!['overrides'] as Map<String, dynamic>;
    expect(ov['0'], 'yoto:#custom-star');
  });

  testWidgets(
      'PluginActionDialog renders unlinked account CTA and opens PluginConfigDialog directly',
      (tester) async {
    setDesktopViewport(tester);

    bool isLinked = false;
    final mockClient = MockClient((request) async {
      if (request.url.path ==
          '/api/v1/plugins/yoto/actions/send_to_yoto/prepare') {
        return http.Response(
          jsonEncode({
            'title': 'Send "Peter Rabbit" to Yoto',
            'submit_label': 'Send to Yoto',
            'auth_required': !isLinked,
            'auth_message':
                'Link your Yoto account in plugin settings before sending audio to Yoto.',
            'fields': [
              {
                'key': 'playlist_title',
                'kind': 'text',
                'title': 'New Card Title',
                'default_value': 'Peter Rabbit',
              },
            ],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/status') {
        return http.Response(
          jsonEncode({
            'linked': isLinked,
            'client_id': 'shelfd-yoto',
            'methods': ['oauth_pkce', 'token'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/complete') {
        isLinked = true;
        return http.Response(
          jsonEncode({
            'linked': true,
            'account_label': 'Yoto Account',
            'client_id': 'shelfd-yoto',
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins') {
        return http.Response(jsonEncode({'plugins': []}), 200);
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

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
          queueProvider.overrideWith(_FakeQueueNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: PluginActionDialog(
              plugin: yotoPlugin,
              action: yotoPlugin.actions.first,
              target: const PluginActionTarget(
                type: 'book',
                id: 'book-peter',
                title: 'Peter Rabbit',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Yoto Player Account Not Linked'), findsOneWidget);
    final configBtn =
        find.byKey(const Key('plugin_action_configure_auth_button'));
    expect(configBtn, findsOneWidget);

    await tester.tap(configBtn);
    await tester.pumpAndSettle();

    // PluginConfigDialog opens with Account Linking section
    expect(find.byKey(const Key('plugin_auth_section_yoto')), findsOneWidget);

    // Link via authorization code
    await tester.enterText(
      find.byKey(const Key('plugin_auth_code_field')),
      'mock-auth-code-123',
    );
    await tester.tap(find.byKey(const Key('plugin_auth_complete_code_button')));
    await tester.pumpAndSettle();

    expect(find.text('Connected'), findsOneWidget);

    // Close PluginConfigDialog -> PluginActionDialog reloads prepare and shows form fields
    await tester.tap(find.text('Cancel').last);
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('plugin_action_field_playlist_title')),
      findsOneWidget,
    );
  });

  testWidgets(
      'PluginConfigDialog supports PKCE S256 begin, token linking, and disconnect',
      (tester) async {
    setDesktopViewport(tester);

    bool isLinked = false;
    Map<String, dynamic>? beginPayload;
    Map<String, dynamic>? tokenPayload;

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/plugins/yoto/auth/status') {
        return http.Response(
          jsonEncode({
            'linked': isLinked,
            'client_id': 'yoto-client',
            'methods': ['oauth_pkce', 'token'],
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/begin') {
        beginPayload = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'authorize_url': 'https://login.yotoplay.com/authorize?code_challenge=xyz',
            'client_id': 'yoto-client',
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/complete') {
        tokenPayload = jsonDecode(request.body) as Map<String, dynamic>;
        isLinked = true;
        return http.Response(
          jsonEncode({
            'linked': true,
            'account_label': 'Yoto Account',
            'client_id': 'yoto-client',
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/plugins/yoto/auth/disconnect') {
        isLinked = false;
        return http.Response(
          jsonEncode({
            'linked': false,
            'client_id': 'yoto-client',
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
            body: PluginConfigDialog(plugin: yotoPlugin),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Tap Connect with OAuth (PKCE)
    await tester.tap(find.byKey(const Key('plugin_auth_oauth_begin_button')));
    await tester.pumpAndSettle();

    expect(beginPayload, isNotNull);
    expect(beginPayload!['method'], 'oauth_pkce');
    expect((beginPayload!['code_challenge'] as String).isNotEmpty, isTrue);
    final savedVerifier = prefs.getString('plugin_pkce_yoto');
    expect(savedVerifier, isNotNull);
    expect(savedVerifier!.length, 64);

    // Toggle manual token entry and link with access token
    await tester.tap(find.byKey(const Key('plugin_auth_toggle_token_button')));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('plugin_auth_field_access_token')),
      'direct-yoto-token-abc',
    );
    await tester.tap(
      find.byKey(const Key('plugin_auth_complete_token_button')),
    );
    await tester.pumpAndSettle();

    expect(tokenPayload?['method'], 'token');
    expect(tokenPayload?['access_token'], 'direct-yoto-token-abc');
    expect(find.text('Connected'), findsOneWidget);

    // Disconnect
    await tester.tap(find.byKey(const Key('plugin_auth_disconnect_button')));
    await tester.pumpAndSettle();
    expect(find.text('Not Linked'), findsOneWidget);
  });

  testWidgets(
      'BackgroundTasks UI renders live job updates, keeps completed history until cleared, and shows error toast on failure',
      (tester) async {
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/jobs') {
        if (request.method == 'DELETE') {
          return http.Response(jsonEncode({'cleared': 1}), 200);
        }
        return http.Response(jsonEncode({'jobs': []}), 200);
      }
      return http.Response('Not Found', 404);
    });
    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'test-token-123',
      client: mockClient,
    );

    late WidgetRef capturedRef;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: Consumer(
              builder: (context, ref, _) {
                capturedRef = ref;
                return const Column(
                  children: [
                    BackgroundTasksSidebarButton(),
                    Expanded(child: BackgroundTasksPanelContent()),
                    BackgroundTasksErrorListener(),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Background Tasks'), findsWidgets);
    expect(find.text('All quiet'), findsOneWidget);

    final notifier = capturedRef.read(backgroundJobsProvider.notifier);
    notifier.upsertJob(
      const BackgroundJob(
        id: 'pjob-1',
        kind: BackgroundJobKind.plugin,
        title: 'The Hobbit',
        itemTitle: 'Chapter 1',
        status: 'running',
        current: 1,
        total: 2,
        itemProgressPct: 50.0,
        message: 'Uploading Chapter 1 (1/2)',
        metadata: {'plugin': 'yoto', 'action': 'send_to_yoto'},
      ),
    );
    await tester.pump();

    expect(
      find.byKey(const Key('background_tasks_sidebar_progress')),
      findsOneWidget,
    );
    expect(find.byKey(const Key('background_task_card_pjob-1')), findsOneWidget);
    expect(find.textContaining('Uploading Chapter 1 (1/2)'), findsWidgets);

    // Complete job -> stays in recent history until cleared
    notifier.upsertJob(
      const BackgroundJob(
        id: 'pjob-1',
        kind: BackgroundJobKind.plugin,
        title: 'The Hobbit',
        status: 'completed',
        current: 2,
        total: 2,
        itemProgressPct: 100.0,
        message: 'Sent to Yoto',
        metadata: {'plugin': 'yoto', 'action': 'send_to_yoto'},
      ),
    );
    await tester.pump();

    expect(find.text('1 task completed'), findsOneWidget);
    expect(find.byKey(const Key('background_task_card_pjob-1')), findsOneWidget);
    expect(
      find.byKey(const Key('background_tasks_clear_finished')),
      findsOneWidget,
    );

    // Emit failed job -> shows error toast
    notifier.upsertJob(
      const BackgroundJob(
        id: 'pjob-2',
        kind: BackgroundJobKind.plugin,
        title: 'The Fellowship of the Ring',
        status: 'failed',
        error: 'Yoto API rate limit exceeded',
        metadata: {'plugin': 'yoto', 'action': 'send_to_yoto'},
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.byKey(const Key('background_task_error_toast')), findsOneWidget);
    expect(
      find.textContaining('The Fellowship of the Ring failed: Yoto API rate limit exceeded'),
      findsOneWidget,
    );
  });

  testWidgets(
      'PluginActionRunner executes immediate client_handoff action by downloading Markdown and showing toast',
      (tester) async {
    bool downloadedMarkdown = false;
    final mockClient = MockClient((request) async {
      if (request.url.path ==
          '/api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke') {
        return http.Response(
          jsonEncode({
            'status': 'client_handoff',
            'open_url': 'https://notebooklm.google.com/',
            'download_api_path': '/api/v1/books/book-dune/markdown',
            'file_name': 'Dune.md',
            'message': 'Prepared "Dune.md" for NotebookLM.',
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/books/book-dune/markdown') {
        downloadedMarkdown = true;
        return http.Response('# Dune\nArrakis', 200);
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

    const nlmPlugin = PluginSummary(
      name: 'notebooklm',
      displayName: 'NotebookLM',
      version: '0.1.0',
      enabled: true,
      status: 'idle',
      capabilities: ['actions'],
      actions: [
        PluginActionDef(
          name: 'send_to_notebooklm',
          title: 'Send to NotebookLM',
          icon: 'auto_awesome',
          targets: ['book'],
          mediaTypes: ['ebook'],
          mode: 'immediate',
          execution: 'sync',
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(api),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: Consumer(
              builder: (context, ref, _) => ElevatedButton(
                onPressed: () {
                  PluginActionRunner.runPluginAction(
                    context,
                    ref,
                    plugin: nlmPlugin,
                    action: nlmPlugin.actions.first,
                    target: const PluginActionTarget(
                      type: 'book',
                      id: 'book-dune',
                      title: 'Dune',
                    ),
                  );
                },
                child: const Text('Run NotebookLM'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Run NotebookLM'));
    await tester.pumpAndSettle();

    expect(downloadedMarkdown, isTrue);
    expect(find.text('Prepared "Dune.md" for NotebookLM.'), findsOneWidget);
  });
}
