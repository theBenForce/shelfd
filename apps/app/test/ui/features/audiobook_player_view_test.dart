import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/repositories/audio_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/features/audiobook/audiobook_player_view.dart';
import 'package:shelf/ui/features/audiobook/widgets/chapter_drawer.dart';
import 'package:shelf/ui/features/audiobook/widgets/mini_player_bar.dart';
import 'package:shelf/ui/features/audiobook/widgets/scrubber_bar.dart';
import 'package:shelf/ui/features/book_detail/book_detail_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  _MockBookDetailNotifier(this.mockBook) : super('book-audio-1');

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }
}

class MockApiService extends ApiService {
  MockApiService() : super(baseUrl: 'http://localhost:8080');

  List<AudioChapter> mockChapters = [];
  AudiobookProgress? mockProgress;

  @override
  Future<List<AudioChapter>> getAudioChapters(String bookId) async {
    return mockChapters;
  }

  @override
  Future<AudiobookProgress> getAudiobookProgress(String bookId) async {
    if (mockProgress != null) return mockProgress!;
    return AudiobookProgress(bookId: bookId, positionSeconds: 120.0, speed: 1.0);
  }

  @override
  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async {
    final progress = AudiobookProgress(
      bookId: bookId,
      positionSeconds: positionSeconds,
      speed: speed,
      isCompleted: isCompleted,
    );
    mockProgress = progress;
    return progress;
  }

