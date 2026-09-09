import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/repositories/auth_repository.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/data/repositories/reader_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late StorageService storageService;

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    storageService = StorageService(prefs);
  });

  group('Repositories Tests', () {
    test('AuthRepository login and session persistence', () async {
      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/auth/login') {
          return http.Response(jsonEncode({'token': 'token-abc'}), 200,
              headers: {'content-type': 'application/json'});
        } else if (request.url.path == '/api/v1/auth/me') {
          return http.Response(
              jsonEncode({'id': 'u1', 'username': 'admin', 'role': 'admin'}), 200,
              headers: {'content-type': 'application/json'});
        }
        return http.Response('Not found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final authRepo = AuthRepository(apiService: apiService, storageService: storageService);

      expect(await authRepo.checkInitialAuth(), isFalse);

      final user = await authRepo.login('http://localhost:8080', 'admin', 'pass');
      expect(user.username, 'admin');
      expect(storageService.getAuthToken(), 'token-abc');
      expect(storageService.getServerUrl(), 'http://localhost:8080');
      expect(storageService.getSavedUsername(), 'admin');

      expect(await authRepo.checkInitialAuth(), isTrue);

      await authRepo.logout();
      expect(await authRepo.checkInitialAuth(), isFalse);
    });

    test('AuthRepository changePassword delegates to ApiService', () async {
      bool called = false;
      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/auth/change-password') {
          called = true;
          return http.Response(jsonEncode({'message': 'ok'}), 200);
        }
        return http.Response('Not found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final authRepo = AuthRepository(apiService: apiService, storageService: storageService);
      await authRepo.changePassword('old-p', 'new-p');
      expect(called, isTrue);
    });

    test('BookRepository getBooks hydrates reading progress', () async {
      final mockClient = MockClient((request) async {
        return http.Response(
          jsonEncode({
            'data': [
              {
                'id': 'book-1',
                'title': 'Dune',
                'authors': [{'id': 'a1', 'name': 'Frank Herbert'}]
              }
            ],
            'total': 1,
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      await storageService.saveReadingProgress('book-1', 0.62);

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      final books = await bookRepo.getBooks();
      expect(books.length, 1);
      expect(books.first.readingProgress, 0.62);
    });

    test('BookRepository getBooksPage returns PaginatedBooks with hydrated progress', () async {
      final mockClient = MockClient((request) async {
        return http.Response(
          jsonEncode({
            'books': [
              {
                'id': 'book-1',
                'title': 'Dune',
                'authors': [{'id': 'a1', 'name': 'Frank Herbert'}],
              }
            ],
            'total': 42,
            'limit': 24,
            'offset': 0,
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      await storageService.saveReadingProgress('book-1', 0.85);

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      final pageData = await bookRepo.getBooksPage(page: 1, perPage: 24);
      expect(pageData.books.length, 1);
      expect(pageData.books.first.readingProgress, 0.85);
      expect(pageData.total, 42);
      expect(pageData.hasMore, true);
    });

    test('ReaderRepository caches chapter content and recovers offline', () async {
      int fetchCount = 0;
      final mockClient = MockClient((request) async {
        fetchCount++;
        return http.Response(
          jsonEncode({
            'id': 'c-1',
            'book_id': 'b-1',
            'chapter_index': 1,
            'title': 'Chapter 1',
            'summary': 'Summary',
            'content': 'Online fetched chapter content.',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final readerRepo = ReaderRepository(apiService: apiService, storageService: storageService);

      final ch1 = await readerRepo.loadChapter('b-1', 1);
      expect(ch1.content, 'Online fetched chapter content.');
      expect(fetchCount, 1);

      // Second load should use cache, not hit the network client
      final ch2 = await readerRepo.loadChapter('b-1', 1);
      expect(ch2.content, 'Online fetched chapter content.');
      expect(ch2.title, 'Chapter 1');
      expect(ch2.chapterIndex, 1);
      expect(fetchCount, 1);
    });

    test('ReaderRepository preserves real title and index when loaded with ULID identifier', () async {
      int fetchCount = 0;
      final mockClient = MockClient((request) async {
        fetchCount++;
        return http.Response(
          jsonEncode({
            'id': '01M22YCE6D6GZJAXZC8A39AVKE',
            'book_id': 'b-1',
            'chapter_index': 5,
            'title': 'Chapter 5: Demographic Threat',
            'summary': 'Summary',
            'content': 'Chapter 5 content.',
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final readerRepo = ReaderRepository(apiService: apiService, storageService: storageService);

      final ch1 = await readerRepo.loadChapter('b-1', '01M22YCE6D6GZJAXZC8A39AVKE');
      expect(ch1.title, 'Chapter 5: Demographic Threat');
      expect(ch1.chapterIndex, 5);
      expect(ch1.id, '01M22YCE6D6GZJAXZC8A39AVKE');
      expect(fetchCount, 1);

      // Second load from cache must preserve original title and index, not 'Chapter 01M22YCE...'
      final ch2 = await readerRepo.loadChapter('b-1', '01M22YCE6D6GZJAXZC8A39AVKE');
      expect(ch2.title, 'Chapter 5: Demographic Threat');
      expect(ch2.chapterIndex, 5);
      expect(ch2.id, '01M22YCE6D6GZJAXZC8A39AVKE');
      expect(fetchCount, 1);
    });

    test('ReaderRepository legacy raw string cache fallback does not synthesize title from ULID', () async {
      await storageService.cacheChapter('b-1', '01M22YCE6D6GZJAXZC8A39AVKE', 'Legacy cached content');

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: MockClient((_) async => http.Response('', 404)));
      final readerRepo = ReaderRepository(apiService: apiService, storageService: storageService);

      final ch = await readerRepo.loadChapter('b-1', '01M22YCE6D6GZJAXZC8A39AVKE');
      expect(ch.content, 'Legacy cached content');
      expect(ch.title, isEmpty);
    });

    test('BookRepository stageUpload, commitUpload, deleteUploadJob delegate to ApiService', () async {
      final mockClient = MockClient((request) async {
        if (request.method == 'POST' && request.url.path == '/api/v1/books/upload/stage') {
          return http.Response(
            jsonEncode({
              'job_id': 'job-repo-1',
              'status': 'staged',
              'filename': 'earthsea.epub',
              'has_cover': false,
              'warnings': [],
              'metadata': {
                'title': 'A Wizard of Earthsea',
                'authors': ['Ursula K. Le Guin'],
              },
            }),
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'POST' && request.url.path == '/api/v1/books/upload/jobs/job-repo-1/commit') {
          return http.Response(
            jsonEncode({
              'id': 'book-earthsea-1',
              'title': 'A Wizard of Earthsea',
              'authors': [{'id': 'a1', 'name': 'Ursula K. Le Guin'}],
            }),
            201,
            headers: {'content-type': 'application/json'},
          );
        }
        if (request.method == 'DELETE' && request.url.path == '/api/v1/books/upload/jobs/job-repo-1') {
          return http.Response('', 204);
        }
        return http.Response('Not found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      final job = await bookRepo.stageUpload(filename: 'earthsea.epub', bytes: [1, 2, 3]);
      expect(job.jobId, 'job-repo-1');
      expect(job.metadata.title, 'A Wizard of Earthsea');

      final coverUrl = bookRepo.getUploadJobCoverUrl('job-repo-1');
      expect(coverUrl, 'http://localhost:8080/api/v1/books/upload/jobs/job-repo-1/cover');

      final committed = await bookRepo.commitUpload(
        'job-repo-1',
        const StagedMetadata(
          title: 'A Wizard of Earthsea',
          authors: ['Ursula K. Le Guin'],
        ),
      );
      expect(committed.id, 'book-earthsea-1');

      await expectLater(bookRepo.deleteUploadJob('job-repo-1'), completes);
    });
  });
}
