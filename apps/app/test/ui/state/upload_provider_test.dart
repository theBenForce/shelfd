import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('UploadNotifier loads staged jobs and sets initial selected job', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/books/upload/jobs') {
        return http.Response(
          jsonEncode({
            'jobs': [
              {
                'job_id': 'job-1',
                'status': 'staged',
                'filename': 'dune.epub',
                'metadata': {
                  'title': 'Dune',
                  'authors': ['Frank Herbert'],
                },
              },
              {
                'job_id': 'job-2',
                'status': 'staged',
                'filename': 'unknown.epub',
                'metadata': {
                  'title': 'Untitled',
                  'authors': [],
                },
              }
            ]
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/queue/status') {
        return http.Response(
          jsonEncode({'pending': 0, 'processing': 0, 'staged_uploads': 2}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(apiService),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(uploadProvider.notifier);
    await notifier.loadStagedJobs();

    final state = container.read(uploadProvider);
    expect(state.stagedJobs.length, 2);
    expect(state.selectedJobId, 'job-1');
    expect(state.readyCount, 1);
    expect(state.warningsCount, 1);
  });

  test('UploadNotifier uploads and stages multiple files', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    final stagedRequests = <String>[];
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/books/upload/stage') {
        stagedRequests.add(request.url.path);
        return http.Response(
          jsonEncode({
            'job_id': 'new-job-${stagedRequests.length}',
            'status': 'staged',
            'filename': 'test.epub',
            'metadata': {'title': 'Test Title', 'authors': ['Author']},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/books/upload/jobs') {
        return http.Response(jsonEncode({'jobs': []}), 200, headers: {'content-type': 'application/json'});
      }
      if (request.url.path == '/api/v1/queue/status') {
        return http.Response(
          jsonEncode({'pending': 0, 'processing': 0, 'staged_uploads': 1}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(apiService),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(uploadProvider.notifier);

    final files = [
      PickedEpubFile(name: 'book1.epub', bytes: [1, 2, 3]),
      PickedEpubFile(name: 'book2.epub', bytes: [4, 5, 6]),
    ];

    final count = await notifier.uploadEpubFiles(files);
    expect(count, 2);
    expect(stagedRequests.length, 2);
  });

  test('UploadNotifier auto-commits when autoCommit is enabled', () async {
    SharedPreferences.setMockInitialValues({'shelfd_auto_commit_uploads': true});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    bool commitCalled = false;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/books/upload/stage') {
        return http.Response(
          jsonEncode({
            'job_id': 'auto-job-1',
            'status': 'staged',
            'filename': 'test.epub',
            'metadata': {'title': 'Auto Book', 'authors': ['Auto Author']},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/books/upload/jobs/auto-job-1/commit') {
        commitCalled = true;
        return http.Response(
          jsonEncode({
            'id': 'book-auto-1',
            'title': 'Auto Book',
            'authors': [{'id': 'a1', 'name': 'Auto Author'}],
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/books/upload/jobs') {
        return http.Response(jsonEncode({'jobs': []}), 200, headers: {'content-type': 'application/json'});
      }
      if (request.url.path == '/api/v1/queue/status') {
        return http.Response(
          jsonEncode({'pending': 0, 'processing': 0, 'staged_uploads': 0}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(apiService),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(uploadProvider.notifier);
    expect(container.read(uploadProvider).autoCommit, isTrue);

    final files = [
      PickedEpubFile(name: 'autobook.epub', bytes: [1, 2, 3]),
    ];

    final count = await notifier.uploadEpubFiles(files);
    expect(count, 1);
    expect(commitCalled, isTrue);

    // Book should be added to libraryProvider
    final libraryState = container.read(libraryProvider);
    expect(libraryState.books.any((b) => b.title == 'Auto Book'), isTrue);
  });

  test('UploadNotifier commitJob, commitAllReady, discardJob, updateStagedMetadata', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    String? committedJobId;
    String? deletedJobId;

    final mockClient = MockClient((request) async {
      if (request.url.path.endsWith('/commit')) {
        committedJobId = request.url.pathSegments[request.url.pathSegments.length - 2];
        return http.Response(
          jsonEncode({
            'id': 'committed-book-id',
            'title': 'Committed Title',
            'authors': [{'id': 'a1', 'name': 'Committed Author'}],
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.method == 'DELETE' && request.url.path.contains('/upload/jobs/')) {
        deletedJobId = request.url.pathSegments.last;
        return http.Response(jsonEncode({'deleted': true}), 200, headers: {'content-type': 'application/json'});
      }
      if (request.url.path == '/api/v1/books/upload/jobs') {
        return http.Response(
          jsonEncode({
            'jobs': [
              {
                'job_id': 'job-ready-1',
                'status': 'staged',
                'filename': 'ready.epub',
                'metadata': {'title': 'Ready Book', 'authors': ['Author One']},
              },
              {
                'job_id': 'job-warning-2',
                'status': 'staged',
                'filename': 'warn.epub',
                'metadata': {'title': 'Untitled', 'authors': []},
              }
            ]
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/queue/status') {
        return http.Response(
          jsonEncode({'pending': 0, 'processing': 0, 'staged_uploads': 1}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(apiService),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(uploadProvider.notifier);
    await notifier.loadStagedJobs();

    // Verify updateStagedMetadata fixes warning
    notifier.updateStagedMetadata(
      'job-warning-2',
      const StagedMetadata(title: 'Fixed Title', authors: ['Fixed Author']),
    );
    var state = container.read(uploadProvider);
    final fixedJob = state.stagedJobs.firstWhere((j) => j.jobId == 'job-warning-2');
    expect(fixedJob.metadata.title, 'Fixed Title');
    expect(fixedJob.warnings.isEmpty, isTrue);

    // Commit single job
    await notifier.commitJob('job-ready-1', const StagedMetadata(title: 'Ready Book', authors: ['Author One']));
    expect(committedJobId, 'job-ready-1');
    state = container.read(uploadProvider);
    expect(state.stagedJobs.any((j) => j.jobId == 'job-ready-1'), isFalse);

    // Discard job
    await notifier.discardJob('job-warning-2');
    expect(deletedJobId, 'job-warning-2');
    state = container.read(uploadProvider);
    expect(state.stagedJobs.any((j) => j.jobId == 'job-warning-2'), isFalse);
  });

  test('UploadNotifier replaces job cover and increments version counter', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    String? uploadedCoverJobId;
    final mockClient = MockClient((request) async {
      if (request.url.path == '/api/v1/books/upload/jobs') {
        return http.Response(
          jsonEncode({
            'jobs': [
              {
                'job_id': 'job-cover-1',
                'status': 'staged',
                'filename': 'book.epub',
                'has_cover': false,
                'metadata': {'title': 'Book Without Cover', 'authors': ['Author']},
              },
            ]
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path == '/api/v1/queue/status') {
        return http.Response(
          jsonEncode({'pending': 0, 'processing': 0, 'staged_uploads': 1}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.method == 'POST' && request.url.path == '/api/v1/books/upload/jobs/job-cover-1/cover') {
        uploadedCoverJobId = 'job-cover-1';
        return http.Response(
          jsonEncode({'message': 'Cover image updated successfully', 'job_id': 'job-cover-1', 'has_cover': true}),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        apiServiceProvider.overrideWithValue(apiService),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(uploadProvider.notifier);
    await notifier.loadStagedJobs();

    var state = container.read(uploadProvider);
    expect(state.stagedJobs.first.hasCover, isFalse);
    expect(state.coverVersions['job-cover-1'], isNull);

    await notifier.replaceJobCover(
      jobId: 'job-cover-1',
      filename: 'new_cover.jpg',
      bytes: [1, 2, 3, 4],
    );

    expect(uploadedCoverJobId, 'job-cover-1');
    state = container.read(uploadProvider);
    expect(state.stagedJobs.first.hasCover, isTrue);
    expect(state.coverVersions['job-cover-1'], 1);
  });
}
