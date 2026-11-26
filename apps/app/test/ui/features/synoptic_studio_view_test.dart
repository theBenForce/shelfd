import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/search_result.dart';
import 'package:shelf/data/models/synoptic.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/search/search_view.dart';
import 'package:shelf/ui/features/search/synoptic_studio_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeLibraryNotifier extends LibraryNotifier {
  final LibraryState _initialState;
  _FakeLibraryNotifier(this._initialState);

  @override
  LibraryState build() => _initialState;

  @override
  Future<void> loadLibrary({bool refresh = false}) async {}
}

class _FakeSearchNotifier extends SearchNotifier {
  final SearchState _initialState;
  _FakeSearchNotifier(this._initialState);

  @override
  SearchState build() => _initialState;
}

class _FakeSynopticNotifier extends SynopticNotifier {
  final SynopticState _initialState;
  _FakeSynopticNotifier(this._initialState);

  @override
  SynopticState build() => _initialState;
}

LibraryState _sampleLibraryState() {
  return const LibraryState(
    books: [
      Book(id: 'book-dune', title: 'Dune'),
      Book(id: 'book-rabbit', title: 'The Tale of Peter Rabbit'),
      Book(id: 'book-earthsea', title: 'A Wizard of Earthsea'),
    ],
    genres: [
      Genre(id: 'genre-scifi', name: 'Science Fiction', bookCount: 2),
      Genre(id: 'genre-fantasy', name: 'Fantasy', bookCount: 1),
    ],
  );
}

