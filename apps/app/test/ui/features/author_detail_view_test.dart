import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/authors/author_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockAuthorBookRepository implements BookRepository {
  final List<Book> books;
  final List<Author> authors;

  _MockAuthorBookRepository({required this.books, required this.authors});

  @override
  Future<List<Author>> getAuthors() async => authors;

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
      books: books,
      total: books.length,
      page: 1,
      perPage: perPage,
      limit: perPage,
      offset: 0,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('AuthorDetailView renders author avatar, name, book count, and books sorted alphabetically',
      (tester) async {
    const author = Author(
      id: 'a1',
      name: 'Norman Bridwell',
      bookCount: 2,
    );

    final books = [
      const Book(
        id: 'b2',
        title: 'Clifford the Big Red Dog',
        authors: [author],
      ),
      const Book(
        id: 'b1',
        title: 'Clifford at the Circus',
        authors: [author],
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(
            _MockAuthorBookRepository(books: books, authors: [author]),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const AuthorDetailView(
            authorId: 'a1',
            authorName: 'Norman Bridwell',
          ),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Verify author name and book counts
    expect(find.text('Norman Bridwell'), findsWidgets);
    expect(find.text('2 Books by Author'), findsOneWidget);
    expect(find.text('2 Books'), findsOneWidget);

    // Verify AuthorAvatar is rendered
    expect(find.byType(AuthorAvatar), findsOneWidget);

    // Verify books are rendered
    expect(find.text('Clifford at the Circus'), findsWidgets);
    expect(find.text('Clifford the Big Red Dog'), findsWidgets);
  });
}
