import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/repositories/audio_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/state/providers.dart';

class _PlaybackStateRecord {
  final AudioPlaybackStatus status;
  final Duration position;
  final Duration duration;
  final double speed;

  const _PlaybackStateRecord({
    required this.status,
    required this.position,
    required this.duration,
    required this.speed,
  });
}

class _FakeMediaSessionBridge implements MediaSessionBridge {
  MediaSessionActionHandlers? boundHandlers;
  final List<AudioMediaMetadata> metadataUpdates = <AudioMediaMetadata>[];
  final List<_PlaybackStateRecord> playbackUpdates = <_PlaybackStateRecord>[];
  int clearCount = 0;
  int disposeCount = 0;

  @override
  void bindHandlers(MediaSessionActionHandlers handlers) {
    boundHandlers = handlers;
  }

  @override
  void updateMetadata(AudioMediaMetadata metadata) {
    metadataUpdates.add(metadata);
  }

  @override
  void updatePlaybackState({
    required AudioPlaybackStatus status,
    required Duration position,
    required Duration duration,
    required double speed,
  }) {
    playbackUpdates.add(
      _PlaybackStateRecord(
        status: status,
        position: position,
        duration: duration,
        speed: speed,
      ),
    );
  }

  @override
  void clear() {
    clearCount++;
  }

  @override
  void dispose() {
    disposeCount++;
  }
}

class _MockAudioApiService extends ApiService {
  _MockAudioApiService()
      : super(baseUrl: 'http://localhost:8080', token: 'secret-tok');

  List<AudioChapter> mockChapters = const [];
  AudiobookProgress? mockProgress;

  @override
  Future<List<AudioChapter>> getAudioChapters(String bookId) async =>
      mockChapters;

  @override
  Future<AudiobookProgress> getAudiobookProgress(String bookId) async =>
      mockProgress ??
      AudiobookProgress(bookId: bookId, positionSeconds: 0.0, speed: 1.0);

  @override
  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async {
    final p = AudiobookProgress(
      bookId: bookId,
      positionSeconds: positionSeconds,
      speed: speed,
      isCompleted: isCompleted,
    );
    mockProgress = p;
    return p;
  }

  @override
  String getAudioStreamUrl(String bookId) =>
      '$baseUrl/api/v1/audiobooks/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('resolveAuthenticatedCoverUrl', () {
    test('returns null when coverUrl is null or empty', () {
      const noCover = Book(id: 'b1', title: 'No Cover');
      const blankCover = Book(id: 'b2', title: 'Blank', coverUrl: '   ');

      expect(
        resolveAuthenticatedCoverUrl(
          noCover,
          baseUrl: 'https://shelf.local',
          token: 'abc',
        ),
        isNull,
      );
      expect(
        resolveAuthenticatedCoverUrl(
          blankCover,
          baseUrl: 'https://shelf.local',
          token: 'abc',
        ),
        isNull,
      );
    });

    test('resolves relative cover path, omits ?token on native, and appends ?token on web', () {
      const book = Book(
        id: 'b1',
        title: 'Dune',
        coverUrl: '/api/v1/books/b1/cover',
      );

      final nativeUrl = resolveAuthenticatedCoverUrl(
        book,
        baseUrl: 'https://shelf.example.com/',
        token: 'jwt-123',
        isWeb: false,
      );
      expect(nativeUrl, 'https://shelf.example.com/api/v1/books/b1/cover');

      final webUrl = resolveAuthenticatedCoverUrl(
        book,
        baseUrl: 'https://shelf.example.com/',
        token: 'jwt-123',
        isWeb: true,
      );
      expect(
        webUrl,
        'https://shelf.example.com/api/v1/books/b1/cover?token=jwt-123',
      );
    });

    test('falls back to fallbackOrigin when baseUrl is empty', () {
      const book = Book(
        id: 'b1',
        title: 'Dune',
        coverUrl: '/api/v1/books/b1/cover?size=lg',
      );

      final url = resolveAuthenticatedCoverUrl(
        book,
        baseUrl: '',
        token: 'jwt-123',
        fallbackOrigin: 'http://localhost:3000/',
        isWeb: true,
      );

      expect(
        url,
        'http://localhost:3000/api/v1/books/b1/cover?size=lg&token=jwt-123',
      );
    });

