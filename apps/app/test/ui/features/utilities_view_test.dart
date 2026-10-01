import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/duplicate_group.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/features/utilities/utilities_view.dart';
import 'package:shelf/ui/state/providers.dart';

class MockUtilitiesBookRepo implements BookRepository {
  DuplicateScanResult scanResult;
  bool wasMergeCalled = false;
  String? lastMergedPrimaryId;
  List<String>? lastMergedDuplicateIds;

  MockUtilitiesBookRepo({
    required this.scanResult,
  });

  @override
  ApiService get apiService => throw UnimplementedError();

  @override
  StorageService get storageService => throw UnimplementedError();

  @override
  Future<DuplicateScanResult> getDuplicateBooks() async => scanResult;

  @override
  Future<Book> mergeBooks({
    required String primaryBookId,
    required List<String> duplicateBookIds,
    MergeOptions? options,
  }) async {
    wasMergeCalled = true;
    lastMergedPrimaryId = primaryBookId;
    lastMergedDuplicateIds = duplicateBookIds;
    return Book(
      id: primaryBookId,
      title: 'Dune (Merged)',
      authors: [const Author(id: 'a1', name: 'Frank Herbert')],
    );
  }

  @override
  Future<List<Author>> getAuthors() async => [];

  @override
  Future<List<Series>> getSeries() async => [];

  @override
  Future<List<Genre>> getGenres() async => [];

  @override
  Future<List<Topic>> getTopics() async => [];

  @override
  Future<PaginatedBooks> getBooksPage({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? authorName,
    String? genreId,
    String? genreName,
    String? topicId,
    String? topicName,
    String? seriesId,
    String? search,
  }) async {
    return PaginatedBooks(
      books: [],
      total: 0,
      page: 1,
      perPage: perPage,
      limit: perPage,
      offset: 0,
    );
  }

  @override
  Future<List<Book>> getBooks({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? authorName,
    String? genreId,
    String? genreName,
    String? topicId,
    String? topicName,
    String? seriesId,
    String? search,
  }) async =>
      [];

  @override
  Future<Book> getBookDetail(String id) async => Book(id: id, title: 'Book');

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  testWidgets('UtilitiesView renders duplicate books and performs merge',
      (tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final duplicateGroup = DuplicateGroup(
      id: 'dup_group_1',
      matchReason: 'Matching Identifier (ISBN)',
      confidence: 1.0,
      books: [
        const Book(
          id: 'b1',
          title: 'Dune',
          authors: [Author(id: 'a1', name: 'Frank Herbert')],
          fileSizeBytes: 2048576,
        ),
        const Book(
          id: 'b2',
          title: 'Dune (Special Edition)',
          authors: [Author(id: 'a1', name: 'Frank Herbert')],
          fileSizeBytes: 1048576,
        ),
      ],
    );

    final mockRepo = MockUtilitiesBookRepo(
      scanResult: DuplicateScanResult(
        groups: [duplicateGroup],
        totalGroups: 1,
        totalDuplicateBooks: 2,
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(mockRepo),
        ],
        child: const MaterialApp(
          home: UtilitiesView(),
        ),
      ),
    );

    await tester.pumpAndSettle();

    // Verify header and summary card
    expect(find.text('Utilities & Maintenance'), findsOneWidget);
    expect(find.text('Duplicate Books Finder'), findsOneWidget);
    expect(find.text('1 Groups Found'), findsOneWidget);
    expect(find.text('Matching Identifier (ISBN)'), findsOneWidget);
    expect(find.text('100% Match'), findsOneWidget);

    // Verify books in group
    expect(find.text('Dune'), findsOneWidget);
    expect(find.text('Dune (Special Edition)'), findsOneWidget);
    expect(find.text('PRIMARY'), findsOneWidget); // First book selected by default

    // Tap on the second book to make it primary
    await tester.tap(find.text('Dune (Special Edition)'));
    await tester.pumpAndSettle();

    // Click Merge Group button
    final mergeButton = find.text('Merge Group');
    expect(mergeButton, findsOneWidget);
    await tester.tap(mergeButton);
    await tester.pumpAndSettle();

    // Verify mock repo was invoked with b2 as primary and b1 as duplicate
    expect(mockRepo.wasMergeCalled, true);
    expect(mockRepo.lastMergedPrimaryId, 'b2');
    expect(mockRepo.lastMergedDuplicateIds, ['b1']);

    // Verify success banner appears
    expect(find.textContaining('Successfully merged'), findsOneWidget);
    // After merging group, clean state is shown
    expect(find.text('No Duplicate Books Found'), findsOneWidget);
  });

  testWidgets('UtilitiesView renders clean state when 0 duplicates found',
      (tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    final mockRepo = MockUtilitiesBookRepo(
      scanResult: const DuplicateScanResult(
        groups: [],
        totalGroups: 0,
        totalDuplicateBooks: 0,
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(mockRepo),
        ],
        child: const MaterialApp(
          home: UtilitiesView(),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.text('No Duplicate Books Found'), findsOneWidget);
    expect(find.text('Library Clean'), findsOneWidget);
  });
}
