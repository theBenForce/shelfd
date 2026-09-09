import 'package:flutter/material.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/book_chat.dart';
import 'package:shelf/data/models/bookmark.dart';
import 'package:shelf/data/models/highlight.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/book_detail/book_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  final testBook = Book(
    id: 'book-42',
    title: 'The Left Hand of Darkness',
    synopsis: 'A groundbreaking work of science fiction set on the planet Gethen.',
    authors: const [Author(id: 'auth-1', name: 'Ursula K. Le Guin')],
    series: const Series(id: 'ser-1', name: 'Hainish Cycle'),
    seriesSequence: 4.0,
    readingProgress: 0.65,
    publisher: 'Ace Books',
    publishedDate: '1969',
    language: 'en',
    spine: const [
      SpineItem(
        id: 'spine-1',
        bookId: 'book-42',
        chapterIndex: 1,
        title: 'Chapter 1: A Parade in Erhenrang',
        summary: 'Genly Ai attends the King\'s festival.',
      ),
      SpineItem(
        id: 'spine-2',
        bookId: 'book-42',
        chapterIndex: 2,
        title: 'Chapter 2: The Place Inside the Blizzard',
        summary: 'An ancient hearth-tale.',
      ),
    ],
    bookmarks: const [
      Bookmark(
        id: 'bm-1',
        bookId: 'book-42',
        title: 'Mid-Parade passage',
        progress: 0.45,
      ),
    ],
    highlights: const [
      Highlight(
        id: 'hl-1',
        bookId: 'book-42',
        selectedText: 'Light is the left hand of darkness and darkness the right hand of light.',
        note: 'Famous duality quote',
        color: 'yellow',
      ),
    ],
  );

  testWidgets('BookDetailView renders metadata, tabs, and actions', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Verify title, author, and series
    expect(find.text('The Left Hand of Darkness'), findsWidgets);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
    expect(find.text('Hainish Cycle #4'), findsOneWidget);

    // Verify reading progress & CTA
    expect(find.text('Reading Progress'), findsOneWidget);
    expect(find.text('65%'), findsOneWidget);
    expect(find.text('Resume Reading'), findsOneWidget);

    // Verify Tabs
    expect(find.text('Overview'), findsOneWidget);
    expect(find.text('Highlights & Bookmarks'), findsOneWidget);
    expect(find.text('Chat with Book'), findsOneWidget);

    // Verify Overview content (Synopsis & TOC)
    expect(find.text('Synopsis'), findsOneWidget);
    expect(find.text('Table of Contents'), findsOneWidget);
    expect(find.text('Chapter 1: A Parade in Erhenrang'), findsOneWidget);

    // Switch to Highlights & Bookmarks Tab
    await tester.tap(find.text('Highlights & Bookmarks'));
    await tester.pumpAndSettle();

    // Verify highlight quote & bookmark title
    expect(
      find.textContaining('Light is the left hand of darkness'),
      findsOneWidget,
    );
    expect(find.text('Mid-Parade passage'), findsOneWidget);
    expect(find.text('Famous duality quote'), findsOneWidget);

    // Switch to Chat with Book Tab
    await tester.tap(find.text('Chat with Book'));
    await tester.pumpAndSettle();

    // Verify RAG chat UI elements
    expect(find.text('Ask anything about The Left Hand of Darkness'), findsOneWidget);
    expect(find.text('Summarize the major themes'), findsOneWidget);
    expect(find.byIcon(Icons.arrow_upward_rounded), findsOneWidget);
  });

  testWidgets('BookDetailView Chat tab sends message and renders response with citation', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
          bookChatProvider('book-42').overrideWith(() => _MockBookChatNotifier('book-42')),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Open Chat Tab
    await tester.tap(find.text('Chat with Book'));
    await tester.pumpAndSettle();

    // Tap prompt chip
    await tester.tap(find.text('Summarize the major themes'));
    await tester.pumpAndSettle();

    // Verify Assistant response & citation rendered
    expect(find.text('Summarize the major themes'), findsWidgets);
    expect(find.text('Themes include duality, gender, and diplomacy.'), findsOneWidget);
    expect(find.text('Ch. 1: A Parade in Erhenrang'), findsOneWidget);
  });

  testWidgets('BookDetailView renders HTML synopsis without raw markup tags', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final bookWithHtmlSynopsis = testBook.copyWith(
      synopsis: '<p><b>Important hook:</b><br>Story details in <i>italics</i>.</p>',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(bookWithHtmlSynopsis)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Synopsis'), findsOneWidget);
    expect(find.textContaining('Important hook:'), findsOneWidget);
    expect(find.textContaining('Story details in italics.'), findsOneWidget);
    expect(find.textContaining('<p>'), findsNothing);
    expect(find.textContaining('<b>'), findsNothing);
    expect(find.textContaining('<i>'), findsNothing);
    expect(find.textContaining('<br>'), findsNothing);
  });

  testWidgets('BookDetailView Chat tab renders rich markdown in chat messages', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final markdownMsgNotifier = _MockMarkdownChatNotifier('book-42');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
          bookChatProvider('book-42').overrideWith(() => markdownMsgNotifier),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Open Chat Tab
    await tester.tap(find.text('Chat with Book'));
    await tester.pumpAndSettle();

    // Tap prompt chip to trigger markdown response
    await tester.tap(find.text('Summarize the major themes'));
    await tester.pumpAndSettle();

    // Verify MarkdownBody is rendered
    expect(find.byType(MarkdownBody), findsWidgets);

    // Verify parsed markdown contents render
    expect(find.text('The Intelligence Explosion'), findsOneWidget);
    expect(find.textContaining('The transition from human-level AI'), findsOneWidget);

    // Verify raw markdown tags are not present in rendered text
    expect(find.textContaining('### 1. The Intelligence Explosion'), findsNothing);
    expect(find.textContaining('**The Theme:**'), findsNothing);
  });
}

class _MockBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  _MockBookDetailNotifier(this.mockBook) : super('book-42');

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }
}

class _MockBookChatNotifier extends BookChatNotifier {
  _MockBookChatNotifier(super.bookId);

  @override
  BookChatState build() {
    return const BookChatState();
  }

  @override
  Future<void> sendMessage(String text) async {
    final userMsg = BookChatMessage(role: 'user', content: text);
    final assistantMsg = BookChatMessage(
      role: 'assistant',
      content: 'Themes include duality, gender, and diplomacy.',
      citations: const [
        BookCitation(
          chapterId: 'spine-1',
          chapterIndex: 1,
          chapterTitle: 'A Parade in Erhenrang',
          summary: 'Genly Ai attends festival.',
        ),
      ],
    );
    state = state.copyWith(messages: [userMsg, assistantMsg]);
  }
}

class _MockMarkdownChatNotifier extends BookChatNotifier {
  _MockMarkdownChatNotifier(super.bookId);

  @override
  BookChatState build() {
    return const BookChatState();
  }

  @override
  Future<void> sendMessage(String text) async {
    final userMsg = BookChatMessage(role: 'user', content: text);
    final assistantMsg = BookChatMessage(
      role: 'assistant',
      content: '''
The book *Superintelligence* addresses critical challenges:

### The Intelligence Explosion
* **The Theme:** The transition from human-level AI to superhuman AI could be rapid.
''',
    );
    state = state.copyWith(messages: [userMsg, assistantMsg]);
  }
}

