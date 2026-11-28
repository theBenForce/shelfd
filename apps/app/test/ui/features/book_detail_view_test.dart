import 'package:flutter/material.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/book_chat.dart';
import 'package:shelf/data/models/bookmark.dart';
import 'package:shelf/data/models/highlight.dart';
import 'package:shelf/data/models/plugin.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/shared_layout.dart';
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

    // Verify title, author, series, and TactileBookCover
    expect(find.byType(TactileBookCover), findsOneWidget);
    expect(find.text('The Left Hand of Darkness'), findsWidgets);
    expect(find.text('Ursula K. Le Guin'), findsOneWidget);
    expect(find.text('Hainish Cycle #4'), findsOneWidget);

    // Verify reading progress & CTA
    expect(find.text('Reading Progress'), findsOneWidget);
    expect(find.text('65%'), findsOneWidget);
    expect(find.text('2 chapters'), findsOneWidget);
    expect(find.text('Resume Reading'), findsOneWidget);
    expect(find.text('Share'), findsOneWidget);
    expect(find.byTooltip('Share'), findsWidgets);

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

  testWidgets('BookDetailView does not display manual Add Highlight button on overview', (tester) async {
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

    // Verify Add Highlight button is absent from AppBar
    expect(find.byIcon(Icons.note_add_outlined), findsNothing);
    // Bookmark button remains
    expect(find.byIcon(Icons.bookmark_add_outlined), findsOneWidget);
  });

  testWidgets('BookDetailView opens Edit Book Metadata modal and displays form fields', (tester) async {
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

    // Tap Edit Metadata button
    final editBtn = find.text('Edit Metadata');
    expect(editBtn, findsWidgets);
    await tester.tap(editBtn.first);
    await tester.pumpAndSettle();

    // Verify modal elements are displayed
    expect(find.text('Edit Book Metadata'), findsOneWidget);
    expect(find.text('Save & Sync EPUB'), findsOneWidget);
    expect(find.byKey(const Key('edit_book_title_field')), findsOneWidget);
    expect(find.byKey(const Key('edit_book_authors_field')), findsOneWidget);
    expect(find.byKey(const Key('edit_book_series_field')), findsOneWidget);
    expect(find.byKey(const Key('edit_book_sequence_field')), findsOneWidget);
  });

  testWidgets('BookDetailView correctly renders multi-file audiobook with tracks and aggregate format', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final hobbitMultiAudiobook = Book(
      id: 'the-hobbit-id',
      title: 'The Hobbit',
      synopsis: 'In a hole in the ground there lived a hobbit.',
      authors: const [Author(id: 'auth-tolkien', name: 'J.R.R. Tolkien')],
      bookType: 'audiobook',
      durationSeconds: 42300.0, // 11h 45m
      audioChapters: const [
        AudioChapter(id: 'c-1', bookId: 'the-hobbit-id', chapterIndex: 1, title: '1. An Unexpected Party', startOffsetSec: 0, durationSec: 8400),
        AudioChapter(id: 'c-2', bookId: 'the-hobbit-id', chapterIndex: 2, title: '2. Roast Mutton', startOffsetSec: 8400, durationSec: 4560),
      ],
      files: const [
        BookFile(id: 'f-1', bookId: 'the-hobbit-id', fileType: 'audiobook', filePath: '01 - An Unexpected Party.m4a', fileSizeBytes: 50000000, durationSeconds: 8400),
        BookFile(id: 'f-2', bookId: 'the-hobbit-id', fileType: 'audiobook', filePath: '02 - Roast Mutton.m4a', fileSizeBytes: 30000000, durationSeconds: 4560),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('the-hobbit-id').overrideWith(() => _MockBookDetailNotifier(hobbitMultiAudiobook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'the-hobbit-id'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Verify Title is The Hobbit, not track title
    expect(find.text('The Hobbit'), findsWidgets);
    expect(find.text('J.R.R. Tolkien'), findsOneWidget);

    // Verify Chapters shows 2 chapters
    expect(find.text('2 chapters'), findsOneWidget);

    // Verify Audio Length displays aggregated time
    expect(find.text('11h 45m'), findsOneWidget);

    // Verify Format displays Multi-file Audiobook
    expect(find.text('Multi-file Audiobook (2 files)'), findsOneWidget);

    // Verify Table of Contents displays tracks
    expect(find.text('2 tracks'), findsOneWidget);
    expect(find.text('1. An Unexpected Party'), findsWidgets);
    expect(find.text('2. Roast Mutton'), findsWidgets);
  });

  testWidgets('BookDetailView shows only named chapters in TOC for fixed layout books', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    const fixedLayoutBook = Book(
      id: 'toy-story-4',
      title: 'Toy Story 4',
      layout: 'pre-paginated',
      spine: [
        SpineItem(id: 's1', bookId: 'toy-story-4', chapterIndex: 1, title: 'Cover'),
        SpineItem(id: 's2', bookId: 'toy-story-4', chapterIndex: 2, title: 'Page 2'),
        SpineItem(id: 's3', bookId: 'toy-story-4', chapterIndex: 3, title: 'Chapter 1: The Antique Shop'),
        SpineItem(id: 's4', bookId: 'toy-story-4', chapterIndex: 4, title: 'Page 4'),
        SpineItem(id: 's5', bookId: 'toy-story-4', chapterIndex: 5, title: 'Chapter 2: Carnival Adventure'),
        SpineItem(id: 's6', bookId: 'toy-story-4', chapterIndex: 6, title: 'Page 6'),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('toy-story-4').overrideWith(() => _MockBookDetailNotifier(fixedLayoutBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'toy-story-4'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Verify Chapter count header displays named chapters and total pages
    expect(find.text('3 chapters (6 pages)'), findsWidgets);

    // Verify Table of Contents displays named chapters and not generic pages
    expect(find.text('Cover'), findsOneWidget);
    expect(find.text('Chapter 1: The Antique Shop'), findsOneWidget);
    expect(find.text('Chapter 2: Carnival Adventure'), findsOneWidget);
    expect(find.text('Page 2'), findsNothing);
    expect(find.text('Page 4'), findsNothing);
    expect(find.text('Page 6'), findsNothing);

    // Verify page badges
    expect(find.text('Page 3'), findsOneWidget);
    expect(find.text('Page 5'), findsOneWidget);
  });

  testWidgets('BookDetailView Download EPUB dropdown opens and shows dynamic plugin action options', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockRepo = _MockBookRepository(prefs);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(mockRepo),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
          pluginsProvider.overrideWith(_MockPluginsNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    // Open the Hero Card Download EPUB dropdown menu
    await tester.tap(find.byKey(const Key('download_hero_menu_button')));
    await tester.pumpAndSettle();

    // Verify all dropdown options are rendered
    expect(find.byKey(const Key('menu_item_download_epub')), findsOneWidget);
    expect(
      find.byKey(const Key('menu_item_plugin_notebooklm_send_to_notebooklm')),
      findsOneWidget,
    );
    expect(
      find.byKey(const Key('menu_item_plugin_yoto_send_to_yoto')),
      findsOneWidget,
    );
    expect(find.byKey(const Key('menu_item_download_markdown')), findsOneWidget);
    expect(find.text('Send to NotebookLM'), findsOneWidget);
    expect(find.text('Send to Yoto'), findsOneWidget);
    expect(find.text('Download Markdown (.md)'), findsOneWidget);

    // Since testBook has no audio files, "Send to Yoto" must be disabled
    final sendYotoItem = tester.widget<PopupMenuItem<String>>(
      find.byKey(const Key('menu_item_plugin_yoto_send_to_yoto')),
    );
    expect(sendYotoItem.enabled, isFalse);

    // Select "Download Markdown (.md)"
    await tester.tap(find.byKey(const Key('menu_item_download_markdown')));
    await tester.pumpAndSettle();

    expect(mockRepo.downloadMarkdownCalledWith, 'book-42');
  });

  testWidgets('BookDetailView enables Send to Yoto plugin menu item when audio files are available', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final audiobook = testBook.copyWith(
      bookType: 'audiobook',
      durationSeconds: 3600.0,
      files: const [
        BookFile(
          id: 'af-1',
          bookId: 'book-42',
          fileType: 'audiobook',
          filePath: 'chapter1.m4b',
          durationSeconds: 3600.0,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(audiobook)),
          pluginsProvider.overrideWith(_MockPluginsNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.byKey(const Key('download_hero_menu_button')));
    await tester.pumpAndSettle();

    final sendYotoItem = tester.widget<PopupMenuItem<String>>(
      find.byKey(const Key('menu_item_plugin_yoto_send_to_yoto')),
    );
    expect(sendYotoItem.enabled, isTrue);

    await tester.tap(find.byKey(const Key('menu_item_plugin_yoto_send_to_yoto')));
    await tester.pump();

    expect(
      find.byKey(const Key('plugin_action_dialog_yoto_send_to_yoto')),
      findsOneWidget,
    );
  });

  testWidgets('BookDetailView AppBar Download EPUB dropdown triggers EPUB and Markdown downloads', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockRepo = _MockBookRepository(prefs);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(mockRepo),
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

    // Open the AppBar Share dropdown menu
    await tester.tap(find.byKey(const Key('download_appbar_menu_button')));
    await tester.pumpAndSettle();

    // Select "Download EPUB"
    await tester.tap(find.byKey(const Key('menu_item_download_epub')));
    await tester.pumpAndSettle();

    expect(mockRepo.downloadEpubCalledWith, 'book-42');

    // Open AppBar dropdown again and select "Download Markdown (.md)"
    await tester.tap(find.byKey(const Key('download_appbar_menu_button')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('menu_item_download_markdown')));
    await tester.pumpAndSettle();

    expect(mockRepo.downloadMarkdownCalledWith, 'book-42');
  });

  testWidgets('BookDetailView file item menu supports Download and Delete with confirmation', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockRepo = _MockBookRepository(prefs);

    final bookWithFiles = testBook.copyWith(
      bookType: 'dual',
      durationSeconds: 3600.0,
      files: const [
        BookFile(
          id: 'file-cover-1',
          bookId: 'book-42',
          fileType: 'cover',
          filePath: 'Stephen Fry/Odyssey/cover.jpg',
          fileSizeBytes: 47200,
          mimeType: 'image/jpeg',
        ),
        BookFile(
          id: 'file-audio-1',
          bookId: 'book-42',
          fileType: 'audiobook',
          filePath: 'Stephen Fry/Odyssey/Odyssey.m4b',
          durationSeconds: 3600.0,
          bitrateKbps: 64,
          mimeType: 'audio/mp4',
        ),
      ],
    );

    final notifier = _MockBookDetailNotifier(bookWithFiles, mockRepo: mockRepo);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(mockRepo),
          bookDetailProvider('book-42').overrideWith(() => notifier),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-42'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Stephen Fry/Odyssey/cover.jpg'), findsOneWidget);
    expect(find.text('Stephen Fry/Odyssey/Odyssey.m4b'), findsOneWidget);

    // Open overflow menu for first file item (cover.jpg)
    final coverMenuFinder = find.byKey(const Key('file_item_menu_button_0'));
    await tester.ensureVisible(coverMenuFinder);
    await tester.tap(coverMenuFinder);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('file_menu_download_0')), findsOneWidget);
    expect(find.byKey(const Key('file_menu_delete_0')), findsOneWidget);
    expect(find.text('Download'), findsOneWidget);
    expect(find.text('Delete'), findsOneWidget);

    // Select Download
    await tester.tap(find.byKey(const Key('file_menu_download_0')));
    await tester.pumpAndSettle();

    expect(mockRepo.downloadBookFileCalledWith, ('book-42', 'file-cover-1'));

    // Open overflow menu again and select Delete
    await tester.tap(coverMenuFinder);
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('file_menu_delete_0')));
    await tester.pumpAndSettle();

    // Verify confirmation dialog appears
    expect(find.text('Delete File'), findsOneWidget);
    expect(find.byKey(const Key('confirm_delete_file_button')), findsOneWidget);

    // Confirm Delete
    await tester.tap(find.byKey(const Key('confirm_delete_file_button')));
    await tester.pumpAndSettle();

    expect(mockRepo.deleteBookFileCalledWith, ('book-42', 'file-cover-1'));
    expect(find.text('Stephen Fry/Odyssey/cover.jpg'), findsNothing);
    expect(find.text('Stephen Fry/Odyssey/Odyssey.m4b'), findsOneWidget);
  });

  testWidgets('BookDetailView tapping Bookmark card navigates to reader view', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    String? navigatedRoute;

    final router = GoRouter(
      initialLocation: '/books/book-42',
      routes: [
        GoRoute(
          path: '/books/:id',
          builder: (context, state) => BookDetailView(bookId: state.pathParameters['id']!),
        ),
        GoRoute(
          path: '/books/:id/read',
          builder: (context, state) {
            navigatedRoute = state.uri.toString();
            return const Scaffold(body: Text('Reader Target'));
          },
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
        ],
        child: MaterialApp.router(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          routerConfig: router,
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.text('Highlights & Bookmarks'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Mid-Parade passage'));
    await tester.pumpAndSettle();

    expect(navigatedRoute, '/books/book-42/read');
    expect(find.text('Reader Target'), findsOneWidget);
  });

  testWidgets('BookDetailView and EditBookMetadataModal adapt to ReadingThemeMode.dark and ReadingThemeMode.sepia',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    for (final mode in [ReadingThemeMode.dark, ReadingThemeMode.sepia]) {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            bookDetailProvider('book-42').overrideWith(() => _MockBookDetailNotifier(testBook)),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(mode),
            home: const BookDetailView(bookId: 'book-42'),
          ),
        ),
      );

      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.text('The Left Hand of Darkness'), findsWidgets);
      expect(find.text('65%'), findsOneWidget);

      // Open Edit Book Metadata modal in this theme
      await tester.tap(find.text('Edit Metadata').first);
      await tester.pumpAndSettle();

      expect(find.text('Edit Book Metadata'), findsOneWidget);
      expect(find.text('Save & Sync EPUB'), findsOneWidget);

      // Close modal
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
    }
  });
}

