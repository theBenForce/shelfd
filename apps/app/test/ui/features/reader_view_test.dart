import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/data/models/highlight.dart';
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

  testWidgets('ReaderView renders rich markdown headings, blockquotes, and inline footnotes',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final formattedChapter = const Chapter(
      id: 'chap-6',
      bookId: 'book-42',
      chapterIndex: 6,
      title: 'Chapter 1: The Realignment',
      summary: 'History of the Dixiecrats.',
      content:
          '> Authoritarian enclaves were founded in the South.[3]\n\n## The reign of the Dixiecrats\n\nRed-blooded men know what I mean.”[5] He won the race.',
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
            chapterIndex: 6,
            initialChapter: formattedChapter,
          ),
        ),
      ),
    );

    // Verify heading renders
    expect(find.text('The reign of the Dixiecrats'), findsOneWidget);

    // Verify blockquote text renders
    expect(find.textContaining('Authoritarian enclaves were founded'), findsOneWidget);

    // Verify footnote superscripts render inline
    expect(find.text('3'), findsOneWidget);
    expect(find.text('5'), findsOneWidget);
    expect(find.textContaining('Red-blooded men know what I mean.”'), findsOneWidget);
    expect(find.textContaining('He won the race.'), findsOneWidget);
  });

  testWidgets('KindleSelectionToolbar renders 4 Kindle colors, note, and copy buttons', (tester) async {
    KindleHighlightColor? selectedColor;
    bool noteTapped = false;
    bool copyTapped = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: KindleSelectionToolbar(
            anchors: const TextSelectionToolbarAnchors(primaryAnchor: Offset(100, 100)),
            onColorSelected: (c) => selectedColor = c,
            onAddNote: () => noteTapped = true,
            onCopy: () => copyTapped = true,
          ),
        ),
      ),
    );

    // Verify all 4 Kindle colors are present
    expect(find.byTooltip('Yellow Highlight'), findsOneWidget);
    expect(find.byTooltip('Blue Highlight'), findsOneWidget);
    expect(find.byTooltip('Pink Highlight'), findsOneWidget);
    expect(find.byTooltip('Orange Highlight'), findsOneWidget);

    // Verify Add Note and Copy buttons
    expect(find.byTooltip('Add Note'), findsOneWidget);
    expect(find.byTooltip('Copy'), findsOneWidget);

    // Tap Blue color button
    await tester.tap(find.byTooltip('Blue Highlight'));
    expect(selectedColor, KindleHighlightColor.blue);

    // Tap Add Note button
    await tester.tap(find.byTooltip('Add Note'));
    expect(noteTapped, true);

    // Tap Copy button
    await tester.tap(find.byTooltip('Copy'));
    expect(copyTapped, true);
  });

  testWidgets('KindleSelectionToolbar renders without external Material ancestor and displays delete button', (tester) async {
    bool deleteTapped = false;

    // Pump directly inside MaterialApp WITHOUT Scaffold or external Material ancestor
    await tester.pumpWidget(
      MaterialApp(
        home: KindleSelectionToolbar(
          anchors: const TextSelectionToolbarAnchors(primaryAnchor: Offset(150, 150)),
          selectedColor: KindleHighlightColor.yellow,
          onColorSelected: (_) {},
          onAddNote: () {},
          onCopy: () {},
          onDelete: () => deleteTapped = true,
        ),
      ),
    );

    // Verify it renders safely without "No Material widget found" error
    expect(find.byType(KindleSelectionToolbar), findsOneWidget);
    expect(find.byTooltip('Edit Note'), findsOneWidget);
    expect(find.byTooltip('Delete Highlight'), findsOneWidget);
    expect(find.byIcon(Icons.check), findsOneWidget); // checkmark on selected yellow color

    // Tap Delete button
    await tester.tap(find.byTooltip('Delete Highlight'));
    expect(deleteTapped, true);
  });

  testWidgets('ReaderView renders highlights from bookDetailProvider and opens detail sheet on tap',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testChapter = const Chapter(
      id: 'chap-10',
      bookId: 'book-100',
      chapterIndex: 1,
      title: 'Arrival',
      content: 'Small town winter was unusually bitter.',
    );

    final highlight = const Highlight(
      id: 'hl-test-1',
      bookId: 'book-100',
      chapterId: 'chap-10',
      selectedText: 'Small town',
      note: 'My childhood memory',
      color: 'pink',
      startOffset: 0,
      endOffset: 10,
      startParagraph: 1,
      endParagraph: 1,
      location: 'p.1:0',
    );

    final mockBook = Book(
      id: 'book-100',
      title: 'A Memoir',
      spine: const [SpineItem(id: 'chap-10', bookId: 'book-100', title: 'Arrival', chapterIndex: 1)],
      highlights: [highlight],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-100').overrideWith(() => _MockBookDetailNotifier(mockBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: ReaderView(
            bookId: 'book-100',
            chapterIndex: 1,
            initialChapter: testChapter,
          ),
        ),
      ),
    );

    await tester.pumpAndSettle();

    // Verify text renders
    expect(find.textContaining('Small town'), findsOneWidget);

    // Tap highlighted span at start
    final textFinder = find.textContaining('Small town');
    final topLeft = tester.getTopLeft(textFinder);
    await tester.tapAt(topLeft + const Offset(25, 8));
    await tester.pumpAndSettle();

    // Verify existing highlight bottom sheet opened with quote and note
    expect(find.text('Highlight'), findsOneWidget);
    expect(find.textContaining('Small town'), findsWidgets);
    expect(find.text('My childhood memory'), findsOneWidget);
    expect(find.text('Location: p.1:0'), findsOneWidget);
    expect(find.text('Delete Highlight'), findsOneWidget);
  });

  testWidgets('ReaderView resolves displayTitle using spine manifest when chapter title is synthetic ULID or empty',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final testChapter = const Chapter(
      id: '01M22YCE6D6GZJAXZC8A39AVKE',
      bookId: 'book-200',
      chapterIndex: 5,
      title: 'Chapter 01M22YCE6D6GZJAXZC8A39AVKE',
      content: 'Chapter 5 body content about demographic changes.',
    );

    final mockBook = Book(
      id: 'book-200',
      title: 'Why We Are Polarized',
      spine: const [
        SpineItem(
          id: '01M22YCE6D6GZJAXZC8A39AVKE',
          bookId: 'book-200',
          title: 'Chapter 5: Demographic Threat',
          chapterIndex: 5,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-200').overrideWith(() => _MockBookDetailNotifier(mockBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: ReaderView(
            bookId: 'book-200',
            chapterIdentifier: '01M22YCE6D6GZJAXZC8A39AVKE',
            initialChapter: testChapter,
          ),
        ),
      ),
    );

    await tester.pumpAndSettle();

    // Verify AppBar renders the human-readable spine title instead of the synthetic ULID title
    expect(find.text('Chapter 5: Demographic Threat'), findsOneWidget);
    expect(find.text('Chapter 01M22YCE6D6GZJAXZC8A39AVKE'), findsNothing);

    // Verify bottom bar also renders the human-readable spine title
    expect(find.textContaining('Chapter 5: Demographic Threat • 8 mins left'), findsOneWidget);
    expect(find.textContaining('Chapter 01M22YCE6D6GZJAXZC8A39AVKE • 8 mins left'), findsNothing);
  });
}

class _MockBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  _MockBookDetailNotifier(this.mockBook) : super('book-100');

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }

  @override
  Future<void> loadBook() async {
    // No-op in mock test
  }
}

