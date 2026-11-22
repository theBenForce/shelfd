import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/repositories/audio_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/reader/fixed_layout_reader.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeReadalongPipBridge implements ReadalongPipBridge {
  @override
  final bool isSupported;

  bool _open = false;
  ReadalongPipCallbacks? callbacks;
  final List<ReadalongPipSnapshot> snapshots = <ReadalongPipSnapshot>[];
  int closeCount = 0;

  _FakeReadalongPipBridge({this.isSupported = true});

  @override
  bool get isOpen => _open;

  @override
  Future<bool> open(
    ReadalongPipSnapshot initial,
    ReadalongPipCallbacks callbacks,
  ) async {
    if (!isSupported) return false;
    _open = true;
    this.callbacks = callbacks;
    snapshots.add(initial);
    return true;
  }

  @override
  void update(ReadalongPipSnapshot snapshot) {
    if (!_open) return;
    snapshots.add(snapshot);
  }

  @override
  void close() {
    _open = false;
    closeCount++;
  }

  @override
  void dispose() {
    _open = false;
  }

  void simulateUserClose() {
    _open = false;
    callbacks?.onClosed();
  }
}

class _MockPipApiService extends ApiService {
  _MockPipApiService()
      : super(baseUrl: 'http://localhost:8080', token: 'pip-tok');

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
  Future<ReadalongManifest> getBookReadalong(String id) async =>
      ReadalongManifest(bookId: id, hasReadalong: true);

  @override
  String getAudioStreamUrl(String bookId) =>
      '$baseUrl/api/v1/audiobooks/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('buildFixedLayoutPageUrl', () {
    test('builds authenticated fixed-layout URL with fit=scale', () {
      final url = buildFixedLayoutPageUrl(
        baseUrl: 'https://shelf.local/',
        bookId: 'b1',
        chapterIdentifier: 'ch-1',
        token: 'tok-9',
      );
      expect(
        url,
        'https://shelf.local/api/v1/books/b1/chapters/ch-1/html?fit=scale&token=tok-9',
      );
    });

