import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/library/library_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeLibraryNotifier extends LibraryNotifier {
  final List<Book> _initialBooks;
  FakeLibraryNotifier(this._initialBooks);

  @override
  LibraryState build() => LibraryState(books: _initialBooks, isLoading: false);
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
    expect(find.text('Series'), findsOneWidget);

    // Verify book cards
    expect(find.text('The Left Hand of Darkness'), findsWidgets);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
    expect(find.text('Neuromancer'), findsWidgets);
    expect(find.text('William Gibson'), findsOneWidget);

    // Verify reading progress badge
    expect(find.text('64% read'), findsOneWidget);
  });
}
