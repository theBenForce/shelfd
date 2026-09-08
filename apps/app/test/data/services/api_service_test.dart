import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
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
  });
}