    test('uses fallbackOrigin when baseUrl is empty and omits empty token', () {
      final url = buildFixedLayoutPageUrl(
        baseUrl: '',
        bookId: 'b1',
        chapterIdentifier: '2',
        fallbackOrigin: 'http://localhost:3000/',
      );
      expect(
        url,
        'http://localhost:3000/api/v1/books/b1/chapters/2/html?fit=scale',
      );
    });
  });

  group('ReadalongPipNotifier', () {
    late ProviderContainer container;
    late SimulatedAudioPlayerEngine engine;
    late _FakeReadalongPipBridge pipBridge;
    late _MockPipApiService apiService;

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);
      apiService = _MockPipApiService();
      engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 120),
      );
      pipBridge = _FakeReadalongPipBridge(isSupported: true);

      container = ProviderContainer(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storage),
          apiServiceProvider.overrideWithValue(apiService),
          audioRepositoryProvider.overrideWithValue(
            AudioRepository(apiService: apiService, storageService: storage),
          ),
          audioPlayerEngineProvider.overrideWithValue(engine),
          mediaSessionBridgeProvider.overrideWithValue(NoopMediaSessionBridge()),
          readalongPipBridgeProvider.overrideWithValue(pipBridge),
        ],
      );
    });

    tearDown(() async {
      container.dispose();
      await engine.dispose();
    });

    test('syncs fixed-layout pages, highlights, and transport controls', () async {
      const book = Book(
        id: 'fxl-1',
        title: 'Four Silly Skeletons',
        coverUrl: '/api/v1/books/fxl-1/cover',
        authors: [Author(id: 'a1', name: 'Mark Sperring')],
        layout: 'pre-paginated',
        hasReadalong: true,
        durationSeconds: 120,
        spine: [
          SpineItem(
            id: 'p1',
            bookId: 'fxl-1',
            chapterIndex: 1,
            title: 'Page 1',
            href: 'OEBPS/p1.xhtml',
          ),
          SpineItem(
            id: 'p2',
            bookId: 'fxl-1',
            chapterIndex: 2,
            title: 'Page 2',
            href: 'OEBPS/p2.xhtml',
          ),
        ],
      );
      const manifest = ReadalongManifest(
        bookId: 'fxl-1',
        hasReadalong: true,
        hasMediaOverlays: true,
        segments: [
          ReadalongSegment(
            chapterIndex: 1,
            chapterHref: 'OEBPS/p1.xhtml',
            fragmentId: 'f1',
            text: 'First page line',
            clipBegin: 0,
            clipEnd: 5,
            globalBegin: 0,
            globalEnd: 5,
          ),
          ReadalongSegment(
            chapterIndex: 2,
            chapterHref: 'OEBPS/p2.xhtml',
            fragmentId: 'f2',
            text: 'Second page line',
            clipBegin: 0,
            clipEnd: 5,
            globalBegin: 5,
            globalEnd: 10,
          ),
        ],
      );

      final pipNotifier = container.read(readalongPipProvider.notifier);
      await pipNotifier.togglePopout(
        book: book,
        manifest: manifest,
        currentPage: book.spine.first,
      );

      expect(container.read(readalongPipProvider).isOpen, isTrue);
      expect(pipBridge.snapshots.first.isFixedLayout, isTrue);
      expect(
        pipBridge.snapshots.first.pageUrl,
        'http://localhost:8080/api/v1/books/fxl-1/chapters/p1/html?fit=scale&token=pip-tok',
      );

      await Future<void>.delayed(const Duration(milliseconds: 30));

      pipBridge.callbacks!.onSeek(const Duration(seconds: 6));
      await Future<void>.delayed(const Duration(milliseconds: 30));

      final updated = pipBridge.snapshots.last;
      expect(
        updated.pageUrl,
        'http://localhost:8080/api/v1/books/fxl-1/chapters/p2/html?fit=scale&token=pip-tok',
      );
      expect(updated.fragmentId, 'f2');
      expect(updated.chapterHref, 'OEBPS/p2.xhtml');

      for (final expectedSpeed in [1.25, 1.5, 2.0, 1.0]) {
        pipBridge.callbacks!.onCycleSpeed();
        await Future<void>.delayed(const Duration(milliseconds: 20));
        expect(container.read(audioPlayerProvider).speed, expectedSpeed);
        expect(pipBridge.snapshots.last.speed, expectedSpeed);
      }

      pipBridge.callbacks!.onTogglePlayPause();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).isPlaying, isTrue);

      pipBridge.callbacks!.onTogglePlayPause();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).isPlaying, isFalse);

      await pipNotifier.togglePopout(book: book, manifest: manifest);
      expect(container.read(readalongPipProvider).isOpen, isFalse);
      expect(pipBridge.closeCount, 1);
    });

    test('syncs reflowable segments and activeSegmentIndex', () async {
      const book = Book(
        id: 'reflow-1',
        title: 'Reflowable Tale',
        layout: 'reflowable',
        hasReadalong: true,
        durationSeconds: 120,
        spine: [
          SpineItem(
            id: 'c1',
            bookId: 'reflow-1',
            chapterIndex: 1,
            title: 'Chapter One',
          ),
        ],
      );
      const manifest = ReadalongManifest(
        bookId: 'reflow-1',
        hasReadalong: true,
        segments: [
          ReadalongSegment(
            chapterIndex: 1,
            fragmentId: 's1',
            text: 'Sentence one.',
            clipBegin: 0,
            clipEnd: 4,
            globalBegin: 0,
            globalEnd: 4,
          ),
          ReadalongSegment(
            chapterIndex: 1,
            fragmentId: 's2',
            text: 'Sentence two.',
            clipBegin: 4,
            clipEnd: 9,
            globalBegin: 4,
            globalEnd: 9,
          ),
        ],
      );

      final pipNotifier = container.read(readalongPipProvider.notifier);
      await pipNotifier.togglePopout(
        book: book,
        manifest: manifest,
        currentChapterIndex: 1,
        currentChapterTitle: 'Chapter One',
      );
      await Future<void>.delayed(const Duration(milliseconds: 30));

      expect(pipBridge.snapshots.first.isFixedLayout, isFalse);
      expect(pipBridge.snapshots.first.segments.length, 2);
      expect(pipBridge.snapshots.first.segments[0].text, 'Sentence one.');

      pipBridge.callbacks!.onSeek(const Duration(seconds: 6));
      await Future<void>.delayed(const Duration(milliseconds: 30));

      expect(pipBridge.snapshots.last.activeSegmentIndex, 1);
      expect(pipBridge.snapshots.last.chapterTitle, 'Chapter One');

      pipBridge.simulateUserClose();
      expect(container.read(readalongPipProvider).isOpen, isFalse);
    });
  });

  group('Read-Along PiP button visibility', () {
    Future<void> pumpFixedReader(
      WidgetTester tester, {
      required bool pipSupported,
    }) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);
      final apiService = _MockPipApiService();
      final engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 60),
      );
      addTearDown(engine.dispose);

      const book = Book(
        id: 'fxl-btn',
        title: 'Illustrated Story',
        layout: 'pre-paginated',
        hasReadalong: true,
        spine: [
          SpineItem(
            id: 'p1',
            bookId: 'fxl-btn',
            chapterIndex: 1,
            title: 'Page 1',
          ),
        ],
      );
      const manifest = ReadalongManifest(
        bookId: 'fxl-btn',
        hasReadalong: true,
      );

      await tester.binding.setSurfaceSize(const Size(1200, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            storageServiceProvider.overrideWithValue(storage),
            apiServiceProvider.overrideWithValue(apiService),
            audioPlayerEngineProvider.overrideWithValue(engine),
            mediaSessionBridgeProvider.overrideWithValue(NoopMediaSessionBridge()),
            readalongPipBridgeProvider.overrideWithValue(
              _FakeReadalongPipBridge(isSupported: pipSupported),
            ),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const FixedLayoutReader(
              book: book,
              initialReadalongManifest: manifest,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
    }

    testWidgets('shows readalong-pip-button when Document PiP is supported', (tester) async {
      await pumpFixedReader(tester, pipSupported: true);
      expect(find.byKey(const Key('readalong-pip-button')), findsOneWidget);
    });

    testWidgets('hides readalong-pip-button when Document PiP is unsupported', (tester) async {
      await pumpFixedReader(tester, pipSupported: false);
      expect(find.byKey(const Key('readalong-pip-button')), findsNothing);
    });
  });
}
