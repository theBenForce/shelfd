import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/genres/genre_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockGenreBookRepository implements BookRepository {
  final List<Book> books;
  final List<Genre> genres;

  _MockGenreBookRepository({required this.books, required this.genres});

  @override
  Future<List<Genre>> getGenres() async => genres;

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

  testWidgets('GenreDetailView renders genre header, book count, and TactileBookCover cards',
      (tester) async {
    const genre = Genre(id: 'g1', name: 'Science Fiction', bookCount: 2);
    final books = [
      const Book(
        id: 'b2',
        title: 'Neuromancer',
        authors: [Author(id: 'a2', name: 'William Gibson')],
        genres: [genre],
      ),
      const Book(
        id: 'b1',
        title: 'Dune',
        authors: [Author(id: 'a1', name: 'Frank Herbert')],
        genres: [genre],
        readingProgress: 0.35,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(
            _MockGenreBookRepository(books: books, genres: [genre]),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: const GenreDetailView(
            genreId: 'g1',
            genreName: 'Science Fiction',
          ),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Science Fiction'), findsWidgets);
    expect(find.text('2 Books in Genre'), findsOneWidget);
    expect(find.text('2 Books'), findsOneWidget);
    expect(find.byType(TactileBookCover), findsNWidgets(2));
    expect(find.text('Dune'), findsOneWidget);
    expect(find.text('Neuromancer'), findsOneWidget);
  });
}
