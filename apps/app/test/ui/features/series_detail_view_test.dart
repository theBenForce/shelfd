import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/series/series_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockBookRepository implements BookRepository {
  final List<Book> books;

  _MockBookRepository(this.books);

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

class _MockPluginsNotifier extends PluginsNotifier {
  @override
  PluginsState build() {
    return const PluginsState(
      plugins: [
        PluginSummary(
          name: 'yoto',
          displayName: 'Yoto Player',
          version: '0.1.0',
          enabled: true,
          status: 'idle',
          capabilities: ['actions'],
          actions: [
            PluginActionDef(
              name: 'send_to_yoto',
              title: 'Send to Yoto',
              icon: 'speaker',
              targets: ['book', 'series'],
              mediaTypes: ['audiobook'],
              mode: 'form',
              execution: 'job',
            ),
          ],
        ),
      ],
    );
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('compareSeriesBooks sorts numbered books first then unnumbered alphabetically', () {
    const s = Series(id: 's1', name: 'Percy Jackson');
    const b1 = Book(id: 'b1', title: 'The Lightning Thief', series: s, seriesSequence: 1.0);
    const b2 = Book(id: 'b2', title: 'The Sea of Monsters', series: s, seriesSequence: 2.0);
    const bGuide = Book(id: 'b3', title: 'The Ultimate Guide', series: s);
    const bCompanion = Book(id: 'b4', title: 'Camp Half-Blood Confidential', series: s);

    final list = [bGuide, b2, bCompanion, b1];
    list.sort(compareSeriesBooks);

    expect(list[0].title, 'The Lightning Thief'); // #1
    expect(list[1].title, 'The Sea of Monsters'); // #2
    expect(list[2].title, 'Camp Half-Blood Confidential'); // Unnumbered C
    expect(list[3].title, 'The Ultimate Guide'); // Unnumbered U
  });

  testWidgets('SeriesDetailView renders header, series sequence badges, books, and dynamic series plugin action button',
      (tester) async {
    const s = Series(id: 's1', name: 'Percy Jackson');
    final books = [
      const Book(
        id: 'b1',
        title: 'The Lightning Thief',
        authors: [Author(id: 'a1', name: 'Rick Riordan')],
        series: s,
        seriesSequence: 1.0,
      ),
      const Book(
        id: 'b2',
        title: 'The Sea of Monsters',
        authors: [Author(id: 'a1', name: 'Rick Riordan')],
        series: s,
        seriesSequence: 2.0,
        bookType: 'audiobook',
        durationSeconds: 1800.0,
      ),
      const Book(
        id: 'b3',
        title: 'The Ultimate Guide',
        authors: [Author(id: 'a1', name: 'Rick Riordan')],
        series: s,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          bookRepositoryProvider.overrideWithValue(_MockBookRepository(books)),
          pluginsProvider.overrideWith(_MockPluginsNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SeriesDetailView(
            seriesId: 's1',
            seriesName: 'Percy Jackson',
          ),
        ),
      ),
    );

    // Initial pump and settle after async loading
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Verify Series title, book count, and dynamic Send to Yoto plugin button
    expect(find.text('Percy Jackson'), findsWidgets);
    expect(find.text('3 Books in Series'), findsOneWidget);
    expect(find.text('3 Books'), findsOneWidget);
    expect(
      find.byKey(const Key('series_plugin_action_yoto_send_to_yoto')),
      findsOneWidget,
    );

    // Verify books are displayed with TactileBookCover
    expect(find.byType(TactileBookCover), findsNWidgets(3));
    expect(find.text('The Lightning Thief'), findsOneWidget);
    expect(find.text('The Sea of Monsters'), findsOneWidget);
    expect(find.text('The Ultimate Guide'), findsOneWidget);

    // Verify sequence badges #1 and #2
    expect(find.text('#1'), findsOneWidget);
    expect(find.text('#2'), findsOneWidget);
  });
}