SynopticResult _sampleSynopticResult() {
  return const SynopticResult(
    summary: 'Both books examine survival under harsh rules and temptation.',
    consensusPoints: [
      SynopticPoint(
        topic: 'Testing courage',
        description: 'Each protagonist faces a high-stakes trial early in the story.',
        citationIndices: [0, 1],
      ),
    ],
    divergencePoints: [
      SynopticPoint(
        topic: 'Scale of stakes',
        description: 'Dune operates at galactic scale while Peter Rabbit is domestic.',
        citationIndices: [1],
      ),
    ],
    citations: [
      SynopticCitation(
        bookId: 'book-dune',
        bookTitle: 'Dune',
        authorName: 'Frank Herbert',
        chapterId: 'ch-1',
        chapterTitle: 'Gom Jabbar',
        chapterIndex: 1,
        startParagraph: 2,
        endParagraph: 4,
        excerpt: 'Fear is the mind-killer.',
      ),
      SynopticCitation(
        bookId: 'book-rabbit',
        bookTitle: 'The Tale of Peter Rabbit',
        authorName: 'Beatrix Potter',
        chapterId: 'ch-2',
        chapterTitle: 'Mr. McGregor Garden',
        chapterIndex: 2,
        startParagraph: 0,
        endParagraph: 2,
        excerpt: 'Peter squeezed under the gate.',
      ),
    ],
    booksCovered: 2,
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('SynopticStudioView handles book selection and Clear button',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(
            () => _FakeLibraryNotifier(_sampleLibraryState()),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(body: SynopticStudioView()),
        ),
      ),
    );

    expect(find.text('0 selected'), findsOneWidget);
    expect(find.text('Clear'), findsNothing);

    await tester.tap(find.text('Dune'));
    await tester.pumpAndSettle();
    expect(find.text('1 selected'), findsOneWidget);
    expect(find.text('Clear'), findsOneWidget);

    await tester.tap(find.text('The Tale of Peter Rabbit'));
    await tester.pumpAndSettle();
    expect(find.text('2 selected'), findsOneWidget);

    await tester.tap(find.text('Clear'));
    await tester.pumpAndSettle();
    expect(find.text('0 selected'), findsOneWidget);
    expect(find.text('Clear'), findsNothing);
  });

  testWidgets(
      'SynopticStudioView shows inline validation when fewer than 2 books and no genre are selected',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    var apiCalls = 0;

    final mockClient = MockClient((request) async {
      apiCalls++;
      return http.Response(jsonEncode(_sampleSynopticResult().toJson()), 200);
    });
    final apiService = ApiService(
      baseUrl: 'http://localhost:8080',
      client: mockClient,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          apiServiceProvider.overrideWithValue(apiService),
          libraryProvider.overrideWith(
            () => _FakeLibraryNotifier(_sampleLibraryState()),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(body: SynopticStudioView()),
        ),
      ),
    );

    await tester.tap(find.text('Dune'));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byType(TextField),
      'How do these books portray courage?',
    );
    await tester.tap(find.text('Compare'));
    await tester.pumpAndSettle();

    expect(apiCalls, 0);
    expect(
      find.text('Select at least 2 books or choose a genre filter to compare.'),
      findsOneWidget,
    );

    await tester.tap(find.text('Science Fiction'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Compare'));
    await tester.pumpAndSettle();

    expect(apiCalls, 1);
    expect(find.text('Executive Summary'), findsOneWidget);
  });

  testWidgets(
      'SynopticStudioView renders consensus/divergence cards and routes on citation tap',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    String? navigatedRoute;

    final router = GoRouter(
      initialLocation: '/search',
      routes: [
        GoRoute(
          path: '/search',
          builder: (context, state) => const Scaffold(body: SynopticStudioView()),
        ),
        GoRoute(
          path: '/books/:bookId/:chapter',
          builder: (context, state) {
            navigatedRoute =
                '/books/${state.pathParameters['bookId']}/${state.pathParameters['chapter']}';
            return Scaffold(body: Text('Reader $navigatedRoute'));
          },
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(
            () => _FakeLibraryNotifier(_sampleLibraryState()),
          ),
          synopticProvider.overrideWith(
            () => _FakeSynopticNotifier(
              SynopticState(
                selectedBookIds: const ['book-dune', 'book-rabbit'],
                query: 'Compare courage and stakes',
                result: _sampleSynopticResult(),
              ),
            ),
          ),
        ],
        child: MaterialApp.router(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          routerConfig: router,
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.text('Executive Summary'), findsOneWidget);
    expect(
      find.text('Both books examine survival under harsh rules and temptation.'),
      findsOneWidget,
    );
    expect(find.text('Consensus Points'), findsOneWidget);
    expect(find.text('Testing courage'), findsOneWidget);
    expect(find.text('Divergence Points'), findsOneWidget);
    expect(find.text('Scale of stakes'), findsOneWidget);

    final duneChip = find.text('Dune · Ch. 1').first;
    await tester.drag(find.byType(ListView), const Offset(0, -400));
    await tester.pumpAndSettle();
    await tester.tap(duneChip);
    await tester.pumpAndSettle();

    expect(navigatedRoute, '/books/book-dune/1');
    expect(find.text('Reader /books/book-dune/1'), findsOneWidget);
  });

  testWidgets(
      'SearchView defaults to Search mode and toggles cleanly to Synoptic Studio and back',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    const hit = SemanticSearchHit(
      bookId: 'book-dune',
      bookTitle: 'Dune',
      authorName: 'Frank Herbert',
      chapterIndex: 1,
      chapterTitle: 'Gom Jabbar',
      summary: 'Paul faces the Gom Jabbar test.',
      score: 0.92,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(
            () => _FakeLibraryNotifier(_sampleLibraryState()),
          ),
          searchProvider.overrideWith(
            () => _FakeSearchNotifier(
              const SearchState(query: 'Gom Jabbar', results: [hit]),
            ),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SearchView(),
        ),
      ),
    );

    final segmented = find.byType(SegmentedButton<SearchViewMode>);
    final searchToggle =
        find.descendant(of: segmented, matching: find.text('Search'));
    final synopticToggle =
        find.descendant(of: segmented, matching: find.text('Synoptic Studio'));

    expect(searchToggle, findsOneWidget);
    expect(synopticToggle, findsOneWidget);
    expect(find.text('Paul faces the Gom Jabbar test.'), findsOneWidget);

    await tester.tap(synopticToggle);
    await tester.pumpAndSettle();

    expect(find.byType(SynopticStudioView), findsOneWidget);
    expect(find.text('Select Books'), findsOneWidget);

    await tester.tap(searchToggle);
    await tester.pumpAndSettle();

    expect(find.byType(SynopticStudioView), findsNothing);
    expect(find.text('Paul faces the Gom Jabbar test.'), findsOneWidget);
  });

  testWidgets('SynopticStudioView renders without overflow on 390x844 mobile viewport',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          libraryProvider.overrideWith(
            () => _FakeLibraryNotifier(_sampleLibraryState()),
          ),
          synopticProvider.overrideWith(
            () => _FakeSynopticNotifier(
              SynopticState(
                selectedBookIds: const ['book-dune', 'book-rabbit'],
                query: 'Compare themes',
                result: _sampleSynopticResult(),
              ),
            ),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(body: SynopticStudioView()),
        ),
      ),
    );

    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.text('Executive Summary'), findsOneWidget);
  });
}
