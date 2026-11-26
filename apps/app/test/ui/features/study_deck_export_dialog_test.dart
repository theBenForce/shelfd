import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/book_detail/book_detail_view.dart';
import 'package:shelf/ui/features/book_detail/study_deck_export_dialog.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const testBook = Book(
    id: 'book-dune',
    title: 'Dune',
    authors: [Author(id: 'auth-1', name: 'Frank Herbert')],
    spine: [
      SpineItem(
        id: 'spine-1',
        bookId: 'book-dune',
        chapterIndex: 1,
        title: 'Chapter 1: Caladan',
      ),
      SpineItem(
        id: 'spine-2',
        bookId: 'book-dune',
        chapterIndex: 2,
        title: 'Chapter 2: Gom Jabbar',
      ),
      SpineItem(
        id: 'spine-3',
        bookId: 'book-dune',
        chapterIndex: 3,
        title: 'Chapter 3: Arrakis',
      ),
    ],
  );

  testWidgets(
    'StudyDeckExportDialog supports format selection, chapter toggling, and empty-selection guard',
    (tester) async {
      StudyDeckExportSelection? result;

      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Builder(
            builder: (context) => Scaffold(
              body: ElevatedButton(
                onPressed: () async {
                  result = await showStudyDeckExportDialog(context, testBook);
                },
                child: const Text('Open Dialog'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open Dialog'));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('study_deck_export_dialog')), findsOneWidget);
      expect(find.text('Anki .apkg'), findsOneWidget);
      expect(find.text('Anki .csv'), findsOneWidget);
      expect(find.text('Markdown Vault .zip'), findsOneWidget);
      expect(find.text('Chapters (3/3)'), findsOneWidget);

      await tester.tap(find.byKey(const Key('study_deck_clear_all_button')));
      await tester.pumpAndSettle();

      expect(find.text('Chapters (0/3)'), findsOneWidget);
      final disabledExportButton = tester.widget<FilledButton>(
        find.byKey(const Key('study_deck_export_button')),
      );
      expect(disabledExportButton.onPressed, isNull);

      await tester.tap(find.byKey(const Key('study_deck_select_all_button')));
      await tester.pumpAndSettle();
      expect(find.text('Chapters (3/3)'), findsOneWidget);

      final chapter2 = find.byKey(const Key('study_deck_chapter_2'));
      await tester.ensureVisible(chapter2);
      await tester.pumpAndSettle();
      await tester.tap(chapter2);
      await tester.pumpAndSettle();
      expect(find.text('Chapters (2/3)'), findsOneWidget);

      final markdownFormat = find.byKey(const Key('study_deck_format_markdown'));
      await tester.ensureVisible(markdownFormat);
      await tester.pumpAndSettle();
      await tester.tap(markdownFormat);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('study_deck_export_button')));
      await tester.pumpAndSettle();

      expect(result, isNotNull);
      expect(result!.format, StudyDeckFormat.markdown);
      expect(result!.chapterIndices, [1, 3]);
    },
  );

  testWidgets('StudyDeckExportDialog cancel dismisses cleanly with null result', (tester) async {
    StudyDeckExportSelection? result = const StudyDeckExportSelection(
      format: StudyDeckFormat.apkg,
      chapterIndices: [1],
    );

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: Builder(
          builder: (context) => Scaffold(
            body: ElevatedButton(
              onPressed: () async {
                result = await showStudyDeckExportDialog(context, testBook);
              },
              child: const Text('Open Dialog'),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open Dialog'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('study_deck_cancel_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('study_deck_export_dialog')), findsNothing);
    expect(result, isNull);
  });

  testWidgets('StudyDeckExportDialog fits narrow mobile viewport without overflow', (tester) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.buildTheme(ReadingThemeMode.bone),
        home: const Scaffold(
          body: StudyDeckExportDialog(book: testBook),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byKey(const Key('study_deck_export_dialog')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('BookDetailView export menu opens StudyDeckExportDialog and exports deck', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockRepo = _StudyDeckMockBookRepository(prefs);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(mockRepo),
          bookDetailProvider('book-dune').overrideWith(() => _MockBookDetailNotifier(testBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-dune'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.byKey(const Key('download_hero_menu_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('menu_item_export_study_deck')), findsOneWidget);
    expect(find.text('Export Study Deck'), findsOneWidget);

    await tester.tap(find.byKey(const Key('menu_item_export_study_deck')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('study_deck_export_dialog')), findsOneWidget);

    await tester.tap(find.byKey(const Key('study_deck_format_csv')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('study_deck_chapter_3')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('study_deck_export_button')));
    await tester.pumpAndSettle();

    expect(mockRepo.studyDeckCalls, 1);
    expect(mockRepo.lastBookId, 'book-dune');
    expect(mockRepo.lastFormat, 'csv');
    expect(mockRepo.lastChapterIndices, [1, 2]);
    expect(find.text('Exported Dune-study-deck.csv'), findsOneWidget);
  });

  testWidgets('BookDetailView shows error snackbar when study deck export fails', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockRepo = _StudyDeckMockBookRepository(
      prefs,
      shouldFail: true,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(mockRepo),
          bookDetailProvider('book-dune').overrideWith(() => _MockBookDetailNotifier(testBook)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const BookDetailView(bookId: 'book-dune'),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.byKey(const Key('download_hero_menu_button')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('menu_item_export_study_deck')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('study_deck_export_button')));
    await tester.pumpAndSettle();

    expect(mockRepo.studyDeckCalls, 1);
    expect(find.textContaining('Failed to export study deck'), findsOneWidget);
  });
}

class _StudyDeckMockBookRepository extends BookRepository {
  final bool shouldFail;
  int studyDeckCalls = 0;
  String? lastBookId;
  String? lastFormat;
  List<int>? lastChapterIndices;

  _StudyDeckMockBookRepository(
    SharedPreferences prefs, {
    this.shouldFail = false,
  }) : super(
          apiService: ApiService(baseUrl: 'http://localhost:8080'),
          storageService: StorageService(prefs),
        );

  @override
  Future<List<int>> downloadStudyDeck(
    String bookId, {
    required String format,
    required List<int> chapterIndices,
  }) async {
    studyDeckCalls += 1;
    lastBookId = bookId;
    lastFormat = format;
    lastChapterIndices = List<int>.from(chapterIndices);
    if (shouldFail) {
      throw const ApiException(500, 'LLM unavailable');
    }
    return 'front,back\nGom Jabbar,High-handed enemy\n'.codeUnits;
  }
}

class _MockBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;

  _MockBookDetailNotifier(this.mockBook) : super(mockBook.id);

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }
}
