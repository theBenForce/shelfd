import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/library/library_filter_bar.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeLibraryNotifier extends LibraryNotifier {
  final List<Author> _authors;
  final List<Genre> _genres;
  final List<Topic> _topics;

  _FakeLibraryNotifier({
    List<Author>? authors,
    List<Genre>? genres,
    List<Topic>? topics,
  })  : _authors = authors ?? [],
        _genres = genres ?? [],
        _topics = topics ?? [];

  @override
  LibraryState build() => LibraryState(
        authors: _authors,
        genres: _genres,
        topics: _topics,
        isLoading: false,
      );

  @override
  void setSearchQuery(String query) {
    state = state.copyWith(searchQuery: query);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  final sampleAuthors = [
    const Author(id: 'a1', name: 'Ryder Carroll', bookCount: 3),
    const Author(id: 'a2', name: 'Robert Caro', bookCount: 2),
    const Author(id: 'a3', name: 'Ursula K. Le Guin', bookCount: 5),
  ];

  final sampleGenres = [
    const Genre(id: 'g1', name: 'Productivity', bookCount: 4),
    const Genre(id: 'g2', name: 'Science Fiction', bookCount: 12),
  ];

  final sampleTopics = [
    const Topic(id: 't1', name: 'Bullet Journal', bookCount: 2),
    const Topic(id: 't2', name: 'Habits', bookCount: 5),
  ];

  Widget buildTestableWidget({
    _FakeLibraryNotifier? notifier,
  }) {
    return ProviderScope(
      overrides: [
        libraryProvider.overrideWith(
          () => notifier ?? _FakeLibraryNotifier(
            authors: sampleAuthors,
            genres: sampleGenres,
            topics: sampleTopics,
          ),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: const Scaffold(
          body: Padding(
            padding: EdgeInsets.all(24.0),
            child: LibraryFilterBar(),
          ),
        ),
      ),
    );
  }

  testWidgets('LibraryFilterBar renders text field with search icon', (tester) async {
    await tester.pumpWidget(buildTestableWidget());
    expect(find.byType(TextField), findsOneWidget);
    expect(find.byIcon(Icons.search_rounded), findsOneWidget);
  });

  testWidgets('Typing free text updates library search query and shows clear button', (tester) async {
    final notifier = _FakeLibraryNotifier(authors: sampleAuthors);
    await tester.pumpWidget(buildTestableWidget(notifier: notifier));

    await tester.enterText(find.byType(TextField), 'Dune');
    await tester.pump();

    expect(notifier.state.searchQuery, 'Dune');
    expect(find.byIcon(Icons.close_rounded), findsOneWidget);

    // Tap clear button
    await tester.tap(find.byIcon(Icons.close_rounded));
    await tester.pump();

    expect(notifier.state.searchQuery, '');
    expect(find.byIcon(Icons.close_rounded), findsNothing);
  });

  testWidgets('Typing author: displays autocomplete suggestions overlay', (tester) async {
    await tester.pumpWidget(buildTestableWidget());

    await tester.enterText(find.byType(TextField), 'author:');
    await tester.pump();

    // All sample authors should be suggested
    expect(find.text('Ryder Carroll'), findsOneWidget);
    expect(find.text('Robert Caro'), findsOneWidget);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
  });

  testWidgets('Typing author:ryd filters suggestions to matching author', (tester) async {
    await tester.pumpWidget(buildTestableWidget());

    await tester.enterText(find.byType(TextField), 'author:ryd');
    await tester.pump();

    expect(find.text('Ryder Carroll'), findsOneWidget);
    expect(find.text('Robert Caro'), findsNothing);
    expect(find.text('Ursula K. Le Guin'), findsNothing);
  });

  testWidgets('Clicking an autocomplete suggestion replaces token with Author:"Name" ', (tester) async {
    final notifier = _FakeLibraryNotifier(authors: sampleAuthors);
    await tester.pumpWidget(buildTestableWidget(notifier: notifier));

    await tester.enterText(find.byType(TextField), 'author:ryd');
    await tester.pump();

    await tester.tap(find.text('Ryder Carroll'));
    await tester.pumpAndSettle();

    final textField = tester.widget<TextField>(find.byType(TextField));
    expect(textField.controller?.text, 'Author:"Ryder Carroll" ');
    expect(notifier.state.searchQuery, 'Author:"Ryder Carroll" ');
  });

  testWidgets('Navigating with Down/Up arrow keys and pressing Enter selects suggestion', (tester) async {
    final notifier = _FakeLibraryNotifier(authors: sampleAuthors);
    await tester.pumpWidget(buildTestableWidget(notifier: notifier));

    await tester.enterText(find.byType(TextField), 'author:');
    await tester.pump();

    // Press Arrow Down to highlight second author (Robert Caro)
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.pump();

    // Press Enter to select
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    final textField = tester.widget<TextField>(find.byType(TextField));
    expect(textField.controller?.text, 'Author:"Robert Caro" ');
    expect(notifier.state.searchQuery, 'Author:"Robert Caro" ');
  });

  testWidgets('Pressing Escape dismisses the autocomplete suggestions overlay', (tester) async {
    await tester.pumpWidget(buildTestableWidget());

    await tester.enterText(find.byType(TextField), 'author:');
    await tester.pump();

    expect(find.text('Ryder Carroll'), findsOneWidget);

    // Press Escape
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();

    expect(find.text('Ryder Carroll'), findsNothing);
  });

  testWidgets('Typing genre: displays genre suggestions', (tester) async {
    final notifier = _FakeLibraryNotifier(genres: sampleGenres);
    await tester.pumpWidget(buildTestableWidget(notifier: notifier));

    await tester.enterText(find.byType(TextField), 'genre:');
    await tester.pump();

    expect(find.text('Productivity'), findsOneWidget);
    expect(find.text('Science Fiction'), findsOneWidget);

    await tester.tap(find.text('Productivity'));
    await tester.pumpAndSettle();

    final textField = tester.widget<TextField>(find.byType(TextField));
    expect(textField.controller?.text, 'Genre:"Productivity" ');
  });

  testWidgets('Typing topic: displays topic suggestions', (tester) async {
    final notifier = _FakeLibraryNotifier(topics: sampleTopics);
    await tester.pumpWidget(buildTestableWidget(notifier: notifier));

    await tester.enterText(find.byType(TextField), 'topic:');
    await tester.pump();

    expect(find.text('Bullet Journal'), findsOneWidget);
    expect(find.text('Habits'), findsOneWidget);

    await tester.tap(find.text('Bullet Journal'));
    await tester.pumpAndSettle();

    final textField = tester.widget<TextField>(find.byType(TextField));
    expect(textField.controller?.text, 'Topic:"Bullet Journal" ');
  });
}
