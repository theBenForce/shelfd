import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/repositories/reader_repository.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/app_shell.dart';
import 'package:shelf/ui/features/authors/author_detail_view.dart';
import 'package:shelf/ui/features/book_detail/book_detail_view.dart';
import 'package:shelf/ui/features/library/library_view.dart';
import 'package:shelf/ui/features/reader/reader_view.dart';
import 'package:shelf/ui/features/search/search_view.dart';
import 'package:shelf/ui/features/series/series_detail_view.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/router.dart';
import 'package:shelf/ui/state/providers.dart';

class MockBookRepo implements BookRepository {
  final List<Book> books;
  final List<Author> authors;
  final List<Series> seriesList;

  MockBookRepo({
    this.books = const [],
    this.authors = const [],
    this.seriesList = const [],
  });

  @override
  Future<List<Author>> getAuthors() async => authors;

  @override
  Future<List<Series>> getSeries() async => seriesList;

  @override
  Future<PaginatedBooks> getBooksPage({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? genreId,
    String? seriesId,
    String? search,
  }) async {
    return PaginatedBooks(
      books: books,
      total: books.length,
      page: 1,
      perPage: perPage,
      limit: perPage,
      offset: 0,
    );
  }

  @override
  Future<Book> getBookDetail(String id) async {
    return const Book(
      id: 'book-1',
      title: 'Book One',
      authors: [Author(id: 'a-1', name: 'Test Author')],
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class MockReaderRepo implements ReaderRepository {
  @override
  Future<Chapter> loadChapter(String bookId, dynamic chapterIdentifier) async {
    return Chapter(
      id: 'ch-1',
      bookId: bookId,
      chapterIndex: 1,
      title: 'Chapter 1',
      content: 'Hello World',
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class FakeLibraryNotifier extends LibraryNotifier {
  @override
  LibraryState build() => const LibraryState(books: [], isLoading: false);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late SharedPreferences prefs;

  setUp(() async {
    SharedPreferences.setMockInitialValues({
      'shelfd_auth_token': 'valid-token',
      'shelfd_server_url': 'http://localhost:8080',
    });
    prefs = await SharedPreferences.getInstance();
  });

  Widget buildApp(String initialLocation) {
    final storage = StorageService(prefs);
    final router = createRouter(initialLocation: initialLocation, storageService: storage);

    return ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        libraryProvider.overrideWith(() => FakeLibraryNotifier()),
        bookRepositoryProvider.overrideWithValue(
          MockBookRepo(
            books: const [
              Book(
                id: 'book-1',
                title: 'Book One',
                authors: [Author(id: 'a-1', name: 'Test Author')],
                series: Series(id: 's-1', name: 'Test Series'),
                seriesSequence: 1,
                readingProgress: 0.0,
              ),
            ],
            authors: const [Author(id: 'a-1', name: 'Test Author')],
            seriesList: const [Series(id: 's-1', name: 'Test Series')],
          ),
        ),
        readerRepositoryProvider.overrideWithValue(MockReaderRepo()),
      ],
      child: MaterialApp.router(
        routerConfig: router,
      ),
    );
  }

  void setViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());
  }

  group('Router ShellRoute & Subroutes Tests', () {
    testWidgets('renders AppShell around /books route', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/books'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(LibraryView), findsOneWidget);
    });

    testWidgets('renders AppShell around /series route', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/series'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(LibraryView), findsOneWidget);
    });

    testWidgets('renders AppShell around /authors route', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/authors'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(LibraryView), findsOneWidget);
    });

    testWidgets('renders AppShell around /library route (redirects to /books)', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/library'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(LibraryView), findsOneWidget);
    });

    testWidgets('renders AppShell around /search route', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/search'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(SearchView), findsOneWidget);
    });

    testWidgets('renders AppShell around /settings route', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/settings'));
      await tester.pumpAndSettle();

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(SettingsView), findsOneWidget);
    });

    testWidgets('renders SeriesDetailView on /series/:seriesId subroute', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/series/s-1?name=Test+Series'));
      await tester.pumpAndSettle();

      expect(find.byType(SeriesDetailView), findsOneWidget);
      expect(find.byType(AppShell), findsNothing);
    });

    testWidgets('renders AuthorDetailView on /author/:authorId subroute', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/author/a-1?name=Test+Author'));
      await tester.pumpAndSettle();

      expect(find.byType(AuthorDetailView), findsOneWidget);
      expect(find.byType(AppShell), findsNothing);
    });

    testWidgets('renders AuthorDetailView on /authors/:authorId subroute', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/authors/a-1?name=Test+Author'));
      await tester.pumpAndSettle();

      expect(find.byType(AuthorDetailView), findsOneWidget);
      expect(find.byType(AppShell), findsNothing);
    });

    testWidgets('redirects /authors/:authorId to AuthorDetailView', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/authors/a-1'));
      await tester.pumpAndSettle();

      expect(find.byType(AuthorDetailView), findsOneWidget);
    });

    testWidgets('renders BookDetailView on /books/:bookId', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/books/book-1'));
      await tester.pumpAndSettle();

      expect(find.byType(BookDetailView), findsOneWidget);
      expect(find.byType(AppShell), findsNothing);
    });

    testWidgets('renders ReaderView on /books/:bookId/read subroute', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/books/book-1/read'));
      await tester.pumpAndSettle();

      expect(find.byType(ReaderView), findsOneWidget);
    });

    testWidgets('renders ReaderView on /books/:bookId/read/:chapterIdentifier subroute', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/books/book-1/read/ch-2'));
      await tester.pumpAndSettle();

      expect(find.byType(ReaderView), findsOneWidget);
    });

    testWidgets('redirects /book/:bookId to /books/:bookId', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/book/book-1'));
      await tester.pumpAndSettle();

      expect(find.byType(BookDetailView), findsOneWidget);
      expect(find.byType(AppShell), findsNothing);
    });

    testWidgets('redirects /reader/:bookId to /book/:bookId', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/reader/book-1'));
      await tester.pumpAndSettle();

      expect(find.byType(BookDetailView), findsOneWidget);
    });

    testWidgets('redirects /reader/:bookId/read to /book/:bookId/read', (tester) async {
      setViewport(tester);
      await tester.pumpWidget(buildApp('/reader/book-1/read'));
      await tester.pumpAndSettle();

      expect(find.byType(ReaderView), findsOneWidget);
    });
  });
}
