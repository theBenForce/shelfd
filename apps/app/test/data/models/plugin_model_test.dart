import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/data/models/background_job.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/state/plugins_provider.dart';

void main() {
  group('Plugin & Catalog Models', () {
    test('PluginSummary and PluginDetail parse manifest, capabilities, auth, actions, and logs', () {
      final json = {
        'name': 'yoto',
        'display_name': 'Yoto Player',
        'version': '0.1.0',
        'description': 'Send audiobooks to Yoto MYO cards',
        'enabled': true,
        'daemon': false,
        'status': 'idle',
        'capabilities': ['actions', 'tools', 'events'],
        'api_scopes': ['books:read'],
        'permissions': {
          'network_domains': ['api.yotoplay.com', 'login.yotoplay.com'],
          'library_read': true,
          'library_write': false,
        },
        'config_schema': {
          'type': 'object',
          'properties': {
            'client_id': {'type': 'string', 'title': 'Client ID'},
          },
        },
        'auth': {
          'methods': ['oauth_pkce', 'token'],
          'title': 'Yoto Account',
          'description': 'Link via PKCE or token',
          'callback_param': 'code',
          'fields': [
            {'key': 'client_id', 'title': 'Yoto Client ID', 'secret': false},
            {'key': 'access_token', 'title': 'Access Token', 'secret': true},
          ],
        },
        'actions': [
          {
            'name': 'send_to_yoto',
            'title': 'Send to Yoto',
            'icon': 'speaker',
            'targets': ['book', 'series'],
            'media_types': ['audiobook'],
            'mode': 'form',
            'execution': 'job',
          },
        ],
        'commands': [
          {
            'name': 'verify_session',
            'title': 'Verify Session',
          },
        ],
        'catalog': {
          'modes': ['browse', 'search'],
          'supported_formats': ['epub'],
          'requires_auth': true,
        },
        'configured': true,
        'logs': ['[info] initialized ok', '[debug] idle reaper armed'],
      };

      final detail = PluginDetail.fromJson(json);
      expect(detail.summary.name, 'yoto');
      expect(detail.summary.displayName, 'Yoto Player');
      expect(detail.summary.hasAuth, isTrue);
      expect(detail.summary.hasActions, isTrue);
      expect(detail.summary.hasConfigSchema, isTrue);
      expect(detail.summary.auth!.supportsOAuthPkce, isTrue);
      expect(detail.summary.auth!.supportsToken, isTrue);
      expect(detail.summary.auth!.fields.length, 2);
      final action = detail.summary.actions.single;
      expect(action.name, 'send_to_yoto');
      expect(action.isForm, isTrue);
      expect(action.isJob, isTrue);
      expect(action.matches(targetType: 'book', mediaType: 'audiobook'), isTrue);
      expect(action.matches(targetType: 'book', mediaType: 'ebook'), isFalse);
      expect(
        action.matchesTarget('series', hasEpub: true, hasAudio: true),
        isTrue,
      );
      expect(
        action.matchesTarget('series', hasEpub: true, hasAudio: false),
        isFalse,
      );
      expect(detail.logs.length, 2);
    });

    test('PluginActionFormSpec, discriminated PluginInputFieldSpec, and visibility rules evaluate cleanly', () {
      final form = PluginActionFormSpec.fromJson({
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
            ],
          },
          {
            'key': 'playlist_title',
            'kind': 'text',
            'title': 'New Card Title',
            'default_value': 'The Hobbit',
            'visible_when': {'field': 'playlist_mode', 'equals': 'create'},
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
            'title': 'Include Cover Art',
            'default_value': true,
          },
          {
            'key': 'default_icon',
            'kind': 'asset_picker',
            'title': 'Default 16x16 Icon',
            'default_value': 'yoto:#default',
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
      });

      expect(form.title, 'Send "The Hobbit" to Yoto');
      expect(form.entries.length, 2);
      expect(form.fields.length, 5);
      expect(form.fields[0], isA<SegmentedFieldSpec>());
      expect(form.fields[1], isA<TextFieldSpec>());
      expect(form.fields[2], isA<RemoteSelectFieldSpec>());
      expect(form.fields[3], isA<BooleanFieldSpec>());
      expect(form.fields[4], isA<AssetPickerFieldSpec>());

      final valuesCreate = {'playlist_mode': 'create'};
      expect(form.fields[1].isVisible(valuesCreate), isTrue);
      expect(form.fields[2].isVisible(valuesCreate), isFalse);

      final valuesAppend = {'playlist_mode': 'append'};
      expect(form.fields[1].isVisible(valuesAppend), isFalse);
      expect(form.fields[2].isVisible(valuesAppend), isTrue);
    });

    test('PluginActionResult and PluginJob parse statuses and progress', () {
      final handoff = PluginActionResult.fromJson({
        'status': 'client_handoff',
        'open_url': 'https://notebooklm.google.com/',
        'download_api_path': '/api/v1/books/b1/markdown',
        'file_name': 'Dune.md',
        'mime_type': 'text/markdown; charset=utf-8',
        'message': 'Prepared Dune.md',
      });
      expect(handoff.isClientHandoff, isTrue);
      expect(handoff.downloadApiPath, '/api/v1/books/b1/markdown');
      expect(handoff.openUrl, 'https://notebooklm.google.com/');

      final jobStarted = PluginActionResult.fromJson({
        'status': 'job_started',
        'job_id': 'pjob-1',
        'job': {
          'id': 'pjob-1',
          'user_id': 'u1',
          'plugin': 'yoto',
          'action': 'send_to_yoto',
          'title': 'The Hobbit',
          'target_type': 'book',
          'target_id': 'b-hobbit',
          'status': 'running',
          'current': 1,
          'total': 2,
          'item_title': 'Chapter 1',
          'item_progress_pct': 50.0,
          'message': 'Uploading Chapter 1',
        },
      });
      expect(jobStarted.isJobStarted, isTrue);
      expect(jobStarted.job, isNotNull);
      expect(jobStarted.job!.isActive, isTrue);
      expect(jobStarted.job!.progressFraction, closeTo(0.5, 0.01));
    });

    test('RFC 7636 PKCE S256 challenge matches standard test vector', () {
      const verifier = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
      final challenge = computePkceS256Challenge(verifier);
      expect(challenge, 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
    });

    test('CatalogPage and CatalogStageResult parse provenance statuses', () {
      final page = CatalogPage.fromJson({
        'items': [
          {
            'external_id': 'pg-1342',
            'title': 'Pride and Prejudice',
            'authors': ['Jane Austen'],
            'group': 'Classic Literature',
            'formats': [
              {'format': 'epub', 'label': 'EPUB'},
            ],
            'status': 'available',
          },
          {
            'external_id': 'pg-84',
            'title': 'Frankenstein',
            'authors': ['Mary Shelley'],
            'status': 'in_library',
            'book_id': 'book-uuid-84',
          },
        ],
        'groups': [
          {'id': 'fiction', 'label': 'Fiction', 'count': 42},
        ],
        'next_cursor': '2',
        'total_count': 100,
      });

      expect(page.hasMore, isTrue);
      expect(page.totalCount, 100);
      expect(page.groups.single.displayLabel, 'Fiction (42)');
      expect(page.items.first.isAvailable, isTrue);
      expect(page.items.last.isInLibrary, isTrue);
      expect(page.items.last.bookId, 'book-uuid-84');

      final stageRes = CatalogStageResult.fromJson({
        'async': false,
        'results': [
          {
            'external_id': 'pg-1342',
            'title': 'Pride and Prejudice',
            'upload_job_id': 'job-1342',
            'status': 'staged',
          },
        ],
      });
      expect(stageRes.isAsync, isFalse);
      expect(stageRes.results.single.uploadJobId, 'job-1342');
      expect(stageRes.results.single.isFailed, isFalse);

      final asyncStageRes = CatalogStageResult.fromJson({
        'batch_id': 'pjob-stage-1',
        'async': true,
        'job_id': 'pjob-stage-1',
        'message': 'Staging started in background',
        'job': {
          'id': 'pjob-stage-1',
          'user_id': 'u-1',
          'plugin': 'project-gutenberg',
          'action': 'catalog_stage',
          'title': 'Staging "Frankenstein"',
          'target_type': 'catalog',
          'target_id': '84',
          'external_ids': ['84'],
          'status': 'running',
          'current': 1,
          'total': 1,
          'item_progress_pct': 0,
          'message': 'Downloading...',
        },
      });
      expect(asyncStageRes.isAsync, isTrue);
      expect(asyncStageRes.jobId, 'pjob-stage-1');
      expect(asyncStageRes.job, isNotNull);
      expect(asyncStageRes.job!.externalIds, ['84']);

      // Verify 1-book active plugin job without percent does not prematurely jump to 100%
      final bgSingle = BackgroundJob.fromPluginJob(asyncStageRes.job!);
      expect(bgSingle.isIndeterminate, isTrue);
      expect(bgSingle.progressFraction, 0.0);
      expect(bgSingle.metadata?['external_ids'], ['84']);

      // Once percent is reported on 1-book plugin job, it becomes determinate
      final bgSingleWithPct = BackgroundJob.fromPluginJob(
        PluginJob.fromJson({
          'id': 'pjob-stage-1',
          'plugin': 'project-gutenberg',
          'action': 'catalog_stage',
          'title': 'Staging "Frankenstein"',
          'status': 'running',
          'current': 1,
          'total': 1,
          'item_progress_pct': 65.0,
        }),
      );
      expect(bgSingleWithPct.isIndeterminate, isFalse);
      expect(bgSingleWithPct.progressFraction, closeTo(0.65, 0.01));

      // Multi-book batch uses (completedItems + itemProgressPct/100) / total
      final bgBatch = BackgroundJob.fromPluginJob(
        PluginJob.fromJson({
          'id': 'pjob-stage-2',
          'plugin': 'project-gutenberg',
          'action': 'catalog_stage',
          'title': 'Staging 2 books',
          'status': 'running',
          'current': 2,
          'total': 2,
          'item_progress_pct': 50.0,
        }),
      );
      expect(bgBatch.isIndeterminate, isFalse);
      expect(bgBatch.progressFraction, closeTo(0.75, 0.01));
    });
  });

  group('ApiService Plugin Endpoints & SSE', () {
    test('ApiService calls plugin catalog, config, auth, actions, jobs, and decodes plugin_event SSE', () async {
      Map<String, dynamic>? savedConfigPayload;
      Map<String, dynamic>? stagePayload;
      Map<String, dynamic>? invokePayload;

      final mockClient = MockClient.streaming((request, bodyStream) async {
        final bodyBytes = await bodyStream.toBytes();
        final bodyStr = utf8.decode(bodyBytes);

        if (request.url.path == '/api/v1/plugins') {
          final bytes = utf8.encode(jsonEncode({
            'plugins': [
              {
                'name': 'project-gutenberg',
                'display_name': 'Project Gutenberg',
                'version': '0.1.0',
                'enabled': true,
                'status': 'idle',
                'capabilities': ['catalog', 'tools'],
              },
            ],
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/project-gutenberg/config' &&
            request.method == 'PUT') {
          savedConfigPayload = jsonDecode(bodyStr) as Map<String, dynamic>;
          final bytes = utf8.encode(jsonEncode({
            'config': {'session_cookie': kPluginSecretRedactedSentinel},
            'secret_fields_set': ['session_cookie'],
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/project-gutenberg/catalog/stage' &&
            request.method == 'POST') {
          stagePayload = jsonDecode(bodyStr) as Map<String, dynamic>;
          final bytes = utf8.encode(jsonEncode({
            'async': false,
            'results': [
              {
                'external_id': '1342',
                'title': 'Pride and Prejudice',
                'upload_job_id': 'job-1',
                'status': 'staged',
              },
            ],
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/yoto/auth/status') {
          final bytes = utf8.encode(jsonEncode({
            'linked': true,
            'account_label': 'Yoto Account',
            'client_id': 'cid-1',
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/yoto/actions/send_to_yoto/prepare') {
          final bytes = utf8.encode(jsonEncode({
            'title': 'Send to Yoto',
            'submit_label': 'Send',
            'fields': [
              {'key': 'playlist_title', 'kind': 'text', 'title': 'Title'},
            ],
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/yoto/actions/send_to_yoto/invoke') {
          invokePayload = jsonDecode(bodyStr) as Map<String, dynamic>;
          final bytes = utf8.encode(jsonEncode({
            'status': 'job_started',
            'job_id': 'pjob-99',
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/plugins/jobs') {
          final bytes = utf8.encode(jsonEncode({
            'jobs': [
              {
                'id': 'pjob-99',
                'plugin': 'yoto',
                'action': 'send_to_yoto',
                'title': 'The Hobbit',
                'status': 'running',
                'current': 1,
                'total': 1,
              },
            ],
          }));
          return http.StreamedResponse(Stream.value(bytes), 200);
        }

        if (request.url.path == '/api/v1/queue/events') {
          const sse = 'event: plugin_event\n'
              'data: {"plugin":"project-gutenberg","kind":"progress","data":{"message":"Downloading EPUB"}}\n\n';
          return http.StreamedResponse(Stream.value(utf8.encode(sse)), 200);
        }

        return http.StreamedResponse(Stream.value(utf8.encode('Not Found')), 404);
      });

      final api = ApiService(
        baseUrl: 'http://localhost:8080',
        token: 'jwt-123',
        client: mockClient,
      );

      final plugins = await api.getPlugins();
      expect(plugins.single.name, 'project-gutenberg');

      final cfg = await api.updatePluginConfig('project-gutenberg', {
        'session_cookie': kPluginSecretRedactedSentinel,
      });
      expect(savedConfigPayload?['session_cookie'], kPluginSecretRedactedSentinel);
      expect(cfg.secretFieldsSet, contains('session_cookie'));

      final stageRes = await api.stagePluginCatalogItems(
        'project-gutenberg',
        const [
          CatalogStageItemRequest(
            externalId: '1342',
            title: 'Pride and Prejudice',
            preferredFormat: 'epub',
          ),
        ],
        async: true,
      );
      expect(stagePayload?['async'], isTrue);
      expect((stagePayload?['items'] as List).first['external_id'], '1342');
      expect((stagePayload?['items'] as List).first['title'], 'Pride and Prejudice');
      expect(stageRes.results.single.uploadJobId, 'job-1');

      final authStatus = await api.getPluginAuthStatus('yoto');
      expect(authStatus.linked, isTrue);
      expect(authStatus.clientId, 'cid-1');

      final form = await api.preparePluginAction(
        'yoto',
        'send_to_yoto',
        target: const PluginActionTarget(type: 'book', id: 'b1', title: 'Hobbit'),
      );
      expect(form.fields.single.key, 'playlist_title');

      final invokeRes = await api.invokePluginAction(
        'yoto',
        'send_to_yoto',
        target: const PluginActionTarget(type: 'book', id: 'b1'),
        input: {'playlist_title': 'Hobbit Card'},
      );
      expect(invokePayload?['values']?['playlist_title'], 'Hobbit Card');
      expect(invokeRes.isJobStarted, isTrue);
      expect(invokeRes.jobId, 'pjob-99');

      final jobs = await api.getPluginJobs();
      expect(jobs.single.id, 'pjob-99');

      final events = await api.streamEvents().toList();
      expect(events.single, isA<PluginEvent>());
      final pEvt = events.single as PluginEvent;
      expect(pEvt.data['plugin'], 'project-gutenberg');
    });

    test('PluginInstallResult, formatInstallMessage, and updatePluginZip work end-to-end', () async {
      final fresh = PluginInstallResult.fromJson({
        'name': 'yoto',
        'display_name': 'Yoto Player',
        'version': '1.0.0',
        'enabled': true,
        'status': 'idle',
        'updated': false,
      });
      expect(fresh.updated, isFalse);
      expect(fresh.previousVersion, isNull);
      expect(
        PluginsNotifier.formatInstallMessage(fresh),
        'Installed Yoto Player v1.0.0.',
      );

      final upgraded = PluginInstallResult.fromJson({
        'name': 'yoto',
        'display_name': 'Yoto Player',
        'version': '1.1.0',
        'enabled': true,
        'status': 'idle',
        'updated': true,
        'previous_version': '1.0.0',
      });
      expect(upgraded.updated, isTrue);
      expect(upgraded.previousVersion, '1.0.0');
      expect(
        PluginsNotifier.formatInstallMessage(upgraded),
        'Updated Yoto Player v1.0.0 → v1.1.0.',
      );

      final reinstalled = PluginInstallResult.fromJson({
        'name': 'yoto',
        'display_name': 'Yoto Player',
        'version': '1.1.0',
        'enabled': true,
        'status': 'idle',
        'updated': true,
        'previous_version': '1.1.0',
      });
      expect(
        PluginsNotifier.formatInstallMessage(reinstalled),
        'Reinstalled Yoto Player v1.1.0.',
      );

      final visitedPaths = <String>[];
      final mockClient = MockClient((request) async {
        visitedPaths.add('${request.method} ${request.url.path}');
        if (request.url.path == '/api/v1/plugins/install') {
          return http.Response(
            jsonEncode({
              'name': 'yoto',
              'display_name': 'Yoto Player',
              'version': '1.0.0',
              'enabled': true,
              'status': 'idle',
              'updated': false,
            }),
            201,
          );
        }
        if (request.url.path == '/api/v1/plugins/yoto/update') {
          return http.Response(
            jsonEncode({
              'name': 'yoto',
              'display_name': 'Yoto Player',
              'version': '1.1.0',
              'enabled': true,
              'status': 'idle',
              'updated': true,
              'previous_version': '1.0.0',
            }),
            200,
          );
        }
        return http.Response('not found', 404);
      });

      final api = ApiService(
        baseUrl: 'http://localhost:8080',
        token: 'jwt-123',
        client: mockClient,
      );

      final installRes = await api.installPluginZip(
        filename: 'yoto-1.0.0.zip',
        bytes: const [80, 75, 3, 4],
      );
      expect(installRes.updated, isFalse);
      expect(installRes.plugin.version, '1.0.0');

      final updateRes = await api.updatePluginZip(
        name: 'yoto',
        filename: 'yoto-1.1.0.zip',
        bytes: const [80, 75, 3, 4],
      );
      expect(updateRes.updated, isTrue);
      expect(updateRes.previousVersion, '1.0.0');
      expect(updateRes.plugin.version, '1.1.0');
      expect(visitedPaths, [
        'POST /api/v1/plugins/install',
        'POST /api/v1/plugins/yoto/update',
      ]);
    });
  });
}
