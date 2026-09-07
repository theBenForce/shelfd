import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/search_result.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/search/search_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeSearchNotifier extends SearchNotifier {
  final List<SemanticSearchHit> _initialResults;
  FakeSearchNotifier(this._initialResults);

  @override
  SearchState build() => SearchState(
        query: 'ice escape',
        results: _initialResults,
        isLoading: false,
      );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('SearchView renders NLP search bar, hit cards, and match score badges',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testHits = [
      const SemanticSearchHit(
        bookId: 'b-1',
        bookTitle: 'The Left Hand of Darkness',
        authorName: 'Ursula K. Le Guin',
        chapterIndex: 14,
        chapterTitle: 'Across the Gobrin Ice',
        summary: 'Genly Ai and Estraven trek across the grueling Gobrin Ice glacier.',
        score: 0.94,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          searchProvider.overrideWith(() => FakeSearchNotifier(testHits)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SearchView(),
        ),
      ),
    );

    // Verify search input bar
    expect(find.byType(TextField), findsOneWidget);
    expect(find.byIcon(Icons.auto_awesome_rounded), findsOneWidget);

    // Verify search hit card content
    expect(find.text('The Left Hand of Darkness'), findsOneWidget);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
    expect(find.text('Chapter 14: Across the Gobrin Ice'), findsOneWidget);
    expect(find.text('94% match'), findsOneWidget);
    expect(find.textContaining('Genly Ai and Estraven trek across'), findsOneWidget);

    // Verify read action
    expect(find.text('Read Chapter at this passage →'), findsOneWidget);
  });
}
