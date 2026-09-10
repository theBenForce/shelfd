import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/library/library_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeLibraryNotifier extends LibraryNotifier {
  final List<Book> _initialBooks;
  final List<Series> _initialSeries;
  final List<Author> _initialAuthors;
  final String _filter;

  FakeLibraryNotifier(
    this._initialBooks, {
    this._initialSeries = const [],
    this._initialAuthors = const [],
    this._filter = 'all',
  });

  @override
  LibraryState build() => LibraryState(
        books: _initialBooks,
        series: _initialSeries,
        authors: _initialAuthors,
        activeFilter: _filter,
        isLoading: false,
      );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('LibraryView displays book grid, filter pills, and bottom navigation',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testBooks = [
      const Book(
        id: 'b1',
        title: 'The Left Hand of Darkness',
        authors: [Author(id: 'a1', name: 'Ursula K. Le Guin')],
        readingProgress: 0.64,
      ),
      const Book(
        id: 'b2',
        title: 'Neuromancer',
        authors: [Author(id: 'a2', name: 'William Gibson')],
        readingProgress: 0.0,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier(testBooks)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    // Verify app bar and homelab connection indicator
    expect(find.text('Shelfd'), findsOneWidget);
    expect(find.text('Connected to Homelab NAS'), findsOneWidget);

    // Verify filter pills
    expect(find.text('All Books'), findsOneWidget);
    expect(find.widgetWithText(ChoiceChip, 'Series'), findsOneWidget);

    // Verify book cards
    expect(find.text('The Left Hand of Darkness'), findsWidgets);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
    expect(find.text('Neuromancer'), findsWidgets);
    expect(find.text('William Gibson'), findsOneWidget);

    // Verify reading progress badge
    expect(find.text('64% read'), findsOneWidget);
  });

  testWidgets('LibraryView adapts grid columns responsively with MaxCrossAxisExtent',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    tester.view.physicalSize = const Size(1920, 1080);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier([
            const Book(id: 'b1', title: 'Book 1', readingProgress: 0.0),
          ])),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    final gridFinder = find.byType(GridView);
    expect(gridFinder, findsOneWidget);

    final grid = tester.widget<GridView>(gridFinder);
    expect(grid.gridDelegate, isA<SliverGridDelegateWithMaxCrossAxisExtent>());

    final delegate = grid.gridDelegate as SliverGridDelegateWithMaxCrossAxisExtent;
    expect(delegate.maxCrossAxisExtent, 200.0);
    expect(delegate.childAspectRatio, 0.55);

    // Verify desktop layout has side navigation and hides bottom navigation
    expect(find.byType(ShelfdSideNav), findsOneWidget);
    expect(find.byType(ShelfdBottomNav), findsNothing);
    expect(find.text('Library'), findsWidgets);
    expect(find.text('1 Books'), findsOneWidget);
  });

  testWidgets('LibraryView renders bottom loading spinner when isLoadingMore is true',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final notifier = FakePagingLibraryNotifier(
      [const Book(id: 'b1', title: 'Book 1', readingProgress: 0.0)],
      isLoadingMore: true,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => notifier),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  testWidgets('LibraryView triggers loadMoreBooks when scrolling near bottom',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testBooks = List.generate(
      24,
      (i) => Book(id: 'b$i', title: 'Book $i', readingProgress: 0.0),
    );

    final notifier = FakePagingLibraryNotifier(testBooks);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => notifier),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    await tester.drag(find.byType(GridView), const Offset(0, -2000));
    await tester.pumpAndSettle();

    expect(notifier.loadMoreCalls, greaterThanOrEqualTo(1));
  });

  testWidgets('LibraryView renders series cards when series filter is active',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testSeries = [
      const Series(id: 's1', name: 'Clifford', bookCount: 5),
      const Series(id: 's2', name: 'Percy Jackson', bookCount: 3),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier(
                const [],
                initialSeries: testSeries,
                filter: 'series',
              )),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    expect(find.text('Clifford'), findsWidgets);
    expect(find.text('5 Books'), findsOneWidget);
    expect(find.text('Percy Jackson'), findsWidgets);
    expect(find.text('3 Books'), findsOneWidget);
  });

  testWidgets('LibraryView renders author cards when authors filter is active',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testAuthors = [
      const Author(id: 'a1', name: 'Norman Bridwell', bookCount: 8),
      const Author(id: 'a2', name: 'Rick Riordan', bookCount: 4),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier(
                const [],
                initialAuthors: testAuthors,
                filter: 'authors',
              )),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(),
        ),
      ),
    );

    expect(find.text('Norman Bridwell'), findsOneWidget);
    expect(find.text('8 Books'), findsOneWidget);
    expect(find.text('Rick Riordan'), findsOneWidget);
    expect(find.text('4 Books'), findsOneWidget);
  });
}

class FakePagingLibraryNotifier extends LibraryNotifier {
  final List<Book> _initialBooks;
  final bool isLoadingMore;
  int loadMoreCalls = 0;

  FakePagingLibraryNotifier(this._initialBooks, {this.isLoadingMore = false});

  @override
  LibraryState build() => LibraryState(
        books: _initialBooks,
        isLoading: false,
        isLoadingMore: isLoadingMore,
        hasMore: true,
        totalBooks: 50,
      );

  @override
  Future<void> loadMoreBooks() async {
    loadMoreCalls++;
  }
}

