import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/topics/topic_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockTopicBookRepository implements BookRepository {
  final List<Book> books;
  final List<Topic> topics;

  _MockTopicBookRepository({required this.books, required this.topics});

  @override
  Future<List<Topic>> getTopics() async => topics;

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

  testWidgets('TopicDetailView renders topic header, book count, and TactileBookCover cards',
      (tester) async {
    const topic = Topic(id: 't1', name: 'Productivity', bookCount: 2);
    final books = [
      const Book(
        id: 'b1',
        title: 'Deep Work',
        authors: [Author(id: 'a1', name: 'Cal Newport')],
        topics: [topic],
      ),
      const Book(
        id: 'b2',
        title: 'Atomic Habits',
        authors: [Author(id: 'a2', name: 'James Clear')],
        topics: [topic],
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(
            _MockTopicBookRepository(books: books, topics: [topic]),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.sepia),
          home: const TopicDetailView(
            topicId: 't1',
            topicName: 'Productivity',
          ),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Productivity'), findsWidgets);
    expect(find.text('2 Books on Topic'), findsOneWidget);
    expect(find.text('2 Books'), findsOneWidget);
    expect(find.byType(TactileBookCover), findsNWidgets(2));
    expect(find.text('Atomic Habits'), findsOneWidget);
    expect(find.text('Deep Work'), findsOneWidget);
  });
}
