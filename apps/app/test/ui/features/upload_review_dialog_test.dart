import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/upload/upload_drop_target.dart';
import 'package:shelf/ui/features/upload/upload_review_dialog.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late StorageService storageService;

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    storageService = StorageService(prefs);
  });

  group('UploadReviewContent Tests', () {
    testWidgets('renders metadata fields, cover placeholder, and missing author warning', (tester) async {
      final stagedJob = StagedUploadJob(
        jobId: 'job-test-1',
        status: 'staged',
        filename: 'unknown_novel.epub',
        hasCover: false,
        warnings: ['No author found in EPUB metadata'],
        metadata: const StagedMetadata(
          title: 'Mystery of the Tower',
          authors: ['Unknown'],
          series: null,
          sequenceNumber: null,
          genres: ['Mystery'],
          description: 'A thrilling mystery novel.',
        ),
      );

      final mockClient = MockClient((request) async => http.Response('{}', 200));
      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: UploadReviewContent(job: stagedJob, isBottomSheet: false),
            ),
          ),
        ),
      );

      // Verify header and filename
      expect(find.text('Review Staged EPUB'), findsOneWidget);
      expect(find.text('unknown_novel.epub'), findsOneWidget);
      expect(find.text('Staged'), findsOneWidget);

      // Verify missing author warning banner
      expect(find.text('Missing Author'), findsOneWidget);
      expect(
        find.text('EPUB metadata did not specify an author. Please provide one to ensure proper folder structure.'),
        findsOneWidget,
      );

      // Verify destination preview
      expect(find.text('DESTINATION PATH'), findsOneWidget);
      expect(
        find.text('/library/Unknown/Mystery of the Tower/Mystery of the Tower.epub'),
        findsOneWidget,
      );

      // Verify form fields
      expect(find.text('Mystery of the Tower'), findsNWidgets(2)); // in cover placeholder and Title TextFormField
      expect(find.text('Mystery'), findsOneWidget);
      expect(find.text('A thrilling mystery novel.'), findsOneWidget);
    });

    testWidgets('updating Title and Author live-updates the destination path preview', (tester) async {
      final stagedJob = StagedUploadJob(
        jobId: 'job-test-2',
        status: 'staged',
        filename: 'custom_book.epub',
        hasCover: false,
        metadata: const StagedMetadata(
          title: 'Initial Title',
          authors: ['Unknown'],
        ),
      );

      final mockClient = MockClient((request) async => http.Response('{}', 200));
      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: UploadReviewContent(job: stagedJob, isBottomSheet: false),
            ),
          ),
        ),
      );

      // Find author field and enter 'Brandon Sanderson'
      final authorField = find.widgetWithText(TextFormField, 'Author(s) *');
      await tester.enterText(authorField, 'Brandon Sanderson');
      await tester.pump();

      // Missing author warning should now disappear
      expect(find.text('Missing Author'), findsNothing);

      // Destination preview should now reflect Brandon Sanderson
      expect(
        find.text('/library/Brandon Sanderson/Initial Title/Initial Title.epub'),
        findsOneWidget,
      );

      // Find title field and enter 'The Way of Kings'
      final titleField = find.widgetWithText(TextFormField, 'Title *');
      await tester.enterText(titleField, 'The Way of Kings');
      await tester.pump();

      // Destination preview should now reflect both updated fields
      expect(
        find.text('/library/Brandon Sanderson/The Way of Kings/The Way of Kings.epub'),
        findsOneWidget,
      );
    });

    testWidgets('Discard button calls deleteUploadJob and closes dialog', (tester) async {
      bool deleteCalled = false;
      final mockClient = MockClient((request) async {
        if (request.method == 'DELETE' && request.url.path == '/api/v1/books/upload/jobs/job-discard-1') {
          deleteCalled = true;
          return http.Response('', 204);
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      final stagedJob = StagedUploadJob(
        jobId: 'job-discard-1',
        status: 'staged',
        filename: 'discard_me.epub',
        metadata: const StagedMetadata(title: 'Discard Me', authors: ['Unknown']),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: Builder(
                builder: (context) => ElevatedButton(
                  onPressed: () => showUploadReviewModal(context, stagedJob),
                  child: const Text('Open Modal'),
                ),
              ),
            ),
          ),
        ),
      );

      // Open the modal
      await tester.tap(find.text('Open Modal'));
      await tester.pumpAndSettle();

      expect(find.text('Review Staged EPUB'), findsOneWidget);

      // Tap Discard
      await tester.tap(find.text('Discard'));
      await tester.pumpAndSettle();

      expect(deleteCalled, isTrue);
      // Modal should be closed
      expect(find.text('Review Staged EPUB'), findsNothing);
    });

    testWidgets('Save & Add to Library commits updated metadata and closes dialog', (tester) async {
      bool commitCalled = false;
      String? committedTitle;
      List<dynamic>? committedAuthors;

      final mockClient = MockClient((request) async {
        if (request.method == 'POST' && request.url.path == '/api/v1/books/upload/jobs/job-commit-1/commit') {
          commitCalled = true;
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          committedTitle = body['title'] as String?;
          committedAuthors = body['authors'] as List<dynamic>?;

          return http.Response(
            jsonEncode({
              'id': 'book-new-1',
              'title': committedTitle,
              'authors': [{'id': 'a1', 'name': committedAuthors?.first ?? 'Author'}],
            }),
            201,
            headers: {'content-type': 'application/json'},
          );
        }
        return http.Response('Not Found', 404);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      final stagedJob = StagedUploadJob(
        jobId: 'job-commit-1',
        status: 'staged',
        filename: 'le_guin.epub',
        metadata: const StagedMetadata(
          title: 'A Wizard of Earthsea',
          authors: ['Unknown'],
        ),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: Builder(
                builder: (context) => ElevatedButton(
                  onPressed: () => showUploadReviewModal(context, stagedJob),
                  child: const Text('Open Modal'),
                ),
              ),
            ),
          ),
        ),
      );

      // Open the modal
      await tester.tap(find.text('Open Modal'));
      await tester.pumpAndSettle();

      // Enter correct author
      final authorField = find.widgetWithText(TextFormField, 'Author(s) *');
      await tester.enterText(authorField, 'Ursula K. Le Guin');
      await tester.pump();

      // Tap Save & Add to Library
      await tester.tap(find.text('Save & Add to Library'));
      await tester.pumpAndSettle();

      expect(commitCalled, isTrue);
      expect(committedTitle, 'A Wizard of Earthsea');
      expect(committedAuthors, ['Ursula K. Le Guin']);
      // Modal should be closed
      expect(find.text('Review Staged EPUB'), findsNothing);
    });
  });

  group('ShelfdDropTarget Tests', () {
    testWidgets('renders child widget normally', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: ShelfdDropTarget(
            child: const Text('Inner Content'),
          ),
        ),
      );

      expect(find.text('Inner Content'), findsOneWidget);
      expect(find.text('Drop EPUB anywhere to upload'), findsNothing);
    });
  });
}
