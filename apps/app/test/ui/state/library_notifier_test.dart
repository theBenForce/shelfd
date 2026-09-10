import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('LibraryNotifier paginates and accumulates books with loadMoreBooks', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storageService = StorageService(prefs);

    int requestedPage = 0;
    final mockClient = MockClient((request) async {
      final uri = request.url;
      if (uri.path == '/api/v1/authors' || uri.path == '/api/v1/genres' || uri.path == '/api/v1/series') {
        return http.Response('[]', 200, headers: {'content-type': 'application/json'});
      }

      if (uri.path == '/api/v1/books') {
        final page = int.parse(uri.queryParameters['page'] ?? '1');
        requestedPage = page;
        if (page == 1) {
          final booksJson = List.generate(
            24,
            (i) => {'id': 'book-page1-$i', 'title': 'Page 1 Book $i'},
          );
          return http.Response(
            jsonEncode({
              'books': booksJson,
              'total': 30,
              'limit': 24,
              'offset': 0,
            }),
            200,
            headers: {'content-type': 'application/json'},
          );
        } else if (page == 2) {
          final booksJson = List.generate(
            6,
            (i) => {'id': 'book-page2-$i', 'title': 'Page 2 Book $i'},
          );
          return http.Response(
            jsonEncode({
              'books': booksJson,
              'total': 30,
              'limit': 24,
              'offset': 24,
            }),
            200,
            headers: {'content-type': 'application/json'},
          );
        }
      }

      return http.Response('Not Found', 404);
    });

    final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
    final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

    final container = ProviderContainer(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
        bookRepositoryProvider.overrideWithValue(bookRepo),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(libraryProvider.notifier);

    // Initial load
    await notifier.loadLibrary();
    var state = container.read(libraryProvider);
    expect(state.books.length, 24);
    expect(state.totalBooks, 30);
    expect(state.currentPage, 1);
    expect(state.hasMore, true);
    expect(state.isLoadingMore, false);

    // Load next page
    await notifier.loadMoreBooks();
    state = container.read(libraryProvider);
    expect(state.books.length, 30);
    expect(state.currentPage, 2);
    expect(state.hasMore, false);
    expect(state.isLoadingMore, false);

    // Try loading again when hasMore is false: should no-op
    await notifier.loadMoreBooks();
    state = container.read(libraryProvider);
    expect(state.books.length, 30);
    expect(requestedPage, 2);
  });

  test('LibraryNotifier.addBook adds new book and updates author/series groupings', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final notifier = container.read(libraryProvider.notifier);
    expect(container.read(libraryProvider).books.isEmpty, isTrue);

    const book = Book(
      id: 'book-1',
      title: 'Dune',
      authors: [Author(id: 'auth-1', name: 'Frank Herbert')],
      series: Series(id: 'series-1', name: 'Dune Chronicles'),
    );

    notifier.addBook(book);

    final state = container.read(libraryProvider);
    expect(state.books.length, 1);
    expect(state.totalBooks, 1);
    expect(state.books.first.title, 'Dune');
    expect(state.authors.length, 1);
    expect(state.authors.first.name, 'Frank Herbert');
    expect(state.series.length, 1);
    expect(state.series.first.name, 'Dune Chronicles');
  });

  test('LibraryNotifier.updateBook updates existing book in place', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final notifier = container.read(libraryProvider.notifier);

    const book = Book(
      id: 'book-1',
      title: 'Dune',
      authors: [Author(id: 'auth-1', name: 'Frank Herbert')],
    );
    notifier.addBook(book);

    const updatedBook = Book(
      id: 'book-1',
      title: 'Dune (Updated Edition)',
      authors: [Author(id: 'auth-1', name: 'Frank Herbert')],
    );
    notifier.updateBook(updatedBook);

    final state = container.read(libraryProvider);
    expect(state.books.length, 1);
    expect(state.books.first.title, 'Dune (Updated Edition)');
  });
}
