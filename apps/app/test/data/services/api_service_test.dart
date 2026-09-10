import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/data/services/api_service.dart';

void main() {
  group('ApiService Tests', () {
    test('getConnectInfo returns ServerConnectInfo', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/server/connect-info');
        return http.Response(
          jsonEncode({
            'server_name': 'shelfd',
            'version': '0.1.0',
            'base_url': 'http://localhost:8080',
            'capabilities': ['mcp', 'semantic_search'],
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final info = await apiService.getConnectInfo();

      expect(info.serverName, 'shelfd');
      expect(info.version, '0.1.0');
    });

    test('login returns token and sets bearer header', () async {
      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/auth/login') {
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(body['username'], 'admin');
          expect(body['password'], 'secret');
          return http.Response(
            jsonEncode({'token': 'jwt-token-xyz', 'user': {'id': 'u1', 'username': 'admin', 'role': 'admin'}}),
            200,
            headers: {'content-type': 'application/json'},
          );
        } else if (request.url.path == '/api/v1/auth/me') {
          expect(request.headers['authorization'], 'Bearer jwt-token-xyz');
          return http.Response(
            jsonEncode({'id': 'u1', 'username': 'admin', 'role': 'admin'}),
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final token = await apiService.login('admin', 'secret');
      expect(token, 'jwt-token-xyz');

      final user = await apiService.getMe();
      expect(user.username, 'admin');
    });

    test('changePassword sends correct payload and handles errors', () async {
      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/auth/change-password') {
          expect(request.method, 'POST');
          expect(request.headers['authorization'], 'Bearer valid-token');
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          if (body['current_password'] == 'wrong-pass') {
            return http.Response(jsonEncode({'error': 'Current password is incorrect'}), 401);
          }
          expect(body['current_password'], 'old-secret');
          expect(body['new_password'], 'new-secret-123');
          return http.Response(jsonEncode({'message': 'Password updated successfully'}), 200);
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', token: 'valid-token', client: mockClient);
      await expectLater(apiService.changePassword('old-secret', 'new-secret-123'), completes);

      expect(
        () => apiService.changePassword('wrong-pass', 'new-secret-123'),
        throwsA(isA<ApiException>().having((e) => e.message, 'message', 'Current password is incorrect')),
      );
    });

    test('getBooks parses paginated books list', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/books');
        expect(request.url.queryParameters['page'], '1');
        return http.Response(
          jsonEncode({
            'data': [
              {
                'id': 'book-1',
                'title': 'Dune',
                'synopsis': 'Arrakis',
                'authors': [{'id': 'a1', 'name': 'Frank Herbert'}],
              }
            ],
            'page': 1,
            'per_page': 20,
            'total': 1,
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final books = await apiService.getBooks(page: 1);
      expect(books.length, 1);
      expect(books.first.title, 'Dune');
      expect(books.first.authorDisplay, 'Frank Herbert');
    });

    test('getBooksPage parses full envelope with total, offset, and limit', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/books');
        expect(request.url.queryParameters['page'], '2');
        expect(request.url.queryParameters['per_page'], '24');
        expect(request.url.queryParameters['limit'], '24');
        expect(request.url.queryParameters['offset'], '24');
        return http.Response(
          jsonEncode({
            'books': [
              {
                'id': 'book-2',
                'title': 'Messiah',
                'authors': [{'id': 'a1', 'name': 'Frank Herbert'}],
              }
            ],
            'total': 50,
            'limit': 24,
            'offset': 24,
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final pageData = await apiService.getBooksPage(page: 2, perPage: 24);
      expect(pageData.books.length, 1);
      expect(pageData.books.first.title, 'Messiah');
      expect(pageData.total, 50);
      expect(pageData.limit, 24);
      expect(pageData.offset, 24);
      expect(pageData.hasMore, true);
    });

    test('getChapter returns chapter content', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/books/book-1/chapters/2');
        return http.Response(
          jsonEncode({
            'id': 'chap-2',
            'book_id': 'book-1',
            'chapter_index': 2,
            'title': 'Chapter 2',
            'summary': 'Summary of chapter 2',
            'content': 'Paragraph one.\n\nParagraph two.',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final chapter = await apiService.getChapter('book-1', 2);
      expect(chapter.chapterIndex, 2);
      expect(chapter.content, contains('Paragraph one.'));
    });

    test('getChapter with string chapterId returns chapter content', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/books/book-1/chapters/01J7SPINE1');
        return http.Response(
          jsonEncode({
            'id': '01J7SPINE1',
            'book_id': 'book-1',
            'chapter_index': 1,
            'title': 'Chapter 1: The Beginning',
            'summary': 'Summary of chapter 1',
            'content': 'Paragraph one.\n\nParagraph two.',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final chapter = await apiService.getChapter('book-1', '01J7SPINE1');
      expect(chapter.id, '01J7SPINE1');
      expect(chapter.title, 'Chapter 1: The Beginning');
      expect(chapter.content, contains('Paragraph one.'));
    });

    test('handles 401 Unauthorized by throwing ApiException', () async {
      final mockClient = MockClient((request) async {
        return http.Response(jsonEncode({'error': 'Unauthorized'}), 401,
            headers: {'content-type': 'application/json'});
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      expect(() => apiService.getMe(), throwsA(isA<ApiException>()));
    });

    test('getQueueStatus returns QueueStatus', () async {
      final mockClient = MockClient((request) async {
        expect(request.url.path, '/api/v1/queue/status');
        return http.Response(
          jsonEncode({
            'total_chapters': 25,
            'indexed_chapters': 10,
            'pending_chapters': 15,
            'pending_uploads': 0,
            'progress_percent': 40.0,
            'is_active': true,
            'current_book': 'Solaris',
            'current_chapter': 'The Station',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final status = await apiService.getQueueStatus();
      expect(status.totalChapters, 25);
      expect(status.indexedChapters, 10);
      expect(status.pendingChapters, 15);
      expect(status.progressPercent, 40.0);
      expect(status.isActive, isTrue);
      expect(status.currentBook, 'Solaris');
      expect(status.currentChapter, 'The Station');
    });

    test('streamQueueEvents decodes SSE stream into QueueStatus events', () async {
      final ssePayload = 'event: queue_status\n'
          'data: {"total_chapters": 10, "indexed_chapters": 5, "pending_chapters": 5, "pending_uploads": 0, "progress_percent": 50.0, "is_active": true, "current_book": "Neuromancer", "current_chapter": "Chiba"}\n\n'
          'event: queue_status\n'
          'data: {"total_chapters": 10, "indexed_chapters": 6, "pending_chapters": 4, "pending_uploads": 0, "progress_percent": 60.0, "is_active": true, "current_book": "Neuromancer", "current_chapter": "The Sprawl"}\n\n';

      final mockClient = MockClient.streaming((request, bodyStream) async {
        expect(request.url.path, '/api/v1/queue/events');
        expect(request.headers['Accept'], 'text/event-stream');
        final stream = Stream.value(utf8.encode(ssePayload));
        return http.StreamedResponse(stream, 200, headers: {'content-type': 'text/event-stream'});
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final events = await apiService.streamQueueEvents().take(2).toList();

      expect(events.length, 2);
      expect(events[0].totalChapters, 10);
      expect(events[0].indexedChapters, 5);
      expect(events[0].progressPercent, 50.0);
      expect(events[0].currentBook, 'Neuromancer');
      expect(events[0].currentChapter, 'Chiba');

      expect(events[1].indexedChapters, 6);
      expect(events[1].progressPercent, 60.0);
      expect(events[1].currentChapter, 'The Sprawl');
    });

    test('streamEvents decodes mixed SSE stream into typed ShelfdEvent instances', () async {
      final ssePayload = 'event: queue_status\n'
          'data: {"total_chapters": 10, "indexed_chapters": 5, "pending_chapters": 5, "pending_uploads": 0, "progress_percent": 50.0, "is_active": true}\n\n'
          'event: book_added\n'
          'data: {"id": "book-99", "title": "Dune", "authors": [{"id": "auth-1", "name": "Frank Herbert"}]}\n\n'
          'event: scan_status\n'
          'data: {"status": "completed", "new": 1}\n\n';

      final mockClient = MockClient.streaming((request, bodyStream) async {
        expect(request.url.path, '/api/v1/queue/events');
        final stream = Stream.value(utf8.encode(ssePayload));
        return http.StreamedResponse(stream, 200, headers: {'content-type': 'text/event-stream'});
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final events = await apiService.streamEvents().take(3).toList();

      expect(events.length, 3);

      expect(events[0], isA<QueueStatusEvent>());
      final qEvt = events[0] as QueueStatusEvent;
      expect(qEvt.status.totalChapters, 10);
      expect(qEvt.status.indexedChapters, 5);

      expect(events[1], isA<BookAddedEvent>());
      final bEvt = events[1] as BookAddedEvent;
      expect(bEvt.book.id, 'book-99');
      expect(bEvt.book.title, 'Dune');
      expect(bEvt.book.authors.first.name, 'Frank Herbert');

      expect(events[2], isA<ScanStatusEvent>());
      final sEvt = events[2] as ScanStatusEvent;
      expect(sEvt.data['status'], 'completed');
      expect(sEvt.data['new'], 1);
    });

    test('streamQueueEvents ignores non-queue_status events in stream', () async {
      final ssePayload = 'event: book_added\n'
          'data: {"id": "book-99", "title": "Dune"}\n\n'
          'event: queue_status\n'
          'data: {"total_chapters": 12, "indexed_chapters": 12, "pending_chapters": 0, "pending_uploads": 0, "progress_percent": 100.0, "is_active": false}\n\n';

      final mockClient = MockClient.streaming((request, bodyStream) async {
        final stream = Stream.value(utf8.encode(ssePayload));
        return http.StreamedResponse(stream, 200, headers: {'content-type': 'text/event-stream'});
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final events = await apiService.streamQueueEvents().take(1).toList();

      expect(events.length, 1);
      expect(events[0].totalChapters, 12);
      expect(events[0].progressPercent, 100.0);
    });

    test('streamQueueEvents throws ApiException on error status', () async {
      final mockClient = MockClient.streaming((request, bodyStream) async {
        final stream = Stream.value(utf8.encode('Unauthorized'));
        return http.StreamedResponse(stream, 401);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      expect(() => apiService.streamQueueEvents().toList(), throwsA(isA<ApiException>()));
    });

    test('createBookmark, getBookmarks, deleteBookmark', () async {
      final mockClient = MockClient((request) async {
        if (request.method == 'POST' && request.url.path == '/api/v1/books/book-1/bookmarks') {
          return http.Response(
            jsonEncode({
              'id': 'bm-new',
              'book_id': 'book-1',
              'title': 'Test Mark',
              'progress': 0.5,
            }),
            201,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'GET' && request.url.path == '/api/v1/books/book-1/bookmarks') {
          return http.Response(
            jsonEncode([
              {
                'id': 'bm-new',
                'book_id': 'book-1',
                'title': 'Test Mark',
                'progress': 0.5,
              }
            ]),
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'DELETE' && request.url.path == '/api/v1/bookmarks/bm-new') {
          return http.Response(jsonEncode({'status': 'deleted'}), 200,
              headers: {'content-type': 'application/json'});
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final created = await apiService.createBookmark('book-1', title: 'Test Mark', progress: 0.5);
      expect(created.id, 'bm-new');
      expect(created.title, 'Test Mark');

      final list = await apiService.getBookmarks('book-1');
      expect(list.length, 1);
      expect(list.first.id, 'bm-new');

      await expectLater(apiService.deleteBookmark('bm-new'), completes);
    });

    test('createHighlight, getHighlights, deleteHighlight', () async {
      final mockClient = MockClient((request) async {
        if (request.method == 'POST' && request.url.path == '/api/v1/books/book-1/highlights') {
          return http.Response(
            jsonEncode({
              'id': 'hl-new',
              'book_id': 'book-1',
              'selected_text': 'A famous line.',
              'color': 'yellow',
            }),
            201,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'GET' && request.url.path == '/api/v1/books/book-1/highlights') {
          return http.Response(
            jsonEncode([
              {
                'id': 'hl-new',
                'book_id': 'book-1',
                'selected_text': 'A famous line.',
                'color': 'yellow',
              }
            ]),
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'DELETE' && request.url.path == '/api/v1/highlights/hl-new') {
          return http.Response(jsonEncode({'status': 'deleted'}), 200,
              headers: {'content-type': 'application/json'});
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final created = await apiService.createHighlight('book-1', selectedText: 'A famous line.');
      expect(created.id, 'hl-new');
      expect(created.selectedText, 'A famous line.');

      final list = await apiService.getHighlights('book-1');
      expect(list.length, 1);
      expect(list.first.id, 'hl-new');

      await expectLater(apiService.deleteHighlight('hl-new'), completes);
    });

    test('chatWithBook calls API with message and returns citations', () async {
      final mockClient = MockClient((request) async {
        expect(request.method, 'POST');
        expect(request.url.path, '/api/v1/books/book-1/chat');
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['message'], 'What happens in Chapter 1?');

        return http.Response(
          jsonEncode({
            'reply': 'In Chapter 1, Ishmael embarks on his journey.',
            'citations': [
              {
                'chapter_id': 'chap-1',
                'chapter_index': 1,
                'chapter_title': 'Loomings',
                'summary': 'Ishmael travels to New Bedford.',
              }
            ],
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final res = await apiService.chatWithBook('book-1', 'What happens in Chapter 1?');
      expect(res.reply, contains('Ishmael embarks on his journey'));
      expect(res.citations.length, 1);
      expect(res.citations.first.chapterIndex, 1);
      expect(res.citations.first.chapterTitle, 'Loomings');
    });

    test('stageUploadBook sends multipart request and returns StagedUploadJob', () async {
      final mockClient = MockClient((request) async {
        expect(request.method, 'POST');
        expect(request.url.path, '/api/v1/books/upload/stage');
        expect(request.headers['authorization'], 'Bearer my-token');

        return http.Response(
          jsonEncode({
            'job_id': 'job-xyz-789',
            'status': 'staged',
            'filename': 'test_upload.epub',
            'has_cover': true,
            'warnings': ['No author found in EPUB metadata'],
            'metadata': {
              'title': 'Test Staged Title',
              'authors': ['Unknown'],
              'series': null,
              'sequence_number': null,
              'description': null,
              'genres': ['Fiction'],
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', token: 'my-token', client: mockClient);
      final job = await apiService.stageUploadBook(
        filename: 'test_upload.epub',
        bytes: [0x50, 0x4B, 0x03, 0x04],
      );

      expect(job.jobId, 'job-xyz-789');
      expect(job.status, 'staged');
      expect(job.filename, 'test_upload.epub');
      expect(job.hasCover, isTrue);
      expect(job.metadata.title, 'Test Staged Title');
      expect(job.metadata.primaryAuthor, 'Unknown');
      expect(job.warnings, contains('No author found in EPUB metadata'));
    });

    test('commitUploadJob sends updated metadata and returns created Book', () async {
      final mockClient = MockClient((request) async {
        expect(request.method, 'POST');
        expect(request.url.path, '/api/v1/books/upload/jobs/job-xyz-789/commit');
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['title'], 'Final Title');
        expect(body['authors'], ['Final Author']);

        return http.Response(
          jsonEncode({
            'id': 'book-committed-1',
            'title': 'Final Title',
            'authors': [{'id': 'a1', 'name': 'Final Author'}],
          }),
          201,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final book = await apiService.commitUploadJob(
        'job-xyz-789',
        const StagedMetadata(
          title: 'Final Title',
          authors: ['Final Author'],
        ),
      );

      expect(book.id, 'book-committed-1');
      expect(book.title, 'Final Title');
      expect(book.authors.first.name, 'Final Author');
    });

    test('deleteUploadJob sends DELETE request', () async {
      final mockClient = MockClient((request) async {
        expect(request.method, 'DELETE');
        expect(request.url.path, '/api/v1/books/upload/jobs/job-xyz-789');
        return http.Response('', 204);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      await expectLater(apiService.deleteUploadJob('job-xyz-789'), completes);
    });
  });
}