class _MockBookRepository extends BookRepository {
  String? downloadEpubCalledWith;
  String? downloadMarkdownCalledWith;
  (String, String)? downloadBookFileCalledWith;
  (String, String)? deleteBookFileCalledWith;

  _MockBookRepository(SharedPreferences prefs)
      : super(
          apiService: ApiService(baseUrl: 'http://localhost:8080'),
          storageService: StorageService(prefs),
        );

  @override
  Future<List<int>> downloadBookEpub(String bookId) async {
    downloadEpubCalledWith = bookId;
    return [0x50, 0x4B, 0x03, 0x04];
  }

  @override
  Future<List<int>> downloadBookFile(String bookId, String fileId) async {
    downloadBookFileCalledWith = (bookId, fileId);
    return [0xFF, 0xD8, 0xFF, 0xE0];
  }

  @override
  Future<void> deleteBookFile(String bookId, String fileId) async {
    deleteBookFileCalledWith = (bookId, fileId);
  }

  @override
  Future<List<int>> downloadBookMarkdown(String bookId) async {
    downloadMarkdownCalledWith = bookId;
    return '# The Left Hand of Darkness\n'.codeUnits;
  }
}

class _MockPluginsNotifier extends PluginsNotifier {
  @override
  PluginsState build() {
    return const PluginsState(
      plugins: [
        PluginSummary(
          name: 'notebooklm',
          displayName: 'NotebookLM',
          version: '0.1.0',
          enabled: true,
          status: 'idle',
          capabilities: ['actions'],
          actions: [
            PluginActionDef(
              name: 'send_to_notebooklm',
              title: 'Send to NotebookLM',
              icon: 'auto_awesome',
              targets: ['book'],
              mediaTypes: ['ebook'],
              mode: 'immediate',
              execution: 'sync',
            ),
          ],
        ),
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

class _MockBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  final _MockBookRepository? mockRepo;
  _MockBookDetailNotifier(this.mockBook, {this.mockRepo}) : super('book-42');

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }

  @override
  Future<void> deleteBookFile(String fileId) async {
    if (mockRepo != null) {
      await mockRepo!.deleteBookFile(bookId, fileId);
    }
    final current = state.book;
    if (current != null) {
      state = state.copyWith(
        book: current.copyWith(
          files: current.files.where((f) => f.id != fileId).toList(),
        ),
      );
    }
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

