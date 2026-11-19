import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/services/api_service.dart';

void main() {
  group('Plugin & Catalog Models', () {
    test('PluginSummary and PluginDetail parse manifest, capabilities, and logs', () {
      final json = {
        'name': 'humble-bundle',
        'display_name': 'Humble Bundle',
        'version': '1.2.0',
        'description': 'Import purchased Humble Bundle ebooks',
        'enabled': true,
        'daemon': false,
        'status': 'idle',
        'capabilities': ['catalog', 'commands'],
        'catalog_modes': ['browse', 'search'],
        'supported_formats': ['epub', 'm4b'],
        'api_scopes': ['uploads:stage'],
        'permissions': {
          'network_domains': ['www.humblebundle.com', 'dl.humble.com'],
          'library_read': false,
          'library_write': false,
        },
        'config_schema': {
          'type': 'object',
          'required': ['session_cookie'],
          'properties': {
            'session_cookie': {
              'type': 'string',
              'title': 'Session Cookie',
              'x-shelfd-secret': true,
              'x-shelfd-curl-paste': true,
            },
          },
        },
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
      expect(detail.summary.name, 'humble-bundle');
      expect(detail.summary.displayName, 'Humble Bundle');
      expect(detail.summary.hasCatalog, isTrue);
      expect(detail.summary.hasConfigSchema, isTrue);
      expect(detail.summary.resolvedCatalogModes, ['browse', 'search']);
      expect(detail.summary.permissions.networkDomains, [
        'www.humblebundle.com',
        'dl.humble.com',
      ]);
      expect(detail.summary.commands.single.displayTitle, 'Verify Session');
      expect(detail.logs.length, 2);
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
    });
  });

  group('ApiService Plugin Endpoints & SSE', () {
    test('ApiService calls plugin catalog, config, stage, and decodes plugin_event SSE', () async {
      Map<String, dynamic>? savedConfigPayload;
      Map<String, dynamic>? stagePayload;

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

      final stageRes = await api.stagePluginCatalogItems('project-gutenberg', const [
        CatalogStageItemRequest(externalId: '1342', preferredFormat: 'epub'),
      ]);
      expect((stagePayload?['items'] as List).first['external_id'], '1342');
      expect(stageRes.results.single.uploadJobId, 'job-1');

      final events = await api.streamEvents().toList();
      expect(events.single, isA<PluginEvent>());
      final pEvt = events.single as PluginEvent;
      expect(pEvt.data['plugin'], 'project-gutenberg');
    });
  });
}
