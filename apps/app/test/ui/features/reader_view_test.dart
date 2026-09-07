import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/reader/reader_view.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('ReaderView renders chapter text, typography controls, and TOC drawer',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testChapter = const Chapter(
      id: 'chap-4',
      bookId: 'book-42',
      chapterIndex: 4,
      title: 'The Archimedes Principle',
      summary: 'Estraven reflects on the political situation.',
      content: 'From the Parade of the nineteenth day we went back to our quarters in Erhenrang.\n\nThe city was quiet now under the cold rain.',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: ReaderView(
            bookId: 'book-42',
            chapterIndex: 4,
            initialChapter: testChapter,
          ),
        ),
      ),
    );

    // Verify top bar elements
    expect(find.text('The Archimedes Principle'), findsOneWidget);
    expect(find.byIcon(Icons.text_fields_rounded), findsOneWidget);
    expect(find.byIcon(Icons.list_rounded), findsOneWidget);

    // Verify reading canvas content
    expect(find.textContaining('From the Parade of the nineteenth day'), findsOneWidget);
    expect(find.textContaining('The city was quiet now'), findsOneWidget);

    // Open typography settings bottom sheet
    await tester.tap(find.byIcon(Icons.text_fields_rounded));
    await tester.pumpAndSettle();

    // Verify theme pills in bottom sheet
    expect(find.text('Bone'), findsOneWidget);
    expect(find.text('Sepia'), findsOneWidget);
    expect(find.text('Dark OLED'), findsOneWidget);

    // Verify font controls
    expect(find.text('Serif'), findsOneWidget);
    expect(find.text('Sans'), findsOneWidget);
    expect(find.byType(Slider), findsOneWidget);
  });
}
