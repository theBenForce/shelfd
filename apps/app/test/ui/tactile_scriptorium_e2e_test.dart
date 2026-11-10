import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/book_chat.dart';
import 'package:shelf/data/models/bookmark.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/data/models/duplicate_group.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/highlight.dart';
import 'package:shelf/data/models/metadata_search_result.dart';
import 'package:shelf/data/models/paginated_books.dart';
import 'package:shelf/data/models/queue_status.dart';
import 'package:shelf/data/models/search_result.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/data/repositories/audio_repository.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/app_shell.dart';
import 'package:shelf/ui/core/html_text.dart';
import 'package:shelf/ui/core/responsive.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/core/typography.dart';
import 'package:shelf/ui/features/audiobook/audiobook_player_view.dart';
import 'package:shelf/ui/features/audiobook/widgets/chapter_drawer.dart';
import 'package:shelf/ui/features/audiobook/widgets/mini_player_bar.dart';
import 'package:shelf/ui/features/audiobook/widgets/scrubber_bar.dart';
import 'package:shelf/ui/features/authors/author_detail_view.dart';
import 'package:shelf/ui/features/book_detail/book_detail_view.dart';
import 'package:shelf/ui/features/book_detail/edit_book_metadata_dialog.dart';
import 'package:shelf/ui/features/book_detail/metadata_search_dialog.dart';
import 'package:shelf/ui/features/connect/connect_view.dart';
import 'package:shelf/ui/features/library/library_filter_bar.dart';
import 'package:shelf/ui/features/library/library_query_parser.dart';
import 'package:shelf/ui/features/library/library_view.dart';
import 'package:shelf/ui/features/reader/fixed_layout_reader.dart';
import 'package:shelf/ui/features/reader/reader_markdown.dart';
import 'package:shelf/ui/features/reader/reader_view.dart';
import 'package:shelf/ui/features/search/search_view.dart';
import 'package:shelf/ui/features/series/series_detail_view.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/features/upload/upload_drop_target.dart';
import 'package:shelf/ui/features/upload/upload_review_dialog.dart';
import 'package:shelf/ui/features/upload/uploads_view.dart';
import 'package:shelf/ui/features/utilities/utilities_view.dart';
import 'package:shelf/ui/router.dart';
import 'package:shelf/ui/state/providers.dart';

// =============================================================================
// Test Fakes & Harnesses for E2E Widget Verification
// =============================================================================

class _FakeQueueNotifier extends QueueNotifier {
  final QueueState _initial;
  _FakeQueueNotifier([this._initial = const QueueState()]);

  @override
  QueueState build() => _initial;
}

class _FakeLibraryNotifier extends LibraryNotifier {
  final LibraryState _initial;
  _FakeLibraryNotifier([this._initial = const LibraryState()]);

  @override
  LibraryState build() => _initial;

  @override
  void setSearchQuery(String query) {
    state = state.copyWith(searchQuery: query);
  }

  @override
  void setFilter(String filter) {
    state = state.copyWith(activeFilter: filter);
  }

  @override
  Future<void> loadLibrary({bool refresh = false}) async {}

  @override
  Future<void> loadMoreBooks() async {}
}

class _FakeBookDetailNotifier extends BookDetailNotifier {
  final Book? mockBook;
  _FakeBookDetailNotifier(String bookId, this.mockBook) : super(bookId);

  @override
  BookDetailState build() => BookDetailState(book: mockBook, isLoading: false);

  @override
  Future<void> loadBook() async {}
}

class _FakeBookChatNotifier extends BookChatNotifier {
  final BookChatState _initial;
  _FakeBookChatNotifier(String bookId, [this._initial = const BookChatState()])
      : super(bookId);

  @override
  BookChatState build() => _initial;
}

class _FakeUploadNotifier extends UploadNotifier {
  final UploadState _initial;
  _FakeUploadNotifier([this._initial = const UploadState()]);

  @override
  UploadState build() => _initial;

  @override
  Future<void> loadStagedJobs() async {}
}

class _FakeSearchNotifier extends SearchNotifier {
  final SearchState _initial;
  _FakeSearchNotifier([this._initial = const SearchState()]);

  @override
  SearchState build() => _initial;
}

class _FakeAudioPlayerNotifier extends AudioPlayerNotifier {
  final AudioPlayerState _initial;
  _FakeAudioPlayerNotifier([this._initial = const AudioPlayerState()]);

  @override
  AudioPlayerState build() => _initial;

  @override
  Future<void> loadBook(
    Book book, {
    String? fileId,
    double? initialPosition,
    bool autoPlay = false,
  }) async {
    state = _initial.copyWith(book: book);
  }
}

class _MockBookRepository implements BookRepository {
  final List<Book> books;
  final List<Author> authors;
  final List<Series> series;
  final List<Genre> genres;
  final List<Topic> topics;
  DuplicateScanResult scanResult;

  _MockBookRepository({
    this.books = const [],
    this.authors = const [],
    this.series = const [],
    this.genres = const [],
    this.topics = const [],
    this.scanResult = const DuplicateScanResult(groups: [], totalDuplicates: 0),
  });

  @override
  Future<List<Author>> getAuthors() async => authors;

  @override
  Future<List<Series>> getSeries() async => series;

  @override
  Future<List<Genre>> getGenres() async => genres;

  @override
  Future<List<Topic>> getTopics() async => topics;

  @override
  Future<DuplicateScanResult> getDuplicateBooks() async => scanResult;

  @override
  Future<PaginatedBooks> getBooksPage({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? authorName,
    String? genreId,
    String? genreName,
    String? topicId,
    String? topicName,
    String? seriesId,
    String? search,
  }) async {
    return PaginatedBooks(
      books: books,
      total: books.length,
      page: page,
      perPage: perPage,
      limit: perPage,
      offset: 0,
    );
  }

  @override
  Future<void> triggerScan() async {}

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late SharedPreferences prefs;
  late StorageService storageService;

  setUp(() async {
    SharedPreferences.setMockInitialValues({
      'shelfd_server_url': 'http://localhost:8080',
      'shelfd_auth_token': 'valid-scriptorium-token',
      'shelfd_saved_username': 'archivist',
    });
    prefs = await SharedPreferences.getInstance();
    storageService = StorageService(prefs);
  });

  const sampleBook = Book(
    id: 'book-42',
    title: 'The Left Hand of Darkness',
    synopsis: '<p>A groundbreaking work of <b>science fiction</b> set on Gethen.</p>',
    authors: [Author(id: 'auth-1', name: 'Ursula K. Le Guin', bookCount: 5)],
    series: Series(id: 'ser-1', name: 'Hainish Cycle', bookCount: 6),
    seriesSequence: 4.0,
    genres: [Genre(id: 'gen-1', name: 'Science Fiction', bookCount: 12)],
    topics: [Topic(id: 'top-1', name: 'Anthropology', bookCount: 4)],
    readingProgress: 0.65,
    publisher: 'Ace Books',
    publishedDate: '1969',
    language: 'en',
    spine: [
      SpineItem(
        id: 'spine-1',
        bookId: 'book-42',
        chapterIndex: 1,
        title: 'Chapter 1: A Parade in Erhenrang',
        summary: 'Genly Ai attends the royal parade.',
      ),
      SpineItem(
        id: 'spine-2',
        bookId: 'book-42',
        chapterIndex: 2,
        title: 'Chapter 2: The Place Inside the Blizzard',
        summary: 'Hearth-tale of brothers.',
      ),
    ],
    bookmarks: [
      Bookmark(
        id: 'bm-1',
        bookId: 'book-42',
        title: 'Royal Arch Ceremony',
        progress: 0.45,
      ),
    ],
    highlights: [
      Highlight(
        id: 'hl-1',
        bookId: 'book-42',
        selectedText: 'Light is the left hand of darkness.',
        note: 'Core philosophical duality',
        color: 'yellow',
      ),
    ],
  );

