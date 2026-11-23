import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/audiobook/audiobook_player_view.dart';
import 'package:shelf/ui/features/audiobook/widgets/mini_player_bar.dart';
import 'package:shelf/ui/features/reader/fixed_layout_reader.dart';
import 'package:shelf/ui/features/reader/html_view.dart';
import 'package:shelf/ui/features/reader/reader_view.dart';
import 'package:shelf/ui/state/providers.dart';

class _StubBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  _StubBookDetailNotifier(this.mockBook) : super(mockBook.id);

  @override
  BookDetailState build() => BookDetailState(book: mockBook, isLoading: false);

  @override
  Future<void> loadBook() async {}
}

class _MockReadalongApiService extends ApiService {
  _MockReadalongApiService(this.manifest) : super(baseUrl: 'http://localhost:8080');

  final ReadalongManifest manifest;

  @override
  Future<ReadalongManifest> getBookReadalong(String id) async => manifest;

  @override
  Future<List<AudioChapter>> getAudioChapters(String bookId) async => const [];

  @override
  Future<AudiobookProgress> getAudiobookProgress(String bookId) async =>
      AudiobookProgress(bookId: bookId, positionSeconds: 0.0, speed: 1.0);

  @override
  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async =>
      AudiobookProgress(
        bookId: bookId,
        positionSeconds: positionSeconds,
        speed: speed,
        isCompleted: isCompleted,
      );