  @override
  String getAudioStreamUrl(String bookId) => 'http://localhost:8080/api/books/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  final testAudioBook = Book(
    id: 'book-audio-1',
    title: 'Project Hail Mary',
    synopsis: 'Ryland Grace is the sole survivor on a desperate mission.',
    authors: const [Author(id: 'auth-1', name: 'Andy Weir')],
    bookType: 'audiobook',
    durationSeconds: 1200.0,
    bitrateKbps: 64,
    audioChapters: const [
      AudioChapter(
        id: 'chap-1',
        bookId: 'book-audio-1',
        chapterIndex: 0,
        title: 'Prologue: The Arrival',
        startOffsetSec: 0.0,
        durationSec: 300.0,
      ),
      AudioChapter(
        id: 'chap-2',
        bookId: 'book-audio-1',
        chapterIndex: 1,
        title: 'Chapter 1: Into the Deep',
        startOffsetSec: 300.0,
        durationSec: 900.0,
      ),
    ],
    files: const [
      BookFile(
        id: 'f-1',
        bookId: 'book-audio-1',
        fileType: 'm4a',
        filePath: 'Project Hail Mary.m4a',
        fileSizeBytes: 244500000,
        durationSeconds: 1200.0,
        bitrateKbps: 64,
        mimeType: 'audio/mp4',
      ),
      BookFile(
        id: 'f-2',
        bookId: 'book-audio-1',
        fileType: 'epub',
        filePath: 'Project Hail Mary.epub',
        fileSizeBytes: 1800000,
        mimeType: 'application/epub+zip',
      ),
    ],
  );

  group('ScrubberBar widget tests', () {
    testWidgets('renders timestamps and scrubber slider', (tester) async {
      Duration? seeked;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            backgroundColor: AppTokens.obsidianBackground,
            body: ScrubberBar(
              position: const Duration(minutes: 5),
              duration: const Duration(minutes: 20),
              onSeek: (d) => seeked = d,
            ),
          ),
        ),
      );

      expect(find.text('05:00'), findsOneWidget);
      expect(find.text('-15:00'), findsOneWidget);
      expect(find.byType(Slider), findsOneWidget);

      await tester.tap(find.byType(Slider));
      await tester.pump();
      expect(seeked, isNotNull);
    });
  });

  group('ChapterDrawerSheet widget tests', () {
    testWidgets('renders tabs, chapters, equalizer animation and triggers callbacks', (tester) async {
      tester.view.physicalSize = const Size(800, 1200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      int? selectedChapter;
      bool bookmarkAdded = false;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                key: const Key('open_sheet_btn'),
                onPressed: () => showModalBottomSheet(
                  context: context,
                  isScrollControlled: true,
                  backgroundColor: Colors.transparent,
                  builder: (ctx) => ChapterDrawerSheet(
                    book: testAudioBook,
                    chapters: testAudioBook.audioChapters,
                    currentChapterIndex: 0,
                    currentPosition: const Duration(seconds: 45),
                    isPlaying: false,
                    onChapterSelected: (idx) => selectedChapter = idx,
                    onSeekToPosition: (_) {},
                    onSetSleepTimer: (_) {},
                    onAddBookmark: () => bookmarkAdded = true,
                  ),
                ),
                child: const Text('Open Sheet'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.byKey(const Key('open_sheet_btn')));
      await tester.pumpAndSettle();

      expect(find.text('Prologue: The Arrival'), findsOneWidget);
      expect(find.text('Chapter 1: Into the Deep'), findsOneWidget);

      // Switch to Sleep Timer tab
      await tester.tap(find.byKey(const Key('chapter_drawer_tab_2')));
      await tester.pumpAndSettle();
      expect(find.text('15 minutes'), findsOneWidget);
      expect(find.text('30 minutes'), findsOneWidget);
      expect(find.text('End of Chapter'), findsOneWidget);

      // Switch to Bookmarks tab
      await tester.tap(find.byKey(const Key('chapter_drawer_tab_1')));
      await tester.pumpAndSettle();
      expect(find.text('Add Audio Bookmark at Current Time'), findsOneWidget);

      // Tap Add Bookmark (which calls callback and pops the modal)
      await tester.tap(find.text('Add Audio Bookmark at Current Time'));
      await tester.pumpAndSettle();
      expect(bookmarkAdded, isTrue);

      // Re-open sheet to select chapter 1 (which calls callback and pops the modal)
      await tester.tap(find.byKey(const Key('open_sheet_btn')));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Chapter 1: Into the Deep'));
      await tester.pumpAndSettle();
      expect(selectedChapter, equals(1));
    });
  });

  group('AudiobookPlayerView tests', () {
    testWidgets('renders audiobook player screen with controls and Stitch tokens', (tester) async {
      tester.view.physicalSize = const Size(1200, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storageService = StorageService(prefs);
      final apiService = MockApiService();
      apiService.mockChapters = testAudioBook.audioChapters;
      final audioRepo = AudioRepository(apiService: apiService, storageService: storageService);
      final engine = SimulatedAudioPlayerEngine(initialDuration: const Duration(seconds: 1200));
      addTearDown(() => engine.dispose());

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            apiServiceProvider.overrideWithValue(apiService),
            audioRepositoryProvider.overrideWithValue(audioRepo),
            audioPlayerEngineProvider.overrideWithValue(engine),
            bookDetailProvider(testAudioBook.id).overrideWith(() => _MockBookDetailNotifier(testAudioBook)),
          ],
          child: MaterialApp(
            home: AudiobookPlayerView(bookId: testAudioBook.id),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Project Hail Mary'), findsWidgets);
      expect(find.text('Andy Weir'), findsWidgets);
      expect(find.byKey(const Key('audio_player_play_pause_button')), findsOneWidget);
      expect(find.byKey(const Key('audio_player_skip_back_button')), findsOneWidget);
      expect(find.byKey(const Key('audio_player_skip_forward_button')), findsOneWidget);
      expect(find.byKey(const Key('audio_player_speed_button')), findsOneWidget);
      expect(find.byKey(const Key('audio_player_chapters_menu_button')), findsOneWidget);

      // Tap Play/Pause Orb to start
      await tester.tap(find.byKey(const Key('audio_player_play_pause_button')));
      await tester.pump();

      // Tap Skip 30s
      await tester.tap(find.byKey(const Key('audio_player_skip_forward_button')));
      await tester.pump();

      // Tap Speed Picker
      await tester.tap(find.byKey(const Key('audio_player_speed_button')));
      await tester.pumpAndSettle();
      expect(find.text('Playback Speed'), findsOneWidget);
      expect(find.byKey(const Key('speed_chip_1.5x')), findsOneWidget);

      await tester.tap(find.byKey(const Key('speed_chip_1.5x')));
      await tester.pumpAndSettle();

      // Stop engine before completing test
      await engine.pause();
      await tester.pump();
    });
  });

  group('MiniPlayerBar widget tests', () {
    testWidgets('displays active audiobook and allows playback control and dismissal', (tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storageService = StorageService(prefs);
      final apiService = MockApiService();
      final audioRepo = AudioRepository(apiService: apiService, storageService: storageService);
      final engine = SimulatedAudioPlayerEngine(initialDuration: const Duration(seconds: 1200));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            apiServiceProvider.overrideWithValue(apiService),
            audioRepositoryProvider.overrideWithValue(audioRepo),
            audioPlayerEngineProvider.overrideWithValue(engine),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: Column(
                children: [
                  Expanded(child: Center(child: Text('Main Content'))),
                  MiniPlayerBar(),
                ],
              ),
            ),
          ),
        ),
      );

      await tester.pump();
      // Initially idle, mini player is hidden
      expect(find.byKey(const Key('mini_player_tap_area')), findsNothing);

      // Put player into ready/playing state
      final container = ProviderScope.containerOf(tester.element(find.byType(Scaffold)));
      await container.read(audioPlayerProvider.notifier).loadBook(testAudioBook);
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('mini_player_tap_area')), findsOneWidget);
      expect(find.text('Project Hail Mary'), findsOneWidget);
      expect(find.byKey(const Key('mini_player_play_pause')), findsOneWidget);

      // Tap play/pause in mini-player
      await tester.tap(find.byKey(const Key('mini_player_play_pause')));
      await tester.pump();

      // Tap dismiss
      await tester.tap(find.byKey(const Key('mini_player_close')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('mini_player_tap_area')), findsNothing);
    });
  });

  group('BookDetailView Dual CTA and Associated Files tests', () {
    testWidgets('renders dual CTAs and associated files section for multi-format book', (tester) async {
      tester.view.physicalSize = const Size(1200, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storageService = StorageService(prefs);
      final apiService = MockApiService();
      final audioRepo = AudioRepository(apiService: apiService, storageService: storageService);
      final engine = SimulatedAudioPlayerEngine(initialDuration: const Duration(seconds: 1200));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storageService),
            apiServiceProvider.overrideWithValue(apiService),
            audioRepositoryProvider.overrideWithValue(audioRepo),
            audioPlayerEngineProvider.overrideWithValue(engine),
            bookDetailProvider(testAudioBook.id).overrideWith(() => _MockBookDetailNotifier(testAudioBook)),
          ],
          child: MaterialApp(
            home: BookDetailView(bookId: testAudioBook.id),
          ),
        ),
      );

      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      // Check Dual CTAs
      expect(find.byKey(const Key('listen_hero_button')), findsOneWidget);
      expect(find.byKey(const Key('read_hero_button')), findsOneWidget);
      expect(find.byKey(const Key('listen_appbar_button')), findsOneWidget);
      expect(find.text('Listen Audiobook'), findsOneWidget);
      expect(find.text('Read EPUB'), findsOneWidget);

      // Check Associated Files Section in Overview
      expect(find.text('Associated Formats & Files'), findsOneWidget);
      expect(find.text('Project Hail Mary.m4a'), findsOneWidget);
      expect(find.text('Project Hail Mary.epub'), findsOneWidget);
      expect(find.text('M4A AUDIOBOOK'), findsOneWidget);
      expect(find.text('EPUB'), findsOneWidget);
    });
  });
}