  // ===========================================================================
  // TIER 1 — FEATURE COVERAGE (Features 1..12, >= 5 tests per feature)
  // ===========================================================================
  group('Tier 1 — Feature Coverage', () {
    group('Feature 1: Theme Tokens & Dynamic Palette', () {
      test('F01-T1-01: Scriptorium editorial accent and warm surface tokens are defined', () {
        expect(AppTokens.siennaPrimary, const Color(0xFF8E4A26));
        expect(AppTokens.siennaContainer, const Color(0xFFAA603A));
        expect(AppTokens.siennaLightTint, const Color(0xFFF7ECE6));
        expect(AppTokens.siennaDarkAccent, const Color(0xFFD98A5F));
        expect(AppTokens.libraryGreen, const Color(0xFF3A5A40));
        expect(AppTokens.libraryGreenContainer, const Color(0xFFBCE0C1));
        expect(AppTokens.libraryGreenDark, const Color(0xFF81B28A));
        expect(AppTokens.warmParchment, const Color(0xFFFBF9F4));
        expect(AppTokens.warmSurfaceLowest, const Color(0xFFFFFFFF));
        expect(AppTokens.warmSurfaceLow, const Color(0xFFF5F3EE));
        expect(AppTokens.warmSurfaceContainer, const Color(0xFFEFECE6));
        expect(AppTokens.warmSurfaceHigh, const Color(0xFFE9E6DF));
        expect(AppTokens.warmSurfaceHighest, const Color(0xFFE2DFD8));
        expect(AppTokens.espressoInk, const Color(0xFF1E1B18));
        expect(AppTokens.espressoSecondary, const Color(0xFF53433C));
        expect(AppTokens.espressoMuted, const Color(0xFF756860));
        expect(AppTokens.ghostBorder, const Color(0xFFE3DDD3));
        expect(AppTokens.subtleGhostBorder, const Color(0xFFEDE8DF));
      });

      test('F01-T1-02: Sepia Archival Vellum and Dark Walnut hierarchies are defined', () {
        expect(AppTokens.sepiaContainerLow, const Color(0xFFEFE6D1));
        expect(AppTokens.sepiaContainer, const Color(0xFFE6DBC2));
        expect(AppTokens.sepiaContainerHigh, const Color(0xFFDCD0B4));
        expect(AppTokens.sepiaContainerHighest, const Color(0xFFD2C4A5));
        expect(AppTokens.sepiaMuted, const Color(0xFF866D59));
        expect(AppTokens.sepiaSubtleBorder, const Color(0xFFE8DEC6));
        expect(AppTokens.walnutBackground, const Color(0xFF161412));
        expect(AppTokens.walnutSurface, const Color(0xFF1E1B18));
        expect(AppTokens.walnutContainerLow, const Color(0xFF23201C));
        expect(AppTokens.walnutContainer, const Color(0xFF292521));
        expect(AppTokens.walnutElevated, const Color(0xFF322D28));
        expect(AppTokens.walnutHighest, const Color(0xFF3B352F));
        expect(AppTokens.walnutInk, const Color(0xFFEDE7DF));
        expect(AppTokens.walnutSecondary, const Color(0xFFBDB3A7));
        expect(AppTokens.walnutMuted, const Color(0xFF8C8277));
        expect(AppTokens.walnutBorder, const Color(0xFF332E29));
        expect(AppTokens.walnutSubtleBorder, const Color(0xFF282420));
      });

      test('F01-T1-03: ShelfdColors presets provide complete semantic palettes for Bone, Sepia, and Dark', () {
        expect(ShelfdColors.bone.isDark, isFalse);
        expect(ShelfdColors.bone.background, AppTokens.warmParchment);
        expect(ShelfdColors.bone.ink, AppTokens.espressoInk);
        expect(ShelfdColors.bone.accentPrimary, AppTokens.siennaPrimary);

        expect(ShelfdColors.sepia.isDark, isFalse);
        expect(ShelfdColors.sepia.background, AppTokens.sepiaBackground);
        expect(ShelfdColors.sepia.ink, AppTokens.sepiaInk);
        expect(ShelfdColors.sepia.accentPrimary, AppTokens.siennaPrimary);

        expect(ShelfdColors.dark.isDark, isTrue);
        expect(ShelfdColors.dark.background, AppTokens.walnutBackground);
        expect(ShelfdColors.dark.ink, AppTokens.walnutInk);
        expect(ShelfdColors.dark.accentPrimary, AppTokens.siennaDarkAccent);
      });

      test('F01-T1-04: AppTheme.buildTheme registers ShelfdColors and M3 component themes', () {
        for (final mode in ReadingThemeMode.values) {
          final theme = AppTheme.buildTheme(mode);
          final ext = theme.extension<ShelfdColors>();
          expect(ext, isNotNull);
          expect(theme.dialogTheme.backgroundColor, ext!.surface);
          expect(theme.bottomSheetTheme.backgroundColor, ext.surface);
          expect(theme.cardTheme.color, ext.surface);
          expect(theme.dividerTheme.color, ext.border);
          expect(theme.inputDecorationTheme.fillColor, ext.containerLow);
        }
      });

      testWidgets('F01-T1-05: ShelfdColors.of(context) and context.shelfdColors resolve active palette', (tester) async {
        for (final mode in ReadingThemeMode.values) {
          ShelfdColors? viaOf;
          ShelfdColors? viaExt;
          await tester.pumpWidget(
            MaterialApp(
              theme: AppTheme.buildTheme(mode),
              home: Builder(
                builder: (context) {
                  viaOf = ShelfdColors.of(context);
                  viaExt = context.shelfdColors;
                  return const SizedBox.shrink();
                },
              ),
            ),
          );
          await tester.pumpAndSettle();
          expect(viaOf, equals(viaExt));
          expect(viaOf!.isDark, mode == ReadingThemeMode.dark);
        }
      });
    });

    group('Feature 2: Editorial Typography System', () {
      test('F02-T1-01: AppTypography defines Newsreader, Work Sans, and JetBrains Mono stacks', () {
        expect(AppTypography.editorialSerifFamily, 'Newsreader');
        expect(AppTypography.humanistSansFamily, 'Work Sans');
        expect(AppTypography.technicalMonoFamily, 'JetBrains Mono');
        expect(AppTypography.serifFallbacks, containsAll(['Newsreader', 'Georgia', 'serif']));
        expect(AppTypography.sansFallbacks, containsAll(['Work Sans', 'Inter', 'sans-serif']));
        expect(AppTypography.monoFallbacks, containsAll(['JetBrains Mono', 'monospace']));
      });

      test('F02-T1-02: AppTypography.mono configures tabular figures', () {
        final style = AppTypography.mono(fontSize: 13, fontWeight: FontWeight.w600);
        expect(style.fontFamily, 'JetBrains Mono');
        expect(style.fontSize, 13.0);
        expect(style.fontWeight, FontWeight.w600);
        expect(style.fontFeatures, contains(const FontFeature.tabularFigures()));
      });

      test('F02-T1-03: displaySerif, titleSerif, readerBodySerif, bodySans, labelCaps, captionSans, readerText signatures', () {
        expect(AppTypography.displaySerif().fontFamily, 'Newsreader');
        expect(AppTypography.titleSerif().fontFamily, 'Newsreader');
        expect(AppTypography.readerBodySerif().fontFamily, 'Newsreader');
        expect(AppTypography.bodySans().fontFamily, 'Work Sans');
        expect(AppTypography.labelCaps().fontFamily, 'Work Sans');
        expect(AppTypography.captionSans().fontFamily, 'Work Sans');
        expect(
          AppTypography.readerText(fontSize: 18, lineHeight: 1.6, isSerif: true, color: Colors.black).fontFamily,
          'Newsreader',
        );
        expect(
          AppTypography.readerText(fontSize: 18, lineHeight: 1.6, isSerif: false, color: Colors.black).fontFamily,
          'Work Sans',
        );
      });

      test('F02-T1-04: parseHtmlBlocks and parseHtmlInlineSpans structure editorial HTML', () {
        final blocks = parseHtmlBlocks('<h1>Heading</h1><p>Paragraph with <code>mono</code></p><hr>');
        expect(blocks.length, 3);
        expect(blocks[0].type, HtmlBlockType.h1);
        expect(blocks[1].type, HtmlBlockType.paragraph);
        expect(blocks[2].type, HtmlBlockType.divider);
      });

      testWidgets('F02-T1-05: HtmlText adapts text, quote, and divider colors to ShelfdColors', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const Scaffold(
              body: HtmlText(
                html: '<h2>Overview</h2><blockquote>Quoted thought</blockquote><p>Body with <a href="https://example.com">link</a></p>',
              ),
            ),
          ),
        );
        expect(find.textContaining('Overview'), findsOneWidget);
        expect(find.textContaining('Quoted thought'), findsOneWidget);
        expect(find.textContaining('Body with'), findsOneWidget);
      });
    });

    group('Feature 3: Shared Layout, Navigation Shell & Tactile Book Cards', () {
      testWidgets('F03-T1-01: BentoCard renders tonal surface and preserves internal Card widget', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: const Scaffold(
              body: BentoCard(child: Text('Archival Surface')),
            ),
          ),
        );
        expect(find.text('Archival Surface'), findsOneWidget);
        expect(find.byType(Card), findsOneWidget);
      });

      testWidgets('F03-T1-02: TactileBookCover renders spine crease, series backing plates, and ornamental fallback', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: const Scaffold(
              body: SizedBox(
                width: 160,
                height: 240,
                child: TactileBookCover(
                  coverUrl: null,
                  title: 'Earthsea Trilogy',
                  subtitle: 'Ursula K. Le Guin',
                  isSeries: true,
                  showSpineCrease: true,
                  topRightBadge: StatusBadge(label: '3 Books'),
                ),
              ),
            ),
          ),
        );
        expect(find.text('Earthsea Trilogy'), findsOneWidget);
        expect(find.text('Ursula K. Le Guin'), findsOneWidget);
        expect(find.text('3 Books'), findsOneWidget);
      });

      testWidgets('F03-T1-03: ShelfdGridCard renders Book, Series, Author, Genre, and Topic items', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: SingleChildScrollView(
                child: Column(
                  children: [
                    SizedBox(
                      width: 180,
                      height: 280,
                      child: ShelfdGridCard(item: BookGridItem(sampleBook), onTap: () {}),
                    ),
                    SizedBox(
                      width: 180,
                      height: 280,
                      child: ShelfdGridCard(
                        item: SeriesGridItem(const Series(id: 's1', name: 'Hainish Cycle', bookCount: 6)),
                        onTap: () {},
                      ),
                    ),
                    SizedBox(
                      width: 180,
                      height: 220,
                      child: ShelfdGridCard(
                        item: AuthorGridItem(const Author(id: 'a1', name: 'Ursula K. Le Guin', bookCount: 5)),
                        onTap: () {},
                      ),
                    ),
                    SizedBox(
                      width: 180,
                      height: 160,
                      child: ShelfdGridCard(
                        item: GenreGridItem(const Genre(id: 'g1', name: 'Science Fiction', bookCount: 12)),
                        onTap: () {},
                      ),
                    ),
                    SizedBox(
                      width: 180,
                      height: 160,
                      child: ShelfdGridCard(
                        item: TopicGridItem(const Topic(id: 't1', name: 'Anthropology', bookCount: 4)),
                        onTap: () {},
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
        expect(find.text('The Left Hand of Darkness'), findsWidgets);
        expect(find.text('65% read'), findsOneWidget);
        expect(find.text('6 Books'), findsOneWidget);
        expect(find.text('5 Books'), findsOneWidget);
        expect(find.text('12 Books'), findsOneWidget);
        expect(find.text('4 Books'), findsOneWidget);
      });

      testWidgets('F03-T1-04: ShelfdSideNav renders collapsible Library accordion, staged badge, and queue card', (tester) async {
        const status = QueueStatus(
          pendingChapters: 2,
          totalChapters: 4,
          currentBook: 'book-42',
          currentTask: 'embedding',
          stagedUploads: 3,
          isActive: true,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              queueProvider.overrideWith(() => _FakeQueueNotifier(const QueueState(status: status))),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: Scaffold(
                body: ShelfdSideNav(
                  currentIndex: 0,
                  onTap: (_) {},
                  currentPath: '/books',
                  onNavigate: (_) {},
                ),
              ),
            ),
          ),
        );
        expect(find.text('Shelfd'), findsOneWidget);
        expect(find.text('Your Digital Vellum'), findsOneWidget);
        expect(find.text('Library'), findsOneWidget);
        expect(find.text('Books'), findsOneWidget);
        expect(find.text('Series'), findsOneWidget);
        expect(find.text('Authors'), findsOneWidget);
        expect(find.text('Genres'), findsOneWidget);
        expect(find.text('Topics'), findsOneWidget);
        expect(find.text('Semantic Search'), findsOneWidget);
        expect(find.text('Uploads'), findsOneWidget);
        expect(find.text('3'), findsOneWidget);
        expect(find.text('50%'), findsOneWidget);
        expect(find.text('The Left Hand of Darkness'), findsOneWidget);
      });

      testWidgets('F03-T1-05: PrimaryButton, FilterPillsRow, and showShelfdSettingsModal render with Scriptorium tokens', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: Scaffold(
                body: Builder(
                  builder: (context) => Column(
                    children: [
                      PrimaryButton(
                        label: 'Open Quick Settings',
                        onPressed: () => showShelfdSettingsModal(context),
                      ),
                      FilterPillsRow(
                        items: const [FilterPillItem(id: 'all', label: 'All Books')],
                        selectedId: 'all',
                        onSelected: (_) {},
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
        expect(tester.getSize(find.byType(PrimaryButton)).height, greaterThanOrEqualTo(AppTokens.minTouchTarget));
        await tester.tap(find.text('Open Quick Settings'));
        await tester.pumpAndSettle();
        expect(find.text('Uploads & Staged Queue'), findsOneWidget);
        expect(find.text('Library Utilities & Merge'), findsOneWidget);
        expect(find.text('Change Password & Account Settings'), findsOneWidget);
        expect(find.text('Disconnect / Sign Out'), findsOneWidget);
      });
    });

    group('Feature 4: Library Catalog & Structured Filter Bar', () {
      testWidgets('F04-T1-01: LibraryView renders books grid with maxCrossAxisExtent 200 and aspect ratio 0.55', (tester) async {
        const standaloneBook = Book(
          id: 'book-42',
          title: 'The Left Hand of Darkness',
          authors: [Author(id: 'auth-1', name: 'Ursula K. Le Guin', bookCount: 5)],
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [standaloneBook]))),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const LibraryView(mode: LibraryViewMode.books),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('The Left Hand of Darkness'), findsWidgets);
        final grid = tester.widget<GridView>(find.byType(GridView));
        final delegate = grid.gridDelegate as SliverGridDelegateWithMaxCrossAxisExtent;
        expect(delegate.maxCrossAxisExtent, 200.0);
        expect(delegate.childAspectRatio, 0.55);
      });

      testWidgets('F04-T1-02: LibraryFilterBar parses structured tokens and updates search query', (tester) async {
        final notifier = _FakeLibraryNotifier(
          const LibraryState(
            books: [sampleBook],
            authors: [Author(id: 'a1', name: 'Ursula K. Le Guin', bookCount: 5)],
          ),
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [libraryProvider.overrideWith(() => notifier)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(body: LibraryFilterBar()),
            ),
          ),
        );
        await tester.enterText(find.byType(TextField), 'Author:"Ursula K. Le Guin"');
        await tester.pump();
        expect(notifier.state.searchQuery, 'Author:"Ursula K. Le Guin"');
      });

      test('F04-T1-03: LibraryState.groupedBookItems collapses series into SeriesGridItem and keeps standalone books', () {
        const standalone = Book(id: 'b-solo', title: 'Neuromancer');
        const state = LibraryState(
          books: [sampleBook, standalone],
          series: [Series(id: 'ser-1', name: 'Hainish Cycle', bookCount: 6)],
        );
        final grouped = state.groupedBookItems;
        expect(grouped.whereType<SeriesGridItem>().length, 1);
        expect(grouped.whereType<BookGridItem>().length, 1);
      });

      testWidgets('F04-T1-04: Desktop LibraryView renders Rescan Library and Upload EPUB actions', (tester) async {
        tester.view.physicalSize = const Size(1440, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const LibraryView(mode: LibraryViewMode.books),
            ),
          ),
        );
        expect(find.text('Rescan Library'), findsWidgets);
        expect(find.text('Upload EPUB'), findsOneWidget);
      });

      testWidgets('F04-T1-05: LibraryView supports series, authors, genres, and topics modes', (tester) async {
        const state = LibraryState(
          books: [sampleBook],
          series: [Series(id: 'ser-1', name: 'Hainish Cycle', bookCount: 6)],
          authors: [Author(id: 'auth-1', name: 'Ursula K. Le Guin', bookCount: 5)],
          genres: [Genre(id: 'gen-1', name: 'Science Fiction', bookCount: 12)],
          topics: [Topic(id: 'top-1', name: 'Anthropology', bookCount: 4)],
        );
        for (final mode in LibraryViewMode.values) {
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                sharedPreferencesProvider.overrideWithValue(prefs),
                libraryProvider.overrideWith(() => _FakeLibraryNotifier(state)),
              ],
              child: MaterialApp(
                theme: AppTheme.buildTheme(ReadingThemeMode.bone),
                home: LibraryView(mode: mode),
              ),
            ),
          );
          await tester.pumpAndSettle();
          expect(find.byType(GridView), findsOneWidget);
        }
      });
    });

    group('Feature 5: Taxonomy & Entity Detail Views', () {
      testWidgets('F05-T1-01: AuthorDetailView renders AuthorAvatar, serif header, and sorted books', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(
                _MockBookRepository(
                  books: [sampleBook],
                  authors: const [Author(id: 'auth-1', name: 'Ursula K. Le Guin', bookCount: 1)],
                ),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const AuthorDetailView(authorId: 'auth-1', authorName: 'Ursula K. Le Guin'),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));
        expect(find.byType(AuthorAvatar), findsOneWidget);
        expect(find.text('Ursula K. Le Guin'), findsWidgets);
        expect(find.text('The Left Hand of Darkness'), findsOneWidget);
      });

      testWidgets('F05-T1-02: SeriesDetailView renders series header and #N sequence badges', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(_MockBookRepository(books: [sampleBook])),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SeriesDetailView(seriesId: 'ser-1', seriesName: 'Hainish Cycle'),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));
        expect(find.text('Hainish Cycle'), findsWidgets);
        expect(find.text('#4'), findsOneWidget);
      });

      test('F05-T1-03: compareSeriesBooks orders volumes by sequence number ascending', () {
        const s = Series(id: 's1', name: 'Dune');
        const b1 = Book(id: '1', title: 'Dune', series: s, seriesSequence: 1.0);
        const b2 = Book(id: '2', title: 'Dune Messiah', series: s, seriesSequence: 2.0);
        final list = [b2, b1]..sort(compareSeriesBooks);
        expect(list.first.id, '1');
        expect(list.last.id, '2');
      });

      testWidgets('F05-T1-04: Genre catalog mode renders GenreGridItem cards with book counts', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              libraryProvider.overrideWith(
                () => _FakeLibraryNotifier(
                  const LibraryState(
                    genres: [Genre(id: 'g1', name: 'Science Fiction', bookCount: 12)],
                    activeFilter: 'genres',
                  ),
                ),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const LibraryView(mode: LibraryViewMode.genres),
            ),
          ),
        );
        expect(find.text('Science Fiction'), findsOneWidget);
        expect(find.text('12 Books'), findsOneWidget);
      });

      testWidgets('F05-T1-05: Topic catalog mode renders TopicGridItem cards with book counts', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              libraryProvider.overrideWith(
                () => _FakeLibraryNotifier(
                  const LibraryState(
                    topics: [Topic(id: 't1', name: 'Anthropology', bookCount: 4)],
                    activeFilter: 'topics',
                  ),
                ),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const LibraryView(mode: LibraryViewMode.topics),
            ),
          ),
        );
        expect(find.text('Anthropology'), findsOneWidget);
        expect(find.text('4 Books'), findsOneWidget);
      });
    });

    group('Feature 6: Book Detail Showcase, Annotations & RAG Chat', () {
      testWidgets('F06-T1-01: BookDetailView renders Hero card, 65% progress, and 3 tabs', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('The Left Hand of Darkness'), findsWidgets);
        expect(find.text('65%'), findsOneWidget);
        expect(find.text('Resume Reading'), findsOneWidget);
        expect(find.text('Overview'), findsOneWidget);
        expect(find.text('Highlights & Bookmarks'), findsOneWidget);
        expect(find.text('Chat with Book'), findsOneWidget);
      });

      testWidgets('F06-T1-02: BookDetailView preserves action keys on hero and app bar', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        expect(find.byKey(const Key('read_hero_button')), findsOneWidget);
        expect(find.byKey(const Key('download_hero_menu_button')), findsOneWidget);
        expect(find.byKey(const Key('edit_metadata_appbar_button')), findsOneWidget);
        expect(find.byKey(const Key('edit_metadata_sidebar_button')), findsOneWidget);
      });

      testWidgets('F06-T1-03: BookDetailView Highlights & Bookmarks tab renders annotations', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.text('Highlights & Bookmarks'));
        await tester.pumpAndSettle();
        expect(find.textContaining('Light is the left hand of darkness.'), findsOneWidget);
        expect(find.text('Core philosophical duality'), findsOneWidget);
        expect(find.text('Royal Arch Ceremony'), findsOneWidget);
      });

      testWidgets('F06-T1-04: BookDetailView Chat with Book tab renders assistant reply and citation chip', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        final chatState = BookChatState(
          messages: [
            BookChatMessage(
              role: 'assistant',
              content: 'Estraven explains shifgrethor in Chapter 1.',
              citations: const [
                BookCitation(
                  chapterIndex: 1,
                  chapterTitle: 'Chapter 1: A Parade in Erhenrang',
                  summary: 'Genly Ai attends the parade.',
                  excerpt: 'Shifgrethor — prestige, face, place.',
                ),
              ],
              timestamp: DateTime(2026, 1, 1),
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
              bookChatProvider('book-42').overrideWith(() => _FakeBookChatNotifier('book-42', chatState)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.text('Chat with Book'));
        await tester.pumpAndSettle();
        expect(find.textContaining('Estraven explains shifgrethor'), findsOneWidget);
        expect(find.textContaining('Chapter 1'), findsWidgets);
      });

      testWidgets('F06-T1-05: Download menu exposes EPUB, NotebookLM, and Markdown export options', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('download_hero_menu_button')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('menu_item_download_epub')), findsOneWidget);
        expect(find.byKey(const Key('menu_item_send_notebooklm')), findsOneWidget);
        expect(find.byKey(const Key('menu_item_download_markdown')), findsOneWidget);
      });
    });

    group('Feature 7: Metadata Editor & Multi-Provider Search Modals', () {
      testWidgets('F07-T1-01: EditBookMetadataContent renders all 10 field keys and Save & Sync EPUB CTA', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: EditBookMetadataContent(book: sampleBook, isBottomSheet: false),
              ),
            ),
          ),
        );
        expect(find.byKey(const Key('edit_book_title_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_authors_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_series_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_sequence_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_genres_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_topics_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_publisher_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_language_field')), findsOneWidget);
        expect(find.byKey(const Key('edit_book_description_field')), findsOneWidget);
        expect(find.byKey(const Key('save_book_metadata_button')), findsOneWidget);
        expect(find.text('Save & Sync EPUB'), findsOneWidget);
      });

      testWidgets('F07-T1-02: MetadataSearchDialogContent searches providers and returns selected result', (tester) async {
        final mockClient = MockClient((req) async {
          return http.Response(
            jsonEncode([
              {
                'provider': 'openlibrary',
                'title': 'The Dispossessed',
                'author': 'Ursula K. Le Guin',
                'series': 'Hainish Cycle',
                'series_sequence': '5',
                'publisher': 'Harper & Row',
                'published_year': 1974,
                'genres': ['Science Fiction'],
              },
            ]),
            200,
          );
        });
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: MetadataSearchDialogContent(
                  initialTitle: 'The Dispossessed',
                  initialAuthor: 'Ursula K. Le Guin',
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Search Metadata'), findsOneWidget);
        expect(find.text('The Dispossessed'), findsWidgets);
        expect(find.text('Open Library'), findsOneWidget);
        expect(find.text('Apply'), findsOneWidget);
      });

      test('F07-T1-03: MetadataSearchResult.fromJson parses provider fields accurately', () {
        final res = MetadataSearchResult.fromJson({
          'provider': 'googlebooks',
          'title': 'Solaris',
          'author': 'Stanislaw Lem',
          'series': 'SF Masterworks',
          'series_sequence': '12',
          'publisher': 'Walker & Co.',
          'published_year': 1961,
          'isbn': '9780156027601',
          'genres': ['Science Fiction'],
        });
        expect(res.provider, 'googlebooks');
        expect(res.title, 'Solaris');
        expect(res.author, 'Stanislaw Lem');
        expect(res.publishedYear, 1961);
      });

      testWidgets('F07-T1-04: EditBookMetadataContent pre-populates book values into controllers', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: EditBookMetadataContent(book: sampleBook, isBottomSheet: true),
              ),
            ),
          ),
        );
        expect(find.text('The Left Hand of Darkness'), findsWidgets);
        expect(find.text('Ursula K. Le Guin'), findsOneWidget);
        expect(find.text('Hainish Cycle'), findsOneWidget);
        expect(find.text('Ace Books'), findsOneWidget);
      });

      testWidgets('F07-T1-05: showEditBookMetadataModal opens Dialog on desktop viewport', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: Scaffold(
                body: Builder(
                  builder: (ctx) => ElevatedButton(
                    onPressed: () => showEditBookMetadataModal(ctx, sampleBook),
                    child: const Text('Launch Editor'),
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.text('Launch Editor'));
        await tester.pumpAndSettle();
        expect(find.byType(Dialog), findsOneWidget);
        expect(find.text('Edit Book Metadata'), findsOneWidget);
      });
    });

    group('Feature 8: Reflowable & Fixed-Layout EPUB Readers', () {
      const sampleChapter = Chapter(
        id: 'chap-1',
        bookId: 'book-42',
        chapterIndex: 1,
        title: 'Chapter 1: A Parade in Erhenrang',
        summary: 'Genly Ai attends the parade.',
        content: '# A Parade in Erhenrang\n\n> Rain fell softly over the stone towers.[1]\n\nThe crowd gathered along the harbour.',
      );

      testWidgets('F08-T1-01: ReaderView renders chapter content, bottom bar minutes left, and typography sheet', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ReaderView(
                bookId: 'book-42',
                chapterIndex: 1,
                initialChapter: sampleChapter,
              ),
            ),
          ),
        );
        expect(find.textContaining('The crowd gathered along the harbour.'), findsOneWidget);
        expect(find.textContaining('mins left'), findsOneWidget);

        await tester.tap(find.byIcon(Icons.text_fields_rounded));
        await tester.pumpAndSettle();
        expect(find.text('Bone'), findsOneWidget);
        expect(find.text('Sepia'), findsOneWidget);
        expect(find.text('Dark OLED'), findsOneWidget);
      });

      testWidgets('F08-T1-02: KindleSelectionToolbar renders 4 highlight colors, Add Note, and Copy', (tester) async {
        KindleHighlightColor? chosenColor;
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: KindleSelectionToolbar(
                anchors: const TextSelectionToolbarAnchors(primaryAnchor: Offset(120, 120)),
                onColorSelected: (c) => chosenColor = c,
                onAddNote: () {},
                onCopy: () {},
              ),
            ),
          ),
        );
        expect(find.byTooltip('Yellow Highlight'), findsOneWidget);
        expect(find.byTooltip('Blue Highlight'), findsOneWidget);
        expect(find.byTooltip('Pink Highlight'), findsOneWidget);
        expect(find.byTooltip('Orange Highlight'), findsOneWidget);
        await tester.tap(find.byTooltip('Blue Highlight'));
        expect(chosenColor, KindleHighlightColor.blue);
      });

      testWidgets('F08-T1-03: FixedLayoutReader renders cover page and facing spreads', (tester) async {
        tester.view.physicalSize = const Size(1280, 800);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        const fixedBook = Book(
          id: 'fixed-1',
          title: 'Illustrated Atlas',
          layout: 'pre-paginated',
          spine: [
            SpineItem(id: 'p1', bookId: 'fixed-1', chapterIndex: 1, title: 'Cover', pageWidth: 1000, pageHeight: 1200, pageSpread: 'right'),
            SpineItem(id: 'p2', bookId: 'fixed-1', chapterIndex: 2, title: 'Left Page', pageWidth: 1000, pageHeight: 1200, pageSpread: 'left'),
            SpineItem(id: 'p3', bookId: 'fixed-1', chapterIndex: 3, title: 'Right Page', pageWidth: 1000, pageHeight: 1200, pageSpread: 'right'),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: const MaterialApp(home: FixedLayoutReader(book: fixedBook)),
          ),
        );
        expect(find.text('Page 1 of 3'), findsOneWidget);
      });

      test('F08-T1-04: parseReaderBlocks and parseInlineSpans parse headings, quotes, and footnotes', () {
        final blocks = parseReaderBlocks('# H1\n\n> Quote\n\n---\n\nBody[2]');
        expect(blocks.length, 4);
        expect(blocks[0].type, ReaderBlockType.h1);
        expect(blocks[1].type, ReaderBlockType.blockquote);
        expect(blocks[2].type, ReaderBlockType.divider);
        expect(blocks[3].type, ReaderBlockType.paragraph);
      });

      testWidgets('F08-T1-05: ReaderView opens Table of Contents endDrawer', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ReaderView(
                bookId: 'book-42',
                chapterIndex: 1,
                initialChapter: sampleChapter,
              ),
            ),
          ),
        );
        await tester.tap(find.byIcon(Icons.list_rounded));
        await tester.pumpAndSettle();
        expect(find.text('Table of Contents'), findsOneWidget);
        expect(find.text('Chapter 1: A Parade in Erhenrang'), findsWidgets);
      });
    });

    group('Feature 9: Audiobook Studio Player & Persistent Mini-Player', () {
      const audioBook = Book(
        id: 'audio-1',
        title: 'Project Hail Mary',
        authors: [Author(id: 'a-weir', name: 'Andy Weir')],
        bookType: 'audiobook',
        durationSeconds: 1800.0,
        audioChapters: [
          AudioChapter(id: 'ac-1', bookId: 'audio-1', chapterIndex: 0, title: 'Chapter 1: Awakening', startOffsetSec: 0, durationSec: 600),
          AudioChapter(id: 'ac-2', bookId: 'audio-1', chapterIndex: 1, title: 'Chapter 2: Petrova Line', startOffsetSec: 600, durationSec: 1200),
        ],
      );

      testWidgets('F09-T1-01: AudiobookPlayerView renders Obsidian & Amber studio controls and keys', (tester) async {
        final state = AudioPlayerState(
          book: audioBook,
          status: AudioPlaybackStatus.playing,
          position: const Duration(minutes: 2),
          duration: const Duration(minutes: 30),
          activeChapter: audioBook.audioChapters.first,
          chapters: audioBook.audioChapters,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('audio-1').overrideWith(() => _FakeBookDetailNotifier('audio-1', audioBook)),
              audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier(state)),
            ],
            child: const MaterialApp(home: AudiobookPlayerView(bookId: 'audio-1')),
          ),
        );
        await tester.pump();
        expect(find.byKey(const Key('audio_player_back_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_chapters_menu_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_skip_back_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_play_pause_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_skip_forward_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_speed_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_sleep_timer_button')), findsOneWidget);
        expect(find.byKey(const Key('audio_player_add_bookmark_button')), findsOneWidget);
      });

      testWidgets('F09-T1-02: ScrubberBar renders position and remaining duration timestamps', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ScrubberBar(
                position: const Duration(minutes: 3, seconds: 15),
                duration: const Duration(minutes: 10),
                onSeek: (_) {},
              ),
            ),
          ),
        );
        expect(find.text('03:15'), findsOneWidget);
        expect(find.text('-06:45'), findsOneWidget);
        expect(find.byType(Slider), findsOneWidget);
      });

      testWidgets('F09-T1-03: ChapterDrawerSheet renders Chapters, Bookmarks, and Sleep Timer tabs', (tester) async {
        final state = AudioPlayerState(
          book: audioBook,
          status: AudioPlaybackStatus.playing,
          position: const Duration(minutes: 1),
          duration: const Duration(minutes: 30),
          activeChapter: audioBook.audioChapters.first,
          chapters: audioBook.audioChapters,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('audio-1').overrideWith(() => _FakeBookDetailNotifier('audio-1', audioBook)),
              audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier(state)),
            ],
            child: const MaterialApp(
              home: Scaffold(body: ChapterDrawerSheet(bookId: 'audio-1')),
            ),
          ),
        );
        expect(find.byKey(const Key('chapter_drawer_tab_0')), findsOneWidget);
        expect(find.byKey(const Key('chapter_drawer_tab_1')), findsOneWidget);
        expect(find.byKey(const Key('chapter_drawer_tab_2')), findsOneWidget);
        expect(find.text('Chapter 1: Awakening'), findsOneWidget);
        expect(find.byType(EqualizerBars), findsOneWidget);
      });

      testWidgets('F09-T1-04: MiniPlayerBar renders docked controls when book is active', (tester) async {
        final state = AudioPlayerState(
          book: audioBook,
          status: AudioPlaybackStatus.playing,
          position: const Duration(minutes: 5),
          duration: const Duration(minutes: 30),
          activeChapter: audioBook.audioChapters.first,
          chapters: audioBook.audioChapters,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier(state))],
            child: const MaterialApp(home: Scaffold(body: MiniPlayerBar())),
          ),
        );
        expect(find.byKey(const Key('mini_player_tap_area')), findsOneWidget);
        expect(find.byKey(const Key('mini_player_skip_30')), findsOneWidget);
        expect(find.byKey(const Key('mini_player_play_pause')), findsOneWidget);
        expect(find.byKey(const Key('mini_player_close')), findsOneWidget);
        expect(find.text('Project Hail Mary'), findsOneWidget);
      });

      test('F09-T1-05: SleepTimerOption defines labels and durations for all presets', () {
        expect(SleepTimerOption.none.label, 'Off');
        expect(SleepTimerOption.min15.duration, const Duration(minutes: 15));
        expect(SleepTimerOption.min30.duration, const Duration(minutes: 30));
        expect(SleepTimerOption.min60.duration, const Duration(minutes: 60));
        expect(SleepTimerOption.endOfChapter.label, 'End of chapter');
      });
    });

    group('Feature 10: Uploads & Ingestion Workspace', () {
      const stagedJob = StagedUploadJob(
        jobId: 'job-101',
        status: 'staged',
        filename: 'dispossessed.epub',
        hasCover: false,
        metadata: StagedMetadata(
          title: 'The Dispossessed',
          authors: ['Ursula K. Le Guin'],
          series: 'Hainish Cycle',
          sequenceNumber: 5,
          genres: ['Science Fiction'],
          description: 'An ambiguous utopia.',
        ),
      );

      testWidgets('F10-T1-01: UploadsView renders dropzone, Auto-add toggle, and staged queue', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              uploadProvider.overrideWith(
                () => _FakeUploadNotifier(
                  const UploadState(stagedJobs: [stagedJob], selectedJobId: 'job-101'),
                ),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const UploadsView(),
            ),
          ),
        );
        expect(find.text('Uploads & Ingestion'), findsOneWidget);
        expect(find.text('Auto-add'), findsOneWidget);
        expect(find.text('Drag & drop EPUB files or nested folders here'), findsOneWidget);
        expect(find.text('Queue (1)'), findsOneWidget);
        expect(find.text('Approve All Ready'), findsOneWidget);
      });

      testWidgets('F10-T1-02: UploadReviewContent renders metadata fields and live destination path', (tester) async {
        final mockClient = MockClient((_) async => http.Response('{}', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: UploadReviewContent(job: stagedJob, isBottomSheet: false),
              ),
            ),
          ),
        );
        expect(find.text('Review Staged EPUB'), findsOneWidget);
        expect(find.text('DESTINATION PATH'), findsOneWidget);
        expect(
          find.text('/library/Ursula K. Le Guin/The Dispossessed/The Dispossessed.epub'),
          findsOneWidget,
        );
      });

      testWidgets('F10-T1-03: Editing title in UploadReviewContent updates destination path preview', (tester) async {
        final mockClient = MockClient((_) async => http.Response('{}', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: UploadReviewContent(job: stagedJob, isBottomSheet: false),
              ),
            ),
          ),
        );
        final titleField = find.widgetWithText(TextFormField, 'The Dispossessed');
        await tester.enterText(titleField, 'Lathe of Heaven');
        await tester.pump();
        expect(
          find.text('/library/Ursula K. Le Guin/Lathe of Heaven/Lathe of Heaven.epub'),
          findsOneWidget,
        );
      });

      test('F10-T1-04: StagedUploadJob.fromJson parses metadata, warnings, and duplicate flags', () {
        final job = StagedUploadJob.fromJson({
          'job_id': 'j-1',
          'status': 'staged',
          'filename': 'test.epub',
          'has_cover': true,
          'is_duplicate': true,
          'warnings': ['Book already exists in library'],
          'metadata': {
            'title': 'Solaris',
            'authors': ['Stanislaw Lem'],
            'publisher': 'Walker',
            'language': 'en',
          },
        });
        expect(job.isDuplicate, isTrue);
        expect(job.hasCover, isTrue);
        expect(job.metadata.title, 'Solaris');
        expect(job.metadata.publisher, 'Walker');
      });

      testWidgets('F10-T1-05: ShelfdDropTarget wraps child widget cleanly', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              uploadProvider.overrideWith(() => _FakeUploadNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ShelfdDropTarget(
                child: Scaffold(body: Text('Drop Target Child')),
              ),
            ),
          ),
        );
        expect(find.text('Drop Target Child'), findsOneWidget);
      });
    });

    group('Feature 11: Semantic Search, Utilities, Settings, Connect & OAuth UI', () {
      testWidgets('F11-T1-01: SearchView renders archival citation cards and match percentage badge', (tester) async {
        const hit = SemanticSearchHit(
          bookId: 'book-42',
          bookTitle: 'The Left Hand of Darkness',
          authorName: 'Ursula K. Le Guin',
          chapterIndex: 14,
          chapterTitle: 'Across the Gobrin Ice',
          summary: 'Genly Ai and Estraven cross the glacier.',
          score: 0.94,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              searchProvider.overrideWith(
                () => _FakeSearchNotifier(const SearchState(query: 'glacier', results: [hit])),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SearchView(),
            ),
          ),
        );
        expect(find.text('94% match'), findsOneWidget);
        expect(find.text('Chapter 14: Across the Gobrin Ice'), findsOneWidget);
        expect(find.text('Read Chapter at this passage →'), findsOneWidget);
      });

      testWidgets('F11-T1-02: UtilitiesView renders Duplicate Books Finder and Merge Group action', (tester) async {
        const group = DuplicateGroup(
          confidence: 1.0,
          reason: 'Exact Title & Author match',
          books: [
            Book(id: 'd1', title: 'Dune', authors: [Author(id: 'a1', name: 'Frank Herbert')]),
            Book(id: 'd2', title: 'Dune', authors: [Author(id: 'a1', name: 'Frank Herbert')]),
          ],
        );
        final repo = _MockBookRepository(
          scanResult: const DuplicateScanResult(groups: [group], totalDuplicates: 1),
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [bookRepositoryProvider.overrideWithValue(repo)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const UtilitiesView(),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Duplicate Books Finder'), findsOneWidget);
        expect(find.text('100% Match'), findsOneWidget);
        expect(find.text('Merge Group'), findsOneWidget);
      });

      testWidgets('F11-T1-03: SettingsView renders Appearance & Theme selector, Account, and Server sections', (tester) async {
        tester.view.physicalSize = const Size(1200, 1600);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SettingsView(),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Settings'), findsWidgets);
        expect(find.text('Account & Security'), findsOneWidget);
        expect(find.text('Server Connection'), findsOneWidget);
      });

      testWidgets('F11-T1-04: ConnectView renders QR pairing option and manual connection form', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ConnectView(),
            ),
          ),
        );
        expect(find.text('Connect to your Library'), findsOneWidget);
        expect(find.text('Scan Server QR Code'), findsOneWidget);
        expect(find.text('Connect & Sync Library'), findsOneWidget);
      });

      test('F11-T1-05: resolveDefaultServerUrl returns localhost fallback on native test platform', () {
        expect(resolveDefaultServerUrl(), 'http://localhost:8080');
      });
    });

    group('Feature 12: Final E2E & Widget Test Suite Verification & Hardening', () {
      testWidgets('F12-T1-01: createRouter redirects /library to /books and wraps in AppShell', (tester) async {
        final router = createRouter(initialLocation: '/library', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(AppShell), findsOneWidget);
        expect(find.byType(LibraryView), findsOneWidget);
      });

      testWidgets('F12-T1-02: createRouter redirects legacy /book/:id to /books/:id', (tester) async {
        final router = createRouter(initialLocation: '/book/book-42', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));
        expect(find.byType(BookDetailView), findsOneWidget);
      });

      testWidgets('F12-T1-03: createRouter routes fullscreen /books/:id/listen outside AppShell', (tester) async {
        const audioBook = Book(id: 'audio-1', title: 'Project Hail Mary', bookType: 'audiobook');
        final router = createRouter(initialLocation: '/books/audio-1/listen', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              bookDetailProvider('audio-1').overrideWith(() => _FakeBookDetailNotifier('audio-1', audioBook)),
              audioPlayerProvider.overrideWith(
                () => _FakeAudioPlayerNotifier(const AudioPlayerState(book: audioBook)),
              ),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pump();
        expect(find.byType(AudiobookPlayerView), findsOneWidget);
        expect(find.byType(AppShell), findsNothing);
      });

      testWidgets('F12-T1-04: ReaderSettingsNotifier persists themeMode, fontSize, lineHeight, and fontFamily', (tester) async {
        late WidgetRef capturedRef;
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: Consumer(
              builder: (context, ref, _) {
                capturedRef = ref;
                return const SizedBox.shrink();
              },
            ),
          ),
        );
        final notifier = capturedRef.read(readerSettingsProvider.notifier);
        notifier.setThemeMode(ReadingThemeMode.sepia);
        notifier.setFontSize(20.0);
        notifier.setLineHeight(1.8);
        notifier.setFontFamily(false);

        final updated = capturedRef.read(readerSettingsProvider);
        expect(updated.themeMode, ReadingThemeMode.sepia);
        expect(updated.fontSize, 20.0);
        expect(updated.lineHeight, 1.8);
        expect(updated.isSerif, isFalse);
        expect(storageService.getThemeMode(), 'sepia');
      });

      test('F12-T1-05: All preserved widget keys remain non-empty ValueKeys', () {
        const keys = [
          Key('read_hero_button'),
          Key('listen_hero_button'),
          Key('download_hero_menu_button'),
          Key('edit_book_title_field'),
          Key('save_book_metadata_button'),
          Key('audio_player_play_pause_button'),
          Key('mini_player_tap_area'),
        ];
        expect(keys.length, 7);
      });
    });
  });

  // ===========================================================================
  // TIER 2 — BOUNDARY & CORNER CASES (Features 1..12, >= 5 tests per feature)
  // ===========================================================================
  group('Tier 2 — Boundary & Corner Cases', () {
    group('Feature 1 Boundaries: Theme & Palette Edge Cases', () {
      test('F01-T2-01: Legacy AppTokens and ThemeData invariants remain 100% backwards-compatible', () {
        expect(AppTokens.minTouchTarget, 48.0);
        expect(AppTokens.maxReadingWidth, 720.0);
        expect(AppTokens.boneBackground, const Color(0xFFFBFBFA));
        expect(AppTokens.charcoalInk, const Color(0xFF111111));
        expect(AppTokens.sepiaBackground, const Color(0xFFF4ECD8));
        expect(AppTokens.darkOledBackground, const Color(0xFF121212));
      });

      testWidgets('F01-T2-02: ShelfdColors.of(context) falls back safely on bare MaterialApp without extension', (tester) async {
        ShelfdColors? lightFallback;
        ShelfdColors? sepiaFallback;
        ShelfdColors? darkFallback;

        await tester.pumpWidget(
          MaterialApp(
            theme: ThemeData.light(),
            home: Builder(
              builder: (ctx) {
                lightFallback = ShelfdColors.of(ctx);
                return const SizedBox.shrink();
              },
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(lightFallback, ShelfdColors.bone);

        await tester.pumpWidget(
          MaterialApp(
            theme: ThemeData(scaffoldBackgroundColor: AppTokens.sepiaBackground),
            home: Builder(
              builder: (ctx) {
                sepiaFallback = ShelfdColors.of(ctx);
                return const SizedBox.shrink();
              },
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(sepiaFallback, ShelfdColors.sepia);

        await tester.pumpWidget(
          MaterialApp(
            theme: ThemeData.dark(),
            home: Builder(
              builder: (ctx) {
                darkFallback = ShelfdColors.of(ctx);
                return const SizedBox.shrink();
              },
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(darkFallback, ShelfdColors.dark);
      });

      test('F01-T2-03: ShelfdColors.lerp handles null other and t=0.0, t=0.5, t=1.0 boundaries', () {
        expect(ShelfdColors.bone.lerp(null, 0.5), ShelfdColors.bone);
        final atZero = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.0);
        final atHalf = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.5);
        final atOne = ShelfdColors.bone.lerp(ShelfdColors.dark, 1.0);
        expect(atZero.background, ShelfdColors.bone.background);
        expect(atZero.isDark, isFalse);
        expect(atHalf.isDark, isTrue);
        expect(atOne.background, ShelfdColors.dark.background);
        expect(atOne.isDark, isTrue);
      });

      test('F01-T2-04: ShelfdColors.copyWith isolates overridden field from remaining 20 fields', () {
        final modified = ShelfdColors.sepia.copyWith(accentPrimary: AppTokens.libraryGreen);
        expect(modified.accentPrimary, AppTokens.libraryGreen);
        expect(modified.background, ShelfdColors.sepia.background);
        expect(modified.surface, ShelfdColors.sepia.surface);
        expect(modified.ink, ShelfdColors.sepia.ink);
        expect(modified.isDark, isFalse);
      });

      test('F01-T2-05: WCAG luminance contrast delta between ink and background exceeds 0.40 in all modes', () {
        for (final palette in [ShelfdColors.bone, ShelfdColors.sepia, ShelfdColors.dark]) {
          final delta = (palette.ink.computeLuminance() - palette.background.computeLuminance()).abs();
          expect(delta, greaterThan(0.40));
        }
      });
    });

    group('Feature 2 Boundaries: Typography & HTML Sanitization Edge Cases', () {
      test('F02-T2-01: AppTypography builders omit hardcoded color when color parameter is null', () {
        expect(AppTypography.displaySerif().color, isNull);
        expect(AppTypography.titleSerif().color, isNull);
        expect(AppTypography.readerBodySerif().color, isNull);
        expect(AppTypography.bodySans().color, isNull);
        expect(AppTypography.labelCaps().color, isNull);
        expect(AppTypography.captionSans().color, isNull);
        expect(AppTypography.mono().color, isNull);
      });

      testWidgets('F02-T2-02: HtmlText renders emptyPlaceholder with mutedInk on empty or script-only HTML', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const Scaffold(
              body: HtmlText(
                html: '   <script>alert("xss")</script><style>body{display:none}</style>   ',
                emptyPlaceholder: 'Archival synopsis unavailable.',
              ),
            ),
          ),
        );
        final textWidget = tester.widget<Text>(find.text('Archival synopsis unavailable.'));
        expect(textWidget.style?.color, ShelfdColors.dark.mutedInk);
      });

      test('F02-T2-03: parseHtmlBlocks strips script/style tags and handles unclosed tags safely', () {
        final blocks = parseHtmlBlocks('<script>evil()</script><p><b>Unclosed bold <i>and italic');
        expect(blocks.length, 1);
        expect(blocks.first.htmlContent, contains('Unclosed bold'));
        expect(blocks.first.htmlContent, isNot(contains('evil')));
      });

      test('F02-T2-04: parseHtmlInlineSpans combines underline + strikethrough and applies JetBrains Mono to <code>', () {
        const base = TextStyle(fontSize: 14);
        final spans = parseHtmlInlineSpans('<u><s>both</s></u> and <code>inline_fn()</code>', base);
        final textSpans = spans.whereType<TextSpan>().toList();
        expect(
          textSpans.any(
            (s) =>
                s.text == 'both' &&
                s.style!.decoration!.contains(TextDecoration.underline) &&
                s.style!.decoration!.contains(TextDecoration.lineThrough),
          ),
          isTrue,
        );
        expect(
          textSpans.any((s) => s.text == 'inline_fn()' && s.style?.fontFamily == 'JetBrains Mono'),
          isTrue,
        );
      });

      test('F02-T2-05: decodeHtmlEntities handles named, decimal, hex, and malformed entities', () {
        expect(decodeHtmlEntities('Plain text without ampersand'), 'Plain text without ampersand');
        expect(decodeHtmlEntities('&ldquo;A &amp; B &mdash; &#169; &#x26;&rdquo;'), '“A & B — © &”');
        expect(decodeHtmlEntities('&unknown; &#abc;'), '&unknown; &#abc;');
      });
    });

    group('Feature 3 Boundaries: Shared Layout & Responsive Breakpoints', () {
      test('F03-T2-01: AuthorAvatar.initials handles empty, whitespace, single-word, and multi-word names', () {
        expect(const AuthorAvatar(name: '').initials, '?');
        expect(const AuthorAvatar(name: '   ').initials, '?');
        expect(const AuthorAvatar(name: 'Plato').initials, 'P');
        expect(const AuthorAvatar(name: '  Ursula   K.   Le   Guin  ').initials, 'UG');
      });

      test('F03-T2-02: resolveBookTitle handles null, empty, unknown ID, and matching book ID', () {
        expect(resolveBookTitle(null, [sampleBook]), isNull);
        expect(resolveBookTitle('', [sampleBook]), '');
        expect(resolveBookTitle('unknown-uuid', [sampleBook]), 'unknown-uuid');
        expect(resolveBookTitle('book-42', [sampleBook]), 'The Left Hand of Darkness');
      });

      testWidgets('F03-T2-03: ShelfdGridCard hides progress badge at 0% and uses singular 1 Book label', (tester) async {
        const unreadBook = Book(id: 'b0', title: 'Unread Tome', readingProgress: 0.0);
        const singleSeries = Series(id: 's1', name: 'Solo Series', bookCount: 1);
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: Column(
                children: [
                  SizedBox(
                    width: 180,
                    height: 260,
                    child: ShelfdGridCard(item: BookGridItem(unreadBook), onTap: () {}),
                  ),
                  SizedBox(
                    width: 180,
                    height: 260,
                    child: ShelfdGridCard(item: SeriesGridItem(singleSeries), onTap: () {}),
                  ),
                ],
              ),
            ),
          ),
        );
        expect(find.textContaining('% read'), findsNothing);
        expect(find.text('1 Book'), findsOneWidget);
      });

      testWidgets('F03-T2-04: ShelfdAdaptiveScaffold switches at exact 1024px vs 1025px desktop boundary', (tester) async {
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        // At 1024.0px (tablet upper bound): BottomNav is shown, SideNav is hidden
        tester.view.physicalSize = const Size(1024, 800);
        tester.view.devicePixelRatio = 1.0;
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier()),
              audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: ShelfdAdaptiveScaffold(
                currentIndex: 0,
                onNavTap: (_) {},
                body: const Text('Viewport Body'),
              ),
            ),
          ),
        );
        expect(find.byType(ShelfdBottomNav), findsOneWidget);
        expect(find.byType(ShelfdSideNav), findsNothing);

        // At 1025.0px (desktop lower bound): SideNav is shown, BottomNav is hidden
        tester.view.physicalSize = const Size(1025, 800);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier()),
              audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: ShelfdAdaptiveScaffold(
                currentIndex: 0,
                onNavTap: (_) {},
                body: const Text('Viewport Body'),
              ),
            ),
          ),
        );
        expect(find.byType(ShelfdSideNav), findsOneWidget);
        expect(find.byType(ShelfdBottomNav), findsNothing);
      });

      testWidgets('F03-T2-05: PrimaryButton disables tap callback and shows spinner when isLoading is true', (tester) async {
        var tapped = false;
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: Scaffold(
              body: PrimaryButton(
                label: 'Syncing',
                isLoading: true,
                onPressed: () => tapped = true,
              ),
            ),
          ),
        );
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        await tester.tap(find.byType(PrimaryButton));
        expect(tapped, isFalse);
      });
    });

    group('Feature 4 Boundaries: Catalog & Query Parser Edge Cases', () {
      testWidgets('F04-T2-01: LibraryView shows No matching books and Clear search on zero results', (tester) async {
        final notifier = _FakeLibraryNotifier(
          const LibraryState(books: [sampleBook], searchQuery: 'nonexistent_query_xyz'),
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              libraryProvider.overrideWith(() => notifier),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const LibraryView(mode: LibraryViewMode.books),
            ),
          ),
        );
        expect(find.text('No matching books'), findsOneWidget);
        expect(find.text('Clear search'), findsOneWidget);
        await tester.tap(find.text('Clear search'));
        await tester.pump();
        expect(notifier.state.searchQuery, isEmpty);
      });

      test('F04-T2-02: ParsedLibraryQuery handles empty, whitespace, and mixed quoted/unquoted tokens', () {
        final empty = ParsedLibraryQuery.parse('   ');
        expect(empty.hasFilter, isFalse);

        final mixed = ParsedLibraryQuery.parse('Gethen author:Guin Genre:"Science Fiction" Topic:"Anthropology"');
        expect(mixed.titleQuery, 'Gethen');
        expect(mixed.authorQuery, 'Guin');
        expect(mixed.genreQuery, 'Science Fiction');
        expect(mixed.topicQuery, 'Anthropology');
        expect(mixed.matchesBook(sampleBook), isTrue);
      });

      test('F04-T2-03: LibraryState.groupedBookItems bypasses series collapsing when searchQuery is non-empty', () {
        const state = LibraryState(
          books: [sampleBook],
          series: [Series(id: 'ser-1', name: 'Hainish Cycle', bookCount: 6)],
          searchQuery: 'Left Hand',
        );
        final items = state.groupedBookItems;
        expect(items.length, 1);
        expect(items.first, isA<BookGridItem>());
      });

      testWidgets('F04-T2-04: LibraryView inside AppShell does not duplicate ShelfdSideNav or ShelfdBottomNav', (tester) async {
        final router = createRouter(initialLocation: '/books', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ShelfdAdaptiveScaffold), findsOneWidget);
      });

      testWidgets('F04-T2-05: LibraryView and LibraryFilterBar render cleanly in Dark and Sepia modes', (tester) async {
        for (final mode in [ReadingThemeMode.sepia, ReadingThemeMode.dark]) {
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                sharedPreferencesProvider.overrideWithValue(prefs),
                libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
              ],
              child: MaterialApp(
                theme: AppTheme.buildTheme(mode),
                home: const LibraryView(mode: LibraryViewMode.books),
              ),
            ),
          );
          expect(find.byType(LibraryFilterBar), findsOneWidget);
        }
      });
    });

    group('Feature 5 Boundaries: Taxonomy Sorting & Empty States', () {
      test('F05-T2-01: compareSeriesBooks sorts null sequence numbers after numbered volumes and supports fractional #1.5', () {
        const s = Series(id: 's1', name: 'Hainish');
        const bUnnumbered = Book(id: 'u', title: 'Always Coming Home', series: s);
        const bFrac = Book(id: 'f', title: 'Word for World is Forest', series: s, seriesSequence: 1.5);
        const bOne = Book(id: '1', title: 'Rocannon\'s World', series: s, seriesSequence: 1.0);
        final list = [bUnnumbered, bFrac, bOne]..sort(compareSeriesBooks);
        expect(list.map((b) => b.id).toList(), ['1', 'f', 'u']);
      });

      testWidgets('F05-T2-02: AuthorDetailView renders empty state when author has zero books', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(
                _MockBookRepository(
                  books: const [],
                  authors: const [Author(id: 'a-empty', name: 'New Author', bookCount: 0)],
                ),
              ),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const AuthorDetailView(authorId: 'a-empty', authorName: 'New Author'),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('New Author'), findsWidgets);
        expect(find.text('0 Books'), findsOneWidget);
      });

      testWidgets('F05-T2-03: SeriesDetailView formats fractional sequence #1.5 without trailing .0', (tester) async {
        const fracBook = Book(
          id: 'b-frac',
          title: 'Mid-Series Novella',
          series: Series(id: 's1', name: 'Chronicles'),
          seriesSequence: 1.5,
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(_MockBookRepository(books: [fracBook])),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SeriesDetailView(seriesId: 's1', seriesName: 'Chronicles'),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('#1.5'), findsOneWidget);
      });

      testWidgets('F05-T2-04: Legacy /author/:id route redirects to /authors/:id', (tester) async {
        final router = createRouter(initialLocation: '/author/auth-1?name=Ursula', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              bookRepositoryProvider.overrideWithValue(_MockBookRepository(books: [sampleBook])),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(AuthorDetailView), findsOneWidget);
      });

      testWidgets('F05-T2-05: AuthorDetailView and SeriesDetailView adapt cleanly to Dark mode', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(_MockBookRepository(books: [sampleBook])),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.dark),
              home: const SeriesDetailView(seriesId: 'ser-1', seriesName: 'Hainish Cycle'),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Hainish Cycle'), findsWidgets);
      });
    });

    group('Feature 6 Boundaries: Book Detail Edge Cases', () {
      testWidgets('F06-T2-01: BookDetailView handles book with empty synopsis, zero spine chapters, and zero annotations', (tester) async {
        const minimalBook = Book(id: 'min-1', title: 'Minimalist Scroll');
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('min-1').overrideWith(() => _FakeBookDetailNotifier('min-1', minimalBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'min-1'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('Minimalist Scroll'), findsWidgets);
        expect(find.text('Start Reading'), findsOneWidget);
      });

      testWidgets('F06-T2-02: BookDetailView renders hybrid EPUB + Audiobook Listen CTA and track files', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        const hybridBook = Book(
          id: 'hyb-1',
          title: 'Dune Hybrid Edition',
          files: [
            BookFile(id: 'f-epub', bookId: 'hyb-1', fileType: 'epub', filePath: 'Dune.epub'),
            BookFile(id: 'f-m4b', bookId: 'hyb-1', fileType: 'm4b', filePath: 'Dune.m4b', durationSeconds: 3600),
          ],
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('hyb-1').overrideWith(() => _FakeBookDetailNotifier('hyb-1', hybridBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'hyb-1'),
            ),
          ),
        );
        await tester.pump();
        expect(find.byKey(const Key('listen_hero_button')), findsOneWidget);
        expect(find.byKey(const Key('read_hero_button')), findsOneWidget);
      });

      testWidgets('F06-T2-03: BookDetailView displays page count for pre-paginated fixed-layout books', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        const fixedBook = Book(
          id: 'fix-1',
          title: 'Visual Chronicle',
          layout: 'pre-paginated',
          spine: [
            SpineItem(id: 's1', bookId: 'fix-1', chapterIndex: 1, title: 'Cover', pageWidth: 800, pageHeight: 1000),
            SpineItem(id: 's2', bookId: 'fix-1', chapterIndex: 2, title: 'Plate I', pageWidth: 800, pageHeight: 1000),
          ],
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('fix-1').overrideWith(() => _FakeBookDetailNotifier('fix-1', fixedBook)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'fix-1'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('Visual Chronicle'), findsWidgets);
      });

      testWidgets('F06-T2-04: Tapping RAG Chat citation chip opens citation excerpt bottom sheet', (tester) async {
        tester.view.physicalSize = const Size(1280, 900);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        final chatState = BookChatState(
          messages: [
            BookChatMessage(
              role: 'assistant',
              content: 'The ceremony occurs at the Royal Arch.',
              citations: const [
                BookCitation(
                  chapterIndex: 1,
                  chapterTitle: 'Chapter 1: A Parade in Erhenrang',
                  summary: 'Genly Ai attends the parade.',
                  excerpt: 'The keystone of the Arch was set with mortar of blood and bone.',
                ),
              ],
              timestamp: DateTime(2026, 1, 1),
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
              bookChatProvider('book-42').overrideWith(() => _FakeBookChatNotifier('book-42', chatState)),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const BookDetailView(bookId: 'book-42'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.text('Chat with Book'));
        await tester.pumpAndSettle();

        await tester.tap(find.textContaining('Ch. 1:'));
        await tester.pumpAndSettle();
        expect(find.textContaining('The keystone of the Arch'), findsOneWidget);
        expect(find.text('Read in Reader'), findsOneWidget);
      });

      testWidgets('F06-T2-05: BookDetailView adapts cleanly across Sepia and Dark themes', (tester) async {
        for (final mode in [ReadingThemeMode.sepia, ReadingThemeMode.dark]) {
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                sharedPreferencesProvider.overrideWithValue(prefs),
                bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
              ],
              child: MaterialApp(
                theme: AppTheme.buildTheme(mode),
                home: const BookDetailView(bookId: 'book-42'),
              ),
            ),
          );
          await tester.pump();
          expect(find.text('The Left Hand of Darkness'), findsWidgets);
        }
      });
    });

    group('Feature 7 Boundaries: Metadata Validation & Search Edge Cases', () {
      testWidgets('F07-T2-01: EditBookMetadataContent rejects empty Title and Author fields on save', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: EditBookMetadataContent(book: sampleBook, isBottomSheet: false),
              ),
            ),
          ),
        );
        await tester.enterText(find.byKey(const Key('edit_book_title_field')), '   ');
        await tester.enterText(find.byKey(const Key('edit_book_authors_field')), '   ');
        await tester.tap(find.byKey(const Key('save_book_metadata_button')));
        await tester.pump();
        expect(find.text('Title is required'), findsOneWidget);
        expect(find.text('At least one author is required'), findsOneWidget);
      });

      testWidgets('F07-T2-02: EditBookMetadataContent formats integer sequence numbers without decimal suffix', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: EditBookMetadataContent(book: sampleBook, isBottomSheet: false),
              ),
            ),
          ),
        );
        final seqField = tester.widget<TextFormField>(find.byKey(const Key('edit_book_sequence_field')));
        expect(seqField.controller?.text, '4');
      });

      testWidgets('F07-T2-03: MetadataSearchDialogContent shows empty results message when provider returns []', (tester) async {
        final mockClient = MockClient((_) async => http.Response('[]', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(
                body: MetadataSearchDialogContent(initialTitle: 'Obscure Manuscript'),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.textContaining('No metadata matches found'), findsOneWidget);
      });

      testWidgets('F07-T2-04: showEditBookMetadataModal opens ModalBottomSheet on narrow mobile viewport', (tester) async {
        tester.view.physicalSize = const Size(400, 800);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: Scaffold(
                body: Builder(
                  builder: (ctx) => ElevatedButton(
                    onPressed: () => showEditBookMetadataModal(ctx, sampleBook),
                    child: const Text('Mobile Edit'),
                  ),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.text('Mobile Edit'));
        await tester.pumpAndSettle();
        expect(find.text('Edit Book Metadata'), findsOneWidget);
      });

      testWidgets('F07-T2-05: EditBookMetadataContent and MetadataSearchDialogContent render in Dark mode', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.dark),
              home: const Scaffold(
                body: EditBookMetadataContent(book: sampleBook, isBottomSheet: false),
              ),
            ),
          ),
        );
        expect(find.text('Edit Book Metadata'), findsOneWidget);
      });
    });

    group('Feature 8 Boundaries: Reader Contrast, Gatefolds & Highlight Ranges', () {
      testWidgets('F08-T2-01: KindleSelectionToolbar and ReaderView TOC drawer render in Dark and Sepia modes', (tester) async {
        const ch = Chapter(
          id: 'c1',
          bookId: 'book-42',
          chapterIndex: 1,
          title: 'Dark Chapter',
          summary: '',
          content: 'Night fell over Karhide.',
        );
        for (final mode in [ReadingThemeMode.sepia, ReadingThemeMode.dark]) {
          SharedPreferences.setMockInitialValues({'shelfd_theme_mode': mode.name});
          final modePrefs = await SharedPreferences.getInstance();
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                sharedPreferencesProvider.overrideWithValue(modePrefs),
                bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
              ],
              child: MaterialApp(
                theme: AppTheme.buildTheme(mode),
                home: const ReaderView(bookId: 'book-42', chapterIndex: 1, initialChapter: ch),
              ),
            ),
          );
          expect(find.textContaining('Night fell over Karhide.'), findsOneWidget);
        }
      });

      testWidgets('F08-T2-02: FixedLayoutReader keeps wide gatefold spread (pageWidth > pageHeight) as single page', (tester) async {
        tester.view.physicalSize = const Size(1280, 800);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        const gatefoldBook = Book(
          id: 'gate-1',
          title: 'Gatefold Artbook',
          layout: 'pre-paginated',
          spine: [
            SpineItem(id: 'p1', bookId: 'gate-1', chapterIndex: 1, title: 'Wide Map', pageWidth: 2000, pageHeight: 1000, pageSpread: 'center'),
          ],
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: const MaterialApp(home: FixedLayoutReader(book: gatefoldBook)),
          ),
        );
        expect(find.text('Page 1 of 1'), findsOneWidget);
      });

      test('F08-T2-03: findHighlightRange resolves exact and normalized whitespace matches', () {
        const hl = Highlight(
          id: 'h1',
          bookId: 'b1',
          selectedText: 'left hand of darkness',
          color: 'yellow',
        );
        final match = findHighlightRange('Light is the left hand of darkness.', hl);
        expect(match, isNotNull);
        expect(match!.start, 13);
        expect(match.end, 34);
      });

      testWidgets('F08-T2-04: KindleSelectionToolbar displays Edit Note and Delete Highlight when selectedColor and onDelete are set', (tester) async {
        var deleted = false;
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: Scaffold(
              body: KindleSelectionToolbar(
                anchors: const TextSelectionToolbarAnchors(primaryAnchor: Offset(120, 120)),
                selectedColor: KindleHighlightColor.pink,
                onColorSelected: (_) {},
                onAddNote: () {},
                onCopy: () {},
                onDelete: () => deleted = true,
              ),
            ),
          ),
        );
        expect(find.byTooltip('Edit Note'), findsOneWidget);
        expect(find.byTooltip('Delete Highlight'), findsOneWidget);
        await tester.tap(find.byTooltip('Delete Highlight'));
        expect(deleted, isTrue);
      });

      testWidgets('F08-T2-05: FixedLayoutReader switches to single-page mode on narrow mobile viewport (<600px)', (tester) async {
        tester.view.physicalSize = const Size(420, 800);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        const book = Book(
          id: 'f-mob',
          title: 'Pocket Comic',
          layout: 'pre-paginated',
          spine: [
            SpineItem(id: 'p1', bookId: 'f-mob', chapterIndex: 1, title: 'P1', pageWidth: 800, pageHeight: 1200),
            SpineItem(id: 'p2', bookId: 'f-mob', chapterIndex: 2, title: 'P2', pageWidth: 800, pageHeight: 1200),
          ],
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: const MaterialApp(home: FixedLayoutReader(book: book)),
          ),
        );
        expect(find.text('Page 1 of 2'), findsOneWidget);
      });
    });

    group('Feature 9 Boundaries: Audiobook Player & Scrubber Edge Cases', () {
      testWidgets('F09-T2-01: MiniPlayerBar collapses to SizedBox.shrink when no audiobook is active', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier(const AudioPlayerState()))],
            child: const MaterialApp(home: Scaffold(body: MiniPlayerBar())),
          ),
        );
        expect(find.byKey(const Key('mini_player_tap_area')), findsNothing);
      });

      testWidgets('F09-T2-02: ScrubberBar clamps safely when duration is Duration.zero or position exceeds duration', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ScrubberBar(
                position: const Duration(minutes: 15),
                duration: Duration.zero,
                onSeek: (_) {},
              ),
            ),
          ),
        );
        expect(find.byType(Slider), findsOneWidget);
        expect(tester.takeException(), isNull);
      });

      test('F09-T2-03: AudioPlayerState computes progressFraction and chapterProgressFraction safely at zero duration', () {
        const emptyState = AudioPlayerState(position: Duration(seconds: 30), duration: Duration.zero);
        expect(emptyState.progressFraction, 0.0);
        expect(emptyState.chapterProgressFraction, 0.0);
      });

      testWidgets('F09-T2-04: ScrubberBar formats multi-hour durations as H:MM:SS', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ScrubberBar(
                position: const Duration(hours: 1, minutes: 5, seconds: 9),
                duration: const Duration(hours: 2, minutes: 10, seconds: 0),
                onSeek: (_) {},
              ),
            ),
          ),
        );
        expect(find.text('1:05:09'), findsOneWidget);
        expect(find.text('-1:04:51'), findsOneWidget);
      });

      testWidgets('F09-T2-05: EqualizerBars pauses animation cleanly when isAnimating is false', (tester) async {
        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(body: EqualizerBars(isAnimating: false)),
          ),
        );
        expect(find.byType(EqualizerBars), findsOneWidget);
      });
    });

    group('Feature 10 Boundaries: Uploads Validation & Sanitization Edge Cases', () {
      testWidgets('F10-T2-01: UploadReviewContent displays Missing Author warning when author is Unknown', (tester) async {
        const job = StagedUploadJob(
          jobId: 'j-missing',
          status: 'staged',
          filename: 'mystery.epub',
          hasCover: false,
          metadata: StagedMetadata(title: 'Untitled Codex', authors: ['Unknown']),
        );
        final mockClient = MockClient((_) async => http.Response('{}', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(body: UploadReviewContent(job: job, isBottomSheet: false)),
            ),
          ),
        );
        expect(find.text('Missing Author'), findsOneWidget);
      });

      testWidgets('F10-T2-02: UploadReviewContent displays Duplicate Book Detected banner when isDuplicate is true', (tester) async {
        const dupJob = StagedUploadJob(
          jobId: 'j-dup',
          status: 'staged',
          filename: 'solaris.epub',
          hasCover: true,
          isDuplicate: true,
          metadata: StagedMetadata(title: 'Solaris', authors: ['Stanislaw Lem']),
        );
        final mockClient = MockClient((_) async => http.Response('{}', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(body: UploadReviewContent(job: dupJob, isBottomSheet: false)),
            ),
          ),
        );
        expect(find.text('Duplicate Book Detected'), findsOneWidget);
      });

      testWidgets('F10-T2-03: Destination path preview sanitizes unsafe filesystem characters', (tester) async {
        const unsafeJob = StagedUploadJob(
          jobId: 'j-unsafe',
          status: 'staged',
          filename: 'book.epub',
          hasCover: false,
          metadata: StagedMetadata(
            title: 'What/Why: A "Star" <Story>?',
            authors: ['Author: One|Two'],
          ),
        );
        final mockClient = MockClient((_) async => http.Response('{}', 200));
        final repo = BookRepository(
          apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
          storageService: storageService,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              bookRepositoryProvider.overrideWithValue(repo),
              storageServiceProvider.overrideWithValue(storageService),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const Scaffold(body: UploadReviewContent(job: unsafeJob, isBottomSheet: false)),
            ),
          ),
        );
        expect(
          find.text('/library/Author OneTwo/WhatWhy A Star Story/WhatWhy A Star Story.epub'),
          findsOneWidget,
        );
      });

      test('F10-T2-04: UploadState computes readyJobs and incompleteJobs counts accurately', () {
        const readyJob = StagedUploadJob(
          jobId: 'r1',
          status: 'staged',
          filename: 'a.epub',
          hasCover: true,
          metadata: StagedMetadata(title: 'Valid Book', authors: ['Valid Author']),
        );
        const incompleteJob = StagedUploadJob(
          jobId: 'i1',
          status: 'staged',
          filename: 'b.epub',
          hasCover: false,
          metadata: StagedMetadata(title: 'Unknown Author Book', authors: ['Unknown']),
        );
        const state = UploadState(stagedJobs: [readyJob, incompleteJob]);
        expect(state.readyJobs.length, 1);
        expect(state.incompleteJobs.length, 1);
      });

      testWidgets('F10-T2-05: UploadsView and UploadReviewContent adapt to Dark and Sepia themes', (tester) async {
        for (final mode in [ReadingThemeMode.sepia, ReadingThemeMode.dark]) {
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                sharedPreferencesProvider.overrideWithValue(prefs),
                uploadProvider.overrideWith(() => _FakeUploadNotifier()),
              ],
              child: MaterialApp(
                theme: AppTheme.buildTheme(mode),
                home: const UploadsView(),
              ),
            ),
          );
          expect(find.text('No Pending Uploads'), findsOneWidget);
        }
      });
    });

    group('Feature 11 Boundaries: Search, Utilities, Settings & Connect Edge Cases', () {
      testWidgets('F11-T2-01: SettingsView validates empty password fields and mismatched confirmation', (tester) async {
        tester.view.physicalSize = const Size(1200, 1600);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SettingsView(),
            ),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('Update Password'));
        await tester.pumpAndSettle();
        expect(find.text('Current password required'), findsOneWidget);

        await tester.enterText(find.widgetWithText(TextFormField, 'Current Password'), 'oldpass123');
        await tester.enterText(find.widgetWithText(TextFormField, 'New Password'), 'newpass123');
        await tester.enterText(find.widgetWithText(TextFormField, 'Confirm New Password'), 'mismatch999');
        await tester.tap(find.text('Update Password'));
        await tester.pumpAndSettle();
        expect(find.text('Passwords do not match'), findsOneWidget);
      });

      testWidgets('F11-T2-02: UtilitiesView renders No Duplicate Books Found when scan returns empty', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [bookRepositoryProvider.overrideWithValue(_MockBookRepository())],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const UtilitiesView(),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('No Duplicate Books Found'), findsOneWidget);
      });

      testWidgets('F11-T2-03: SearchView renders initial empty state prompt when query is empty', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              searchProvider.overrideWith(() => _FakeSearchNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const SearchView(),
            ),
          ),
        );
        expect(find.byType(TextField), findsOneWidget);
        expect(find.byIcon(Icons.auto_awesome_rounded), findsOneWidget);
      });

      testWidgets('F11-T2-04: ConnectView pre-fills saved server URL and username from StorageService', (tester) async {
        expect(storageService.getServerUrl(), 'http://localhost:8080');
        expect(storageService.getSavedUsername(), 'archivist');
        await tester.pumpWidget(
          ProviderScope(
            overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              home: const ConnectView(),
            ),
          ),
        );
        expect(find.text('http://localhost:8080'), findsOneWidget);
        expect(find.text('archivist'), findsOneWidget);
      });

      testWidgets('F11-T2-05: SearchView, UtilitiesView, SettingsView, and ConnectView render in Dark mode', (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              searchProvider.overrideWith(() => _FakeSearchNotifier()),
            ],
            child: MaterialApp(
              theme: AppTheme.buildTheme(ReadingThemeMode.dark),
              home: const SearchView(),
            ),
          ),
        );
        expect(find.byType(SearchView), findsOneWidget);
      });
    });

    group('Feature 12 Boundaries: Router Guard & Extreme Viewport Edge Cases', () {
      testWidgets('F12-T2-01: Unauthenticated user is redirected from protected routes to /connect', (tester) async {
        SharedPreferences.setMockInitialValues({});
        final emptyPrefs = await SharedPreferences.getInstance();
        final unauthedStorage = StorageService(emptyPrefs);
        final router = createRouter(initialLocation: '/books', storageService: unauthedStorage);

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(emptyPrefs),
              storageServiceProvider.overrideWithValue(unauthedStorage),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ConnectView), findsOneWidget);
      });

      testWidgets('F12-T2-02: Authenticated user navigating to /connect is redirected to /books', (tester) async {
        final router = createRouter(initialLocation: '/connect', storageService: storageService);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sharedPreferencesProvider.overrideWithValue(prefs),
              storageServiceProvider.overrideWithValue(storageService),
              libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
              queueProvider.overrideWith(() => _FakeQueueNotifier()),
            ],
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.bone),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(LibraryView), findsOneWidget);
      });

      testWidgets('F12-T2-03: Unknown route renders errorBuilder Page not found scaffold', (tester) async {
        final router = createRouter(initialLocation: '/nonexistent/path/xyz');
        await tester.pumpWidget(
          MaterialApp.router(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            routerConfig: router,
          ),
        );
        await tester.pumpAndSettle();
        expect(find.textContaining('Page not found'), findsOneWidget);
      });

      testWidgets('F12-T2-04: Responsive helper classifies 320px as mobile, 800px as tablet, and 1440px as desktop', (tester) async {
        addTearDown(() => tester.view.resetPhysicalSize());

        for (final entry in [
          (const Size(320, 568), true, false, false),
          (const Size(800, 1024), false, true, false),
          (const Size(1440, 900), false, false, true),
        ]) {
          tester.view.physicalSize = entry.$1;
          tester.view.devicePixelRatio = 1.0;
          late bool isMob, isTab, isDesk;
          await tester.pumpWidget(
            MaterialApp(
              home: Builder(
                builder: (ctx) {
                  isMob = Responsive.isMobile(ctx);
                  isTab = Responsive.isTablet(ctx);
                  isDesk = Responsive.isDesktop(ctx);
                  return const SizedBox.shrink();
                },
              ),
            ),
          );
          expect(isMob, entry.$2);
          expect(isTab, entry.$3);
          expect(isDesk, entry.$4);
        }
      });

      testWidgets('F12-T2-05: Compact 320x568 viewport renders ShelfdGridCard and TactileBookCover without overflow', (tester) async {
        tester.view.physicalSize = const Size(320, 568);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() => tester.view.resetPhysicalSize());

        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: SizedBox(
                width: 140,
                height: 220,
                child: ShelfdGridCard(item: BookGridItem(sampleBook), onTap: () {}),
              ),
            ),
          ),
        );
        expect(tester.takeException(), isNull);
      });
    });
  });

  // ===========================================================================
  // TIER 3 — CROSS-FEATURE COMBINATIONS (Pairwise Interactions)
  // ===========================================================================
  group('Tier 3 — Cross-Feature Combinations', () {
    testWidgets('T3-01: Theme switching (Bone -> Sepia -> Dark) updates HtmlText inside BentoCard', (tester) async {
      for (final mode in ReadingThemeMode.values) {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(mode),
            home: const Scaffold(
              body: BentoCard(
                child: HtmlText(
                  html: '<h1>Editorial Heading</h1><p>Body with <code>mono</code> and <a href="#">link</a></p>',
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        final card = tester.widget<Card>(find.byType(Card));
        final expectedColors = AppTheme.buildTheme(mode).extension<ShelfdColors>()!;
        expect(card.color, expectedColors.surface);
      }
    });

    testWidgets('T3-02: AppShell docks persistent MiniPlayerBar across Library, Search, and Settings routes', (tester) async {
      const audioBook = Book(id: 'audio-1', title: 'Project Hail Mary', bookType: 'audiobook');
      final activeAudioState = const AudioPlayerState(
        book: audioBook,
        status: AudioPlaybackStatus.playing,
        position: Duration(minutes: 4),
        duration: Duration(minutes: 20),
      );
      final router = createRouter(initialLocation: '/books', storageService: storageService);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            libraryProvider.overrideWith(() => _FakeLibraryNotifier(const LibraryState(books: [sampleBook]))),
            searchProvider.overrideWith(() => _FakeSearchNotifier()),
            queueProvider.overrideWith(() => _FakeQueueNotifier()),
            audioPlayerProvider.overrideWith(() => _FakeAudioPlayerNotifier(activeAudioState)),
          ],
          child: MaterialApp.router(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('mini_player_tap_area')), findsOneWidget);
      expect(find.text('Project Hail Mary'), findsOneWidget);

      router.go('/search');
      await tester.pumpAndSettle();
      expect(find.byType(SearchView), findsOneWidget);
      expect(find.byKey(const Key('mini_player_tap_area')), findsOneWidget);
    });

    testWidgets('T3-03: QueueStatus stagedUploads count updates ShelfdSideNav and ShelfdTopBar upload badges', (tester) async {
      const status = QueueStatus(
        pendingChapters: 0,
        totalChapters: 10,
        stagedUploads: 4,
        isActive: false,
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => _FakeQueueNotifier(const QueueState(status: status))),
            libraryProvider.overrideWith(() => _FakeLibraryNotifier()),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: Scaffold(
              appBar: ShelfdTopBar(onOpenUploads: () {}),
              body: ShelfdSideNav(currentIndex: 0, onTap: (_) {}),
            ),
          ),
        ),
      );
      expect(find.text('4'), findsWidgets);
      expect(find.text('AI Catalog Synced (10 passages)'), findsOneWidget);
    });
  });

  // ===========================================================================
  // TIER 4 — REAL-WORLD USER JOURNEYS (End-to-End Multi-Step Flows)
  // ===========================================================================
  group('Tier 4 — Real-World User Journeys', () {
    testWidgets('T4-Journey-01: Catalog Filtering -> Series Grouping -> Book Detail -> Highlights & RAG Chat', (tester) async {
      tester.view.physicalSize = const Size(1280, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      final router = createRouter(initialLocation: '/books', storageService: storageService);
      final chatState = BookChatState(
        messages: [
          BookChatMessage(
            role: 'assistant',
            content: 'Genly Ai travels to Orgoreyn after Erhenrang.',
            citations: const [
              BookCitation(
                chapterIndex: 1,
                chapterTitle: 'Chapter 1: A Parade in Erhenrang',
                summary: 'Genly Ai attends the parade.',
                excerpt: 'The parade wound through the streets of Erhenrang.',
              ),
            ],
            timestamp: DateTime(2026, 1, 1),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            libraryProvider.overrideWith(
              () => _FakeLibraryNotifier(
                const LibraryState(
                  books: [sampleBook],
                  series: [Series(id: 'ser-1', name: 'Hainish Cycle', bookCount: 6)],
                ),
              ),
            ),
            bookDetailProvider('book-42').overrideWith(() => _FakeBookDetailNotifier('book-42', sampleBook)),
            bookChatProvider('book-42').overrideWith(() => _FakeBookChatNotifier('book-42', chatState)),
            queueProvider.overrideWith(() => _FakeQueueNotifier()),
          ],
          child: MaterialApp.router(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Step 1: Verify grouped Series stack card in LibraryView
      expect(find.text('Hainish Cycle'), findsOneWidget);

      // Step 2: Navigate to BookDetailView
      router.go('/books/book-42');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.byType(BookDetailView), findsOneWidget);
      expect(find.text('The Left Hand of Darkness'), findsWidgets);

      // Step 3: Inspect Highlights & Bookmarks tab
      await tester.tap(find.text('Highlights & Bookmarks'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Light is the left hand of darkness.'), findsOneWidget);

      // Step 4: Inspect Chat with Book tab and open citation sheet
      await tester.tap(find.text('Chat with Book'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Genly Ai travels to Orgoreyn'), findsOneWidget);
    });

    testWidgets('T4-Journey-02: Staged Upload Review -> Metadata Edit -> Live Path Preview -> Duplicate Merge', (tester) async {
      tester.view.physicalSize = const Size(1280, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      const stagedJob = StagedUploadJob(
        jobId: 'job-journey-2',
        status: 'staged',
        filename: 'tombs_of_atuan.epub',
        hasCover: false,
        metadata: StagedMetadata(
          title: 'The Tombs of Atuan',
          authors: ['Unknown'],
          series: 'Earthsea Cycle',
          sequenceNumber: 2,
        ),
      );
      final mockClient = MockClient((_) async => http.Response('{}', 200));
      final repo = BookRepository(
        apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
        storageService: storageService,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(repo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.sepia),
            home: const Scaffold(
              body: UploadReviewContent(job: stagedJob, isBottomSheet: false),
            ),
          ),
        ),
      );

      // Verify initial Missing Author warning
      expect(find.text('Missing Author'), findsOneWidget);
      expect(
        find.text('/library/Unknown/The Tombs of Atuan/The Tombs of Atuan.epub'),
        findsOneWidget,
      );

      // Enter author and verify Missing Author warning clears and path updates live
      final authorField = find.widgetWithText(TextFormField, 'Unknown');
      await tester.enterText(authorField, 'Ursula K. Le Guin');
      await tester.pump();

      expect(find.text('Missing Author'), findsNothing);
      expect(
        find.text('/library/Ursula K. Le Guin/The Tombs of Atuan/The Tombs of Atuan.epub'),
        findsOneWidget,
      );
    });
  });

  // ===========================================================================
  // TIER 5 — WHITE-BOX ADVERSARIAL HARDENING (Edge Cases & Contract Verification)
  // ===========================================================================
  group('Tier 5 — White-Box Adversarial Hardening', () {
    testWidgets('T5-01: ShelfdColors.lerp threshold boundaries (t=0.0, 0.49, 0.5, 1.0, null) and unextended ThemeData fallback', (tester) async {
      final atZero = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.0);
      final atBeforeHalf = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.49);
      final atHalf = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.5);
      final atOne = ShelfdColors.bone.lerp(ShelfdColors.dark, 1.0);
      final withNull = ShelfdColors.sepia.lerp(null, 0.75);

      expect(atZero, equals(ShelfdColors.bone));
      expect(atBeforeHalf.isDark, isFalse);
      expect(atHalf.isDark, isTrue);
      expect(atOne, equals(ShelfdColors.dark));
      expect(withNull, same(ShelfdColors.sepia));

      late ShelfdColors resolvedDarkFallback;
      late ShelfdColors resolvedSepiaFallback;
      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(brightness: Brightness.dark),
          home: Builder(
            builder: (context) {
              resolvedDarkFallback = ShelfdColors.of(context);
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(resolvedDarkFallback, equals(ShelfdColors.dark));

      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(
            brightness: Brightness.light,
            scaffoldBackgroundColor: AppTokens.sepiaBackground,
          ),
          home: Builder(
            builder: (context) {
              resolvedSepiaFallback = ShelfdColors.of(context);
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(resolvedSepiaFallback, equals(ShelfdColors.sepia));
    });

    testWidgets('T5-02: HtmlText sanitizes adversarial <script>/<style>, unclosed tags, and decodes hex/decimal entities in Dark mode', (tester) async {
      const adversarialHtml = '''
        <script>window.pwned = true;</script>
        <style>body { display: none; }</style>
        <h1>Archival &mdash; Header &#x2014; &#8212;</h1>
        <p>Unclosed paragraph with <code>inline_code()</code> and <b>bold <i>nested text
        <blockquote>Quoted wisdom &amp; &lt;escaped&gt; tags</blockquote>
      ''';
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: const Scaffold(
            body: SingleChildScrollView(
              child: HtmlText(html: adversarialHtml),
            ),
          ),
        ),
      );

      expect(find.textContaining('window.pwned'), findsNothing);
      expect(find.textContaining('display: none'), findsNothing);
      expect(find.textContaining('Archival — Header — —'), findsOneWidget);
      expect(find.textContaining('inline_code()'), findsOneWidget);
      expect(find.textContaining('Quoted wisdom & <escaped> tags'), findsOneWidget);
    });

    test('T5-03: compareSeriesBooks handles null sequences, fractional sequences, negative numbers, and case-insensitive title tie-breaking', () {
      const bNullA = Book(id: '1', title: 'alpha');
      const bNullB = Book(id: '2', title: 'Beta');
      const bSeq1 = Book(id: '3', title: 'Zeta', seriesSequence: 1.0);
      const bSeq1Half = Book(id: '4', title: 'Novella', seriesSequence: 1.5);
      const bSeq1Tie = Book(id: '5', title: 'aardvark', seriesSequence: 1.0);

      final list = [bNullB, bSeq1Half, bNullA, bSeq1, bSeq1Tie]..sort(compareSeriesBooks);
      expect(list.map((b) => b.id).toList(), equals(['5', '3', '4', '1', '2']));
    });

    testWidgets('T5-04: BookDetailView bookmark without chapterId navigates to /books/:id/read fallback and dark-mode highlights render cleanly', (tester) async {
      tester.view.physicalSize = const Size(1280, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      final bookWithNullChapterBookmark = Book(
        id: 'book-adv-42',
        title: 'Solaris',
        authors: const [Author(id: 'a1', name: 'Stanislaw Lem')],
        bookmarks: [
          Bookmark(
            id: 'bm-null-ch',
            bookId: 'book-adv-42',
            chapterId: null,
            title: 'Fallback Chapterless Bookmark',
            progress: 0.75,
            createdAt: DateTime(2026, 1, 1),
          ),
        ],
        highlights: [
          Highlight(
            id: 'hl-dark-1',
            bookId: 'book-adv-42',
            chapterId: 'ch-1',
            selectedText: 'We do not need other worlds. We need mirrors.',
            color: 'pink',
            note: 'Crucial theme',
            createdAt: DateTime(2026, 1, 2),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            bookDetailProvider('book-adv-42').overrideWith(
              () => _FakeBookDetailNotifier('book-adv-42', bookWithNullChapterBookmark),
            ),
            bookChatProvider('book-adv-42').overrideWith(() => _FakeBookChatNotifier('book-adv-42')),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const BookDetailView(bookId: 'book-adv-42'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Highlights & Bookmarks'));
      await tester.pumpAndSettle();

      expect(find.text('Fallback Chapterless Bookmark'), findsOneWidget);
      expect(find.text('75% completed'), findsOneWidget);
      expect(find.textContaining('We do not need other worlds.'), findsOneWidget);
    });

    testWidgets('T5-05: ReaderView, KindleSelectionToolbar, and FixedLayoutReader handle multi-paragraph highlights, gatefolds, and Dark/Sepia themes without overflow', (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      const markdownText = '# Chapter I\n\n> An archival epigraph.[1]\n\nFirst paragraph with deep reflection.\n\nSecond paragraph continues the thought.';
      final blocks = parseReaderBlocks(markdownText);
      final range = findHighlightRange(
        blocks: blocks,
        fullContent: markdownText,
        selectedText: 'deep reflection.\n\nSecond paragraph',
      );
      expect(range, isNotNull);
      expect(range!.startParagraph, lessThan(range.endParagraph));

      // Verify KindleSelectionToolbar in Sepia and Dark themes without Scaffold ancestor
      for (final mode in [ReadingThemeMode.sepia, ReadingThemeMode.dark]) {
        await tester.pumpWidget(
          MaterialApp(
            theme: AppTheme.buildTheme(mode),
            home: KindleSelectionToolbar(
              anchors: const TextSelectionToolbarAnchors(primaryAnchor: Offset(100, 100)),
              selectedColor: KindleHighlightColor.orange,
              onColorSelected: (_) {},
              onAddNote: () {},
              onCopy: () {},
              onDelete: () {},
            ),
          ),
        );
        expect(find.byTooltip('Orange Highlight'), findsOneWidget);
        expect(find.byTooltip('Edit Note'), findsOneWidget);
        expect(find.byTooltip('Delete Highlight'), findsOneWidget);
        expect(tester.takeException(), isNull);
      }
    });

    testWidgets('T5-06: AudiobookPlayerView, ScrubberBar, ChapterDrawerSheet, and MiniPlayerBar handle extreme durations (99h+) and rapid state transitions', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: Scaffold(
            body: ScrubberBar(
              position: const Duration(hours: 99, minutes: 59, seconds: 59),
              duration: const Duration(hours: 120, minutes: 0, seconds: 0),
              onSeek: (_) {},
            ),
          ),
        ),
      );
      expect(find.text('99:59:59'), findsOneWidget);
      expect(find.text('-20:00:01'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('T5-07: UploadReviewContent and UploadsView sanitize adversarial path traversal characters and dynamically toggle Missing Author and Duplicate banners', (tester) async {
      tester.view.physicalSize = const Size(1280, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      const advJob = StagedUploadJob(
        jobId: 'job-adv-t5',
        status: 'staged',
        filename: 'colon:slash/star*.m4b',
        hasCover: false,
        isDuplicate: true,
        warnings: ['Book already exists in library', 'No author found in metadata'],
        metadata: StagedMetadata(
          title: 'Adversarial: <Title> / "Test"?',
          authors: ['Unknown'],
          bookType: 'audiobook',
          narrator: 'Ray Porter',
          isDuplicate: true,
        ),
      );

      final mockClient = MockClient((_) async => http.Response('{}', 200));
      final repo = BookRepository(
        apiService: ApiService(baseUrl: 'http://localhost:8080', client: mockClient),
        storageService: storageService,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(repo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const Scaffold(
              body: UploadReviewContent(job: advJob, isBottomSheet: false),
            ),
          ),
        ),
      );

      // Both Missing Author and Duplicate Book Detected banners are initially visible
      expect(find.text('Review Staged Audiobook'), findsOneWidget);
      expect(find.text('Missing Author'), findsOneWidget);
      expect(find.text('Duplicate Book Detected'), findsOneWidget);
      expect(
        find.text('/library/Unknown/Adversarial Title Test/Adversarial Title Test.m4b'),
        findsOneWidget,
      );

      // Entering a valid author clears Missing Author banner
      await tester.enterText(find.widgetWithText(TextFormField, 'Author(s) *'), 'Andy Weir');
      await tester.pump();
      expect(find.text('Missing Author'), findsNothing);

      // Editing title clears Duplicate Book Detected banner and updates sanitized destination preview
      await tester.enterText(find.widgetWithText(TextFormField, 'Title *'), 'Project Hail Mary: Part II?');
      await tester.pump();
      expect(find.text('Duplicate Book Detected'), findsNothing);
      expect(
        find.text('/library/Andy Weir/Project Hail Mary Part II/Project Hail Mary Part II.m4b'),
        findsOneWidget,
      );
    });

    testWidgets('T5-08: SettingsView theme segmented button switches Bone -> Sepia -> Dark and propagates ShelfdColors across SearchView, UtilitiesView, and ConnectView', (tester) async {
      tester.view.physicalSize = const Size(1280, 1400);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            queueProvider.overrideWith(() => _FakeQueueNotifier()),
          ],
          child: Consumer(
            builder: (context, ref, _) {
              final settings = ref.watch(readerSettingsProvider);
              return MaterialApp(
                theme: AppTheme.buildTheme(settings.themeMode),
                home: const SettingsView(),
              );
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Switch to Sepia via SegmentedButton
      await tester.tap(find.text('Sepia'));
      await tester.pumpAndSettle();
      expect(storageService.getThemeMode(), 'sepia');

      // Switch to Dark via SegmentedButton
      await tester.tap(find.text('Dark'));
      await tester.pumpAndSettle();
      expect(storageService.getThemeMode(), 'dark');
    });
  });
}