  @override
  String getAudioStreamUrl(String bookId) =>
      'http://localhost:8080/api/v1/audiobooks/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    resetHtmlFrameHighlightsForTest();
  });

  group('ReadalongManifest & Book read-along model', () {
    test('parses has_readalong and SMIL segments accurately', () {
      final book = Book.fromJson({
        'id': 'book-1',
        'title': 'Four Silly Skeletons',
        'book_type': 'ebook',
        'has_readalong': true,
        'duration_seconds': 9.0,
        'authors': [
          {'id': 'a1', 'name': 'Mark Sperring & Sue Hendra'},
        ],
      });

      expect(book.isAudiobook, isFalse);
      expect(book.hasReadalong, isTrue);
      expect(book.canReadAlong, isTrue);
      expect(book.durationSeconds, 9.0);

      final manifest = ReadalongManifest.fromJson({
        'book_id': 'book-1',
        'has_readalong': true,
        'has_media_overlays': true,
        'active_class': '-epub-media-overlay-active',
        'playback_active_class': '-epub-media-overlay-playing',
        'duration_seconds': 9.0,
        'segments': [
          {
            'chapter_index': 1,
            'fragment_id': 'w1',
            'text': 'Four silly skeletons',
            'clip_begin': 0.0,
            'clip_end': 4.0,
            'global_begin': 0.0,
            'global_end': 4.0,
          },
          {
            'chapter_index': 2,
            'fragment_id': 'w2',
            'text': 'dancing in the night!',
            'clip_begin': 0.0,
            'clip_end': 5.0,
            'global_begin': 4.0,
            'global_end': 9.0,
          },
        ],
      });

      expect(manifest.segmentAt(1.5)?.fragmentId, 'w1');
      expect(manifest.segmentAt(1.5)?.text, 'Four silly skeletons');
      expect(manifest.segmentAt(5.2)?.fragmentId, 'w2');
      expect(manifest.segmentAt(5.2)?.chapterIndex, 2);
    });
  });

  group('FixedLayoutReader Read-Along playback, highlighting, and page turning', () {
    testWidgets('plays audio, highlights spoken phrases without subtitle bubble, and turns pages automatically',
        (tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();

      final manifest = ReadalongManifest(
        bookId: 'b-skeletons',
        hasReadalong: true,
        hasMediaOverlays: true,
        durationSeconds: 9.0,
        segments: const [
          ReadalongSegment(
            chapterIndex: 1,
            chapterHref: 'OEBPS/xhtml/page01.xhtml',
            fragmentId: 'w1',
            text: 'Four silly skeletons',
            clipBegin: 0.0,
            clipEnd: 4.0,
            globalBegin: 0.0,
            globalEnd: 4.0,
          ),
          ReadalongSegment(
            chapterIndex: 2,
            chapterHref: 'OEBPS/xhtml/page02.xhtml',
            fragmentId: 'w2',
            text: 'dancing in the night!',
            clipBegin: 0.0,
            clipEnd: 5.0,
            globalBegin: 4.0,
            globalEnd: 9.0,
          ),
        ],
      );

      final book = Book(
        id: 'b-skeletons',
        title: 'Four Silly Skeletons',
        layout: 'pre-paginated',
        bookType: 'ebook',
        hasReadalong: true,
        durationSeconds: 9.0,
        authors: const [Author(id: 'a1', name: 'Mark Sperring & Sue Hendra')],
        spine: const [
          SpineItem(
            id: 'ch1',
            bookId: 'b-skeletons',
            chapterIndex: 1,
            title: 'Page 1',
            href: 'OEBPS/xhtml/page01.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'ch2',
            bookId: 'b-skeletons',
            chapterIndex: 2,
            title: 'Page 2',
            href: 'OEBPS/xhtml/page02.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
        ],
      );

      final engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 9),
      );
      final api = _MockReadalongApiService(manifest);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(engine),
          ],
          child: MaterialApp(
            home: FixedLayoutReader(
              book: book,
              initialReadalongManifest: manifest,
            ),
          ),
        ),
      );
      await tester.pump();

      // Starts on Page 1 of 2
      expect(find.text('Page 1 of 2'), findsOneWidget);
      expect(find.byKey(const Key('readalong-play-pause-button')), findsOneWidget);

      // Tap Play Read-Along
      await tester.tap(find.byKey(const Key('readalong-play-pause-button')));
      await tester.pump();

      // Seek to 1.0s -> Chapter 1 fragment "w1" is highlighted on Page 1 without bottom subtitle bubble
      await engine.seek(const Duration(seconds: 1));
      await tester.pump();
      expect(find.byKey(const Key('readalong-overlay-pill')), findsNothing);
      expect(activeFragmentForViewKey('fixed_b-skeletons_1'), 'w1');
      expect(activeFragmentForViewKey('fixed_b-skeletons_2'), '');
      expect(find.text('Page 1 of 2'), findsOneWidget);

      // Seek to 5.0s -> Advances to Chapter 2 fragment "w2" and auto-turns to Page 2
      await engine.seek(const Duration(seconds: 5));
      await tester.pump();
      expect(find.byKey(const Key('readalong-overlay-pill')), findsNothing);
      expect(activeFragmentForViewKey('fixed_b-skeletons_1'), '');
      expect(activeFragmentForViewKey('fixed_b-skeletons_2'), 'w2');
      expect(find.text('Page 2 of 2'), findsOneWidget);

      await engine.stop();
      await tester.pump();
    });

    testWidgets('scopes word highlights to only the active page on a two-page facing spread',
        (tester) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() => tester.view.resetPhysicalSize());

      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();

      // Both Page 2 (left) and Page 3 (right) reuse identical fragment IDs ('w1', 'w9')
      final manifest = ReadalongManifest(
        bookId: 'b-spidey',
        hasReadalong: true,
        hasMediaOverlays: true,
        durationSeconds: 12.0,
        segments: const [
          ReadalongSegment(
            chapterIndex: 2,
            chapterHref: 'OEBPS/xhtml/page02.xhtml',
            fragmentId: 'w9',
            text: 'Junkyard.',
            clipBegin: 0.0,
            clipEnd: 4.0,
            globalBegin: 0.0,
            globalEnd: 4.0,
          ),
          ReadalongSegment(
            chapterIndex: 3,
            chapterHref: 'OEBPS/xhtml/page03.xhtml',
            fragmentId: 'w9',
            text: 'Goblin.',
            clipBegin: 4.0,
            clipEnd: 8.0,
            globalBegin: 4.0,
            globalEnd: 8.0,
          ),
          ReadalongSegment(
            chapterIndex: 4,
            chapterHref: 'OEBPS/xhtml/page04.xhtml',
            fragmentId: 'w1',
            text: 'Spidey!',
            clipBegin: 8.0,
            clipEnd: 12.0,
            globalBegin: 8.0,
            globalEnd: 12.0,
          ),
        ],
      );

      final book = Book(
        id: 'b-spidey',
        title: 'Spidey and His Amazing Friends',
        layout: 'pre-paginated',
        bookType: 'ebook',
        hasReadalong: true,
        durationSeconds: 12.0,
        spine: const [
          SpineItem(
            id: 'p1',
            bookId: 'b-spidey',
            chapterIndex: 1,
            title: 'Cover',
            href: 'OEBPS/xhtml/cover.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'p2',
            bookId: 'b-spidey',
            chapterIndex: 2,
            title: 'Page 2',
            href: 'OEBPS/xhtml/page02.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'p3',
            bookId: 'b-spidey',
            chapterIndex: 3,
            title: 'Page 3',
            href: 'OEBPS/xhtml/page03.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'p4',
            bookId: 'b-spidey',
            chapterIndex: 4,
            title: 'Page 4',
            href: 'OEBPS/xhtml/page04.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'p5',
            bookId: 'b-spidey',
            chapterIndex: 5,
            title: 'Page 5',
            href: 'OEBPS/xhtml/page05.xhtml',
            pageWidth: 600,
            pageHeight: 800,
          ),
        ],
      );

      final engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 12),
      );
      final api = _MockReadalongApiService(manifest);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(engine),
          ],
          child: MaterialApp(
            home: FixedLayoutReader(
              book: book,
              initialChapterIdentifier: 'p2',
              initialReadalongManifest: manifest,
            ),
          ),
        ),
      );
      await tester.pump();

      // Two-page facing spread: Pages 2–3 of 5
      expect(find.text('Pages 2–3 of 5'), findsOneWidget);
      expect(find.byKey(const Key('html-frame-fixed_b-spidey_2')), findsOneWidget);
      expect(find.byKey(const Key('html-frame-fixed_b-spidey_3')), findsOneWidget);

      await tester.tap(find.byKey(const Key('readalong-play-pause-button')));
      await tester.pump();

      // At 2.0s (Page 2): ONLY left page (page 2) highlights 'w9'; right page (page 3) is empty
      await engine.seek(const Duration(seconds: 2));
      await tester.pump();
      expect(find.byKey(const Key('readalong-overlay-pill')), findsNothing);
      expect(activeFragmentForViewKey('fixed_b-spidey_2'), 'w9');
      expect(activeFragmentForViewKey('fixed_b-spidey_3'), '');
      expect(lastReadalongHighlight?.chapterHref, 'OEBPS/xhtml/page02.xhtml');

      // At 5.0s (Page 3): stays on Pages 2–3 spread, clears left page (page 2), highlights ONLY right page (page 3)
      await engine.seek(const Duration(seconds: 5));
      await tester.pump();
      expect(find.text('Pages 2–3 of 5'), findsOneWidget);
      expect(activeFragmentForViewKey('fixed_b-spidey_2'), '');
      expect(activeFragmentForViewKey('fixed_b-spidey_3'), 'w9');
      expect(lastReadalongHighlight?.chapterHref, 'OEBPS/xhtml/page03.xhtml');

      // At 9.0s (Page 4): auto-turns to Pages 4–5 spread and highlights ONLY left page (page 4)
      await engine.seek(const Duration(seconds: 9));
      await tester.pump();
      expect(find.text('Pages 4–5 of 5'), findsOneWidget);
      expect(activeFragmentForViewKey('fixed_b-spidey_4'), 'w1');
      expect(activeFragmentForViewKey('fixed_b-spidey_5'), '');

      await engine.stop();
      await tester.pump();
    });

    testWidgets(
      'toggles seamlessly between Audiobook player, Read-Along reader, and MiniPlayerBar while preserving playback position',
      (tester) async {
        tester.view.physicalSize = const Size(900, 1000);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        SharedPreferences.setMockInitialValues({});
        final prefs = await SharedPreferences.getInstance();

        final manifest = ReadalongManifest(
          bookId: 'b-toggle',
          hasReadalong: true,
          hasMediaOverlays: true,
          durationSeconds: 10.0,
          segments: const [
            ReadalongSegment(
              chapterIndex: 1,
              chapterHref: 'OEBPS/xhtml/page01.xhtml',
              fragmentId: 'w1',
              text: 'Page one text',
              clipBegin: 0.0,
              clipEnd: 4.0,
              globalBegin: 0.0,
              globalEnd: 4.0,
            ),
            ReadalongSegment(
              chapterIndex: 2,
              chapterHref: 'OEBPS/xhtml/page02.xhtml',
              fragmentId: 'w2',
              text: 'Page two text',
              clipBegin: 4.0,
              clipEnd: 10.0,
              globalBegin: 4.0,
              globalEnd: 10.0,
            ),
          ],
        );

        final book = Book(
          id: 'b-toggle',
          title: 'Four Silly Skeletons',
          layout: 'pre-paginated',
          bookType: 'ebook',
          hasReadalong: true,
          durationSeconds: 10.0,
          authors: const [Author(id: 'a1', name: 'Sue Hendra')],
          spine: const [
            SpineItem(
              id: 'ch1',
              bookId: 'b-toggle',
              chapterIndex: 1,
              title: 'Page 1',
              href: 'OEBPS/xhtml/page01.xhtml',
              pageWidth: 600,
              pageHeight: 800,
            ),
            SpineItem(
              id: 'ch2',
              bookId: 'b-toggle',
              chapterIndex: 2,
              title: 'Page 2',
              href: 'OEBPS/xhtml/page02.xhtml',
              pageWidth: 600,
              pageHeight: 800,
            ),
          ],
        );

        final engine = SimulatedAudioPlayerEngine(
          initialDuration: const Duration(seconds: 10),
        );
        final api = _MockReadalongApiService(manifest);

        final container = ProviderContainer(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(engine),
            bookDetailProvider('b-toggle').overrideWith(() => _StubBookDetailNotifier(book)),
          ],
        );
        addTearDown(() => container.dispose());

        final router = GoRouter(
          initialLocation: '/books',
          routes: [
            GoRoute(
              path: '/books',
              builder: (context, state) => const Scaffold(
                body: Center(child: Text('Library')),
                bottomNavigationBar: MiniPlayerBar(),
              ),
            ),
            GoRoute(
              path: '/books/:bookId/listen',
              builder: (context, state) => AudiobookPlayerView(
                bookId: state.pathParameters['bookId']!,
              ),
            ),
            GoRoute(
              path: '/books/:bookId/read',
              builder: (context, state) => ReaderView(
                bookId: state.pathParameters['bookId']!,
              ),
            ),
          ],
        );

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp.router(
              theme: AppTheme.buildTheme(ReadingThemeMode.dark),
              routerConfig: router,
            ),
          ),
        );
        await tester.pump();

        // 1. Start playing the readalong book as an audiobook and advance to 5.0s (Page 2)
        router.push('/books/b-toggle/listen');
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        expect(find.byType(AudiobookPlayerView), findsOneWidget);
        expect(find.byKey(const Key('audio_player_mode_toggle')), findsOneWidget);
        expect(find.byKey(const Key('switch_to_readalong_button')), findsOneWidget);

        await tester.tap(find.byKey(const Key('audio_player_play_pause_button')));
        await tester.pump();
        await engine.seek(const Duration(seconds: 5));
        await tester.pump();

        expect(container.read(audioPlayerProvider).isPlaying, isTrue);
        expect(container.read(audioPlayerProvider).position, const Duration(seconds: 5));
        expect(container.read(audioPlayerProvider).playbackMode, AudioPlaybackMode.audiobook);

        // 2. Toggle from Audiobook -> Read-Along
        await tester.tap(find.byKey(const Key('switch_to_readalong_button')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        // Opens Read-Along directly on Page 2 at 5.0s while keeping playback running
        expect(find.byType(FixedLayoutReader), findsOneWidget);
        expect(find.text('Page 2 of 2'), findsOneWidget);
        expect(find.byKey(const Key('switch_to_audiobook_button')), findsOneWidget);
        expect(container.read(audioPlayerProvider).isPlaying, isTrue);
        expect(container.read(audioPlayerProvider).position.inSeconds, greaterThanOrEqualTo(5));
        expect(container.read(audioPlayerProvider).playbackMode, AudioPlaybackMode.readalong);

        // 3. Toggle from Read-Along -> Audiobook
        await tester.tap(find.byKey(const Key('switch_to_audiobook_button')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        expect(find.byType(AudiobookPlayerView), findsOneWidget);
        expect(container.read(audioPlayerProvider).isPlaying, isTrue);
        expect(container.read(audioPlayerProvider).position.inSeconds, greaterThanOrEqualTo(5));
        expect(container.read(audioPlayerProvider).playbackMode, AudioPlaybackMode.audiobook);

        // 4. Minimize back to Library and verify MiniPlayerBar mode toggle switches to Read-Along
        await tester.tap(find.byKey(const Key('audio_player_back_button')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        expect(find.text('Library'), findsOneWidget);
        expect(find.byKey(const Key('mini_player_mode_toggle')), findsOneWidget);

        await tester.tap(find.byKey(const Key('mini_player_mode_toggle')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        expect(find.byType(FixedLayoutReader), findsOneWidget);
        expect(container.read(audioPlayerProvider).playbackMode, AudioPlaybackMode.readalong);
        expect(container.read(audioPlayerProvider).position.inSeconds, greaterThanOrEqualTo(5));

        await engine.stop();
        // Stop AudioPlayerNotifier to cancel its _syncTimer (periodic 10s progress sync)
        // which is started by play() but never cancelled when toggling away from player view.
        await container.read(audioPlayerProvider.notifier).stop();
        await tester.pump();
      },
    );
  });
}