    test('preserves existing token query parameter', () {
      const book = Book(
        id: 'b1',
        title: 'Dune',
        coverUrl: 'https://shelf.example.com/api/v1/books/b1/cover?token=existing',
      );

      final url = resolveAuthenticatedCoverUrl(
        book,
        baseUrl: 'https://shelf.example.com',
        token: 'different',
        isWeb: true,
      );

      expect(
        url,
        'https://shelf.example.com/api/v1/books/b1/cover?token=existing',
      );
    });
  });

  group('AudioMediaMetadata.fromAudioState', () {
    test('returns null when state has no book', () {
      expect(
        AudioMediaMetadata.fromAudioState(
          const AudioPlayerState(),
          baseUrl: 'https://shelf.local',
        ),
        isNull,
      );
    });

    test('uses active chapter title as album and attaches origin-scoped artworkHeaders on native', () {
      const book = Book(
        id: 'b1',
        title: 'Project Hail Mary',
        coverUrl: '/api/v1/books/b1/cover',
        authors: [Author(id: 'a1', name: 'Andy Weir')],
        series: Series(id: 's1', name: 'Standalone'),
        seriesSequence: 1.0,
      );
      const chapter = AudioChapter(
        id: 'ch1',
        bookId: 'b1',
        chapterIndex: 0,
        title: 'Chapter 1: Awakening',
        startOffsetSec: 0,
        durationSec: 600,
      );
      const state = AudioPlayerState(
        book: book,
        status: AudioPlaybackStatus.playing,
        duration: Duration(hours: 1),
        activeChapter: chapter,
      );

      final nativeMeta = AudioMediaMetadata.fromAudioState(
        state,
        baseUrl: 'https://shelf.local',
        token: 'tok',
        isWeb: false,
      );

      expect(nativeMeta, isNotNull);
      expect(nativeMeta!.id, 'b1');
      expect(nativeMeta.title, 'Project Hail Mary');
      expect(nativeMeta.artist, 'Andy Weir');
      expect(nativeMeta.album, 'Chapter 1: Awakening');
      expect(nativeMeta.artworkUrl, 'https://shelf.local/api/v1/books/b1/cover');
      expect(nativeMeta.artworkUri, Uri.parse('https://shelf.local/api/v1/books/b1/cover'));
      expect(nativeMeta.artworkHeaders, {'Authorization': 'Bearer tok'});
      expect(nativeMeta.duration, const Duration(hours: 1));
      expect(nativeMeta.isReadalong, isFalse);

      final webMeta = AudioMediaMetadata.fromAudioState(
        state,
        baseUrl: 'https://shelf.local',
        token: 'tok',
        isWeb: true,
      );
      expect(webMeta!.artworkUrl, 'https://shelf.local/api/v1/books/b1/cover?token=tok');
      expect(webMeta.artworkHeaders, isNull);

      const externalCoverBook = Book(
        id: 'b-ext',
        title: 'External Cover',
        coverUrl: 'https://covers.openlibrary.org/b/id/123-L.jpg',
      );
      final externalMeta = AudioMediaMetadata.fromAudioState(
        const AudioPlayerState(book: externalCoverBook),
        baseUrl: 'https://shelf.local',
        token: 'tok',
        isWeb: false,
      );
      expect(externalMeta!.artworkHeaders, isNull);
    });

    test('falls back to series label and then playback mode', () {
      const seriesBook = Book(
        id: 'b2',
        title: 'Dune Messiah',
        authors: [Author(id: 'a1', name: 'Frank Herbert')],
        series: Series(id: 's1', name: 'Dune'),
        seriesSequence: 2.0,
      );
      final seriesMeta = AudioMediaMetadata.fromAudioState(
        const AudioPlayerState(book: seriesBook),
        baseUrl: 'https://shelf.local',
      );
      expect(seriesMeta!.album, 'Dune #2');

      const readalongBook = Book(
        id: 'b3',
        title: 'Picture Book',
        hasReadalong: true,
      );
      final readalongMeta = AudioMediaMetadata.fromAudioState(
        const AudioPlayerState(
          book: readalongBook,
          playbackMode: AudioPlaybackMode.readalong,
        ),
        baseUrl: 'https://shelf.local',
      );
      expect(readalongMeta!.album, 'Read-Along');
      expect(readalongMeta.isReadalong, isTrue);

      final audiobookMeta = AudioMediaMetadata.fromAudioState(
        const AudioPlayerState(
          book: readalongBook,
          playbackMode: AudioPlaybackMode.audiobook,
        ),
        baseUrl: 'https://shelf.local',
      );
      expect(audiobookMeta!.album, 'Audiobook');
    });
  });

  group('AudioPlayerNotifier MediaSessionBridge integration', () {
    late ProviderContainer container;
    late SimulatedAudioPlayerEngine engine;
    late _FakeMediaSessionBridge mediaBridge;
    late _MockAudioApiService apiService;

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);
      apiService = _MockAudioApiService();
      engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 600),
      );
      mediaBridge = _FakeMediaSessionBridge();

      container = ProviderContainer(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storage),
          apiServiceProvider.overrideWithValue(apiService),
          audioRepositoryProvider.overrideWithValue(
            AudioRepository(apiService: apiService, storageService: storage),
          ),
          audioPlayerEngineProvider.overrideWithValue(engine),
          mediaSessionBridgeProvider.overrideWithValue(mediaBridge),
        ],
      );
    });

    tearDown(() async {
      container.dispose();
      await engine.dispose();
    });

    test('syncs metadata, chapters, playback state, action handlers, and stop()', () async {
      final chapters = [
        const AudioChapter(
          id: 'ch1',
          bookId: 'book-1',
          chapterIndex: 0,
          title: 'Chapter 1',
          startOffsetSec: 0,
          durationSec: 200,
        ),
        const AudioChapter(
          id: 'ch2',
          bookId: 'book-1',
          chapterIndex: 1,
          title: 'Chapter 2',
          startOffsetSec: 200,
          durationSec: 400,
        ),
      ];
      apiService.mockChapters = chapters;

      const book = Book(
        id: 'book-1',
        title: 'The Martian',
        coverUrl: '/api/v1/books/book-1/cover',
        authors: [Author(id: 'a1', name: 'Andy Weir')],
        bookType: 'audiobook',
        durationSeconds: 600,
      );

      final notifier = container.read(audioPlayerProvider.notifier);
      expect(mediaBridge.boundHandlers, isNotNull);

      await notifier.loadBook(book, autoPlay: true);

      expect(mediaBridge.metadataUpdates, isNotEmpty);
      expect(mediaBridge.metadataUpdates.last.title, 'The Martian');
      expect(mediaBridge.metadataUpdates.last.artist, 'Andy Weir');
      expect(mediaBridge.metadataUpdates.last.album, 'Chapter 1');
      expect(
        mediaBridge.metadataUpdates.last.artworkUrl,
        'http://localhost:8080/api/v1/books/book-1/cover',
      );
      expect(
        mediaBridge.metadataUpdates.last.artworkHeaders,
        {'Authorization': 'Bearer secret-tok'},
      );
      expect(
        engine.lastHeaders,
        {'Authorization': 'Bearer secret-tok'},
      );
      expect(mediaBridge.playbackUpdates.last.status, AudioPlaybackStatus.playing);

      mediaBridge.boundHandlers!.onPause?.call();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).status, AudioPlaybackStatus.paused);
      expect(mediaBridge.playbackUpdates.last.status, AudioPlaybackStatus.paused);

      mediaBridge.boundHandlers!.onPlay?.call();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).status, AudioPlaybackStatus.playing);

      // Pause so SimulatedAudioPlayerEngine's 100ms ticker does not drift during exact seek assertions.
      mediaBridge.boundHandlers!.onPause?.call();
      await Future<void>.delayed(const Duration(milliseconds: 20));

      mediaBridge.boundHandlers!.onNextChapter?.call();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).activeChapter?.title, 'Chapter 2');
      expect(mediaBridge.metadataUpdates.last.album, 'Chapter 2');

      mediaBridge.boundHandlers!.onSeek?.call(const Duration(seconds: 45));
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).position, const Duration(seconds: 45));
      expect(mediaBridge.metadataUpdates.last.album, 'Chapter 1');

      mediaBridge.boundHandlers!.onSkipForward?.call(30);
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).position, const Duration(seconds: 75));

      mediaBridge.boundHandlers!.onSkipBackward?.call(15);
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(container.read(audioPlayerProvider).position, const Duration(seconds: 60));

      notifier.setPlaybackMode(AudioPlaybackMode.readalong);
      expect(mediaBridge.metadataUpdates.last.isReadalong, isTrue);

      mediaBridge.boundHandlers!.onStop?.call();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(mediaBridge.clearCount, greaterThanOrEqualTo(1));
      expect(container.read(audioPlayerProvider).book, isNull);
    });
  });
}
