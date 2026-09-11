import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/library/library_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeLibraryNotifier extends LibraryNotifier {
  final List<Book> _initialBooks;
  final List<Series> _initialSeries;
  final List<Author> _initialAuthors;
  final List<Genre> _initialGenres;
  final List<Topic> _initialTopics;
  final String _filter;
  final String _searchQuery;

  FakeLibraryNotifier(
    this._initialBooks, {
    this._initialSeries = const [],
    this._initialAuthors = const [],
    this._initialGenres = const [],
    this._initialTopics = const [],
    this._filter = 'all',
    this._searchQuery = '',
  });

  @override
  LibraryState build() => LibraryState(
        books: _initialBooks,
        series: _initialSeries,
        authors: _initialAuthors,
        genres: _initialGenres,
        topics: _initialTopics,
        activeFilter: _filter,
        searchQuery: _searchQuery,
        isLoading: false,
      );

  @override
  void setSearchQuery(String query) {
    state = state.copyWith(searchQuery: query);
  }

  @override
  void setFilter(String filter) {
    state = state.copyWith(activeFilter: filter);
  }
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

  testWidgets('LibraryView renders genre cards when genres filter is active',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testGenres = [
      const Genre(id: 'g1', name: 'Science Fiction', bookCount: 15),
      const Genre(id: 'g2', name: 'Fantasy', bookCount: 7),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier(
                const [],
                initialGenres: testGenres,
                filter: 'genres',
              )),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(mode: LibraryViewMode.genres),
        ),
      ),
    );

    expect(find.text('Science Fiction'), findsOneWidget);
    expect(find.text('15 Books'), findsOneWidget);
    expect(find.text('Fantasy'), findsOneWidget);
    expect(find.text('7 Books'), findsOneWidget);
  });

  testWidgets('LibraryView renders topic cards when topics filter is active',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testTopics = [
      const Topic(id: 't1', name: 'Productivity', bookCount: 3),
      const Topic(id: 't2', name: 'Deep Work', bookCount: 2),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(() => FakeLibraryNotifier(
                const [],
                initialTopics: testTopics,
                filter: 'topics',
              )),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const LibraryView(mode: LibraryViewMode.topics),
        ),
      ),
    );

    expect(find.text('Productivity'), findsOneWidget);
    expect(find.text('3 Books'), findsOneWidget);
    expect(find.text('Deep Work'), findsOneWidget);
    expect(find.text('2 Books'), findsOneWidget);
  });

  testWidgets('LibraryView filters books live by search query and Author:"Ryder Carrol"',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testBooks = [
      const Book(
        id: 'b1',
        title: 'The Bullet Journal Method',
        authors: [Author(id: 'a1', name: 'Ryder Carroll')],
      ),
      const Book(
        id: 'b2',
        title: 'Deep Work',
        authors: [Author(id: 'a2', name: 'Cal Newport')],
      ),
    ];

    final notifier = FakeLibraryNotifier(testBooks);

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

    expect(find.text('The Bullet Journal Method'), findsWidgets);
    expect(find.text('Deep Work'), findsWidgets);

    // Type Author:"Ryder Carrol" into the filter bar
    await tester.enterText(find.byType(TextField), 'Author:"Ryder Carrol"');
    await tester.pump();

    // Only Bullet Journal should remain
    expect(find.text('The Bullet Journal Method'), findsWidgets);
    expect(find.text('Deep Work'), findsNothing);
  });

  testWidgets('LibraryView displays empty search state with clear search button when no books match',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testBooks = [
      const Book(
        id: 'b1',
        title: 'The Bullet Journal Method',
        authors: [Author(id: 'a1', name: 'Ryder Carroll')],
      ),
    ];

    final notifier = FakeLibraryNotifier(testBooks);

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

    // Type non-existent query
    await tester.enterText(find.byType(TextField), 'Nonexistent Book');
    await tester.pump();

    expect(find.text('No matching books'), findsOneWidget);
    expect(find.text('Clear search'), findsOneWidget);

    // Tap clear search
    await tester.tap(find.text('Clear search'));
    await tester.pump();

    expect(find.text('The Bullet Journal Method'), findsWidgets);
    expect(find.text('No matching books'), findsNothing);
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

