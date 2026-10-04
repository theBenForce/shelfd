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
import 'package:shelf/ui/features/audiobook/audiobook_player_view.dart';
import 'package:shelf/ui/features/audiobook/widgets/mini_player_bar.dart';
import 'package:shelf/ui/state/providers.dart';

class MockApiService extends ApiService {
  MockApiService() : super(baseUrl: 'http://localhost:8080');

  List<AudioChapter> mockChapters = [];
  AudiobookProgress? mockProgress;
  bool saveProgressCalled = false;
  bool deleteProgressCalled = false;

  @override
  Future<List<AudioChapter>> getAudioChapters(String bookId) async {
    return mockChapters;
  }

  @override
  Future<AudiobookProgress> getAudiobookProgress(String bookId) async {
    if (mockProgress != null) return mockProgress!;
    throw Exception('Not found');
  }

  @override
  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async {
    saveProgressCalled = true;
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
  Future<void> deleteAudiobookProgress(String bookId) async {
    deleteProgressCalled = true;
    mockProgress = null;
  }

  @override
  String getAudioStreamUrl(String bookId) => 'http://localhost:8080/api/books/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('Audiobook Models', () {
    test('AudioChapter serialization and deserialization', () {
      final json = {
        'id': 'ch1',
        'book_id': 'b1',
        'title': 'Chapter 1: The Beginning',
        'start_offset_sec': 0.0,
        'duration_sec': 300.0,
        'chapter_index': 0,
      };

      final chapter = AudioChapter.fromJson(json);
      expect(chapter.id, 'ch1');
      expect(chapter.bookId, 'b1');
      expect(chapter.title, 'Chapter 1: The Beginning');
      expect(chapter.startOffsetSec, 0.0);
      expect(chapter.durationSec, 300.0);
      expect(chapter.chapterIndex, 0);

      final encoded = chapter.toJson();
      expect(encoded['id'], 'ch1');
      expect(encoded['title'], 'Chapter 1: The Beginning');
      expect(encoded['start_offset_sec'], 0.0);
      expect(encoded['duration_sec'], 300.0);
    });

    test('BookFile serialization and deserialization', () {
      final json = {
        'id': 'f1',
        'book_id': 'b1',
        'file_type': 'm4b',
        'file_path': 'book.m4b',
        'file_size_bytes': 10485760,
        'duration_seconds': 1800.0,
        'bitrate_kbps': 128,
      };

      final file = BookFile.fromJson(json);
      expect(file.id, 'f1');
      expect(file.fileType, 'm4b');
      expect(file.filePath, 'book.m4b');
      expect(file.fileSizeBytes, 10485760);
      expect(file.durationSeconds, 1800.0);
      expect(file.bitrateKbps, 128);

      final encoded = file.toJson();
      expect(encoded['file_type'], 'm4b');
      expect(encoded['duration_seconds'], 1800.0);
    });

    test('AudiobookProgress serialization and deserialization', () {
      final json = {
        'book_id': 'b1',
        'position_seconds': 142.5,
        'speed': 1.25,
        'is_completed': false,
        'updated_at': '2026-10-03T18:00:00.000Z',
      };

      final progress = AudiobookProgress.fromJson(json);
      expect(progress.bookId, 'b1');
      expect(progress.positionSeconds, 142.5);
      expect(progress.speed, 1.25);
      expect(progress.isCompleted, false);
      expect(progress.updatedAt, isNotNull);

      final encoded = progress.toJson();
      expect(encoded['book_id'], 'b1');
      expect(encoded['position_seconds'], 142.5);
      expect(encoded['speed'], 1.25);
      expect(encoded['is_completed'], false);
    });

    test('Book audio properties and isAudiobook getter', () {
      final book = Book(
        id: 'b1',
        title: 'Project Hail Mary',
        bookType: 'audiobook',
        durationSeconds: 57600.0,
        bitrateKbps: 64,
        files: [
          BookFile(
            id: 'f1',
            bookId: 'b1',
            fileType: 'm4b',
            filePath: 'phm.m4b',
            fileSizeBytes: 200000000,
            durationSeconds: 57600.0,
          ),
        ],
        audioChapters: [
          AudioChapter(
            id: 'ch1',
            bookId: 'b1',
            title: 'Chapter 1',
            startOffsetSec: 0.0,
            durationSec: 1800.0,
            chapterIndex: 0,
          ),
          AudioChapter(
            id: 'ch2',
            bookId: 'b1',
            title: 'Chapter 2',
            startOffsetSec: 1800.0,
            durationSec: 1800.0,
            chapterIndex: 1,
          ),
        ],
      );

      expect(book.isAudiobook, isTrue);
      expect(book.durationSeconds, 57600.0);
      expect(book.bitrateKbps, 64);
      expect(book.files.length, 1);
      expect(book.audioChapters.length, 2);

      final json = book.toJson();
      expect(json['book_type'], 'audiobook');
      expect(json['duration_seconds'], 57600.0);
      expect((json['files'] as List).length, 1);
      expect((json['audio_chapters'] as List).length, 2);
    });
  });

  group('AudioRepository', () {
    late SharedPreferences prefs;
    late StorageService storageService;
    late MockApiService apiService;
    late AudioRepository repository;

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      prefs = await SharedPreferences.getInstance();
      storageService = StorageService(prefs);
      apiService = MockApiService();
      repository = AudioRepository(
        apiService: apiService,
        storageService: storageService,
      );
    });

    test('getChapters returns API chapters', () async {
      apiService.mockChapters = [
        AudioChapter(
          id: 'ch1',
          bookId: 'b1',
          title: 'Intro',
          startOffsetSec: 0.0,
          durationSec: 120.0,
          chapterIndex: 0,
        ),
      ];

      final chapters = await repository.getChapters('b1');
      expect(chapters.length, 1);
      expect(chapters.first.title, 'Intro');
    });

    test('saveProgress saves locally and remotely', () async {
      final res = await repository.saveProgress('b1', positionSeconds: 250.0, speed: 1.5);
      expect(res.positionSeconds, 250.0);
      expect(res.speed, 1.5);
      expect(apiService.saveProgressCalled, isTrue);
      expect(storageService.getAudiobookProgress('b1'), 250.0);
      expect(storageService.getAudiobookSpeed('b1'), 1.5);
    });

    test('getProgress falls back to local storage on API failure', () async {
      await storageService.saveAudiobookProgress('b2', 75.0);
      await storageService.saveAudiobookSpeed('b2', 1.25);

      final progress = await repository.getProgress('b2');
      expect(progress.positionSeconds, 75.0);
      expect(progress.speed, 1.25);
      expect(progress.bookId, 'b2');
    });
  });

  group('AudioPlayerNotifier & Playback Logic', () {
    late SharedPreferences prefs;
    late StorageService storageService;
    late MockApiService apiService;
    late AudioRepository repository;
    late SimulatedAudioPlayerEngine engine;
    late ProviderContainer container;

    final testBook = Book(
      id: 'b1',
      title: 'Dune',
      bookType: 'audiobook',
      durationSeconds: 3600.0,
      audioChapters: [
        AudioChapter(
          id: 'c1',
          bookId: 'b1',
          title: 'Chapter 1',
          startOffsetSec: 0.0,
          durationSec: 1200.0,
          chapterIndex: 0,
        ),
        AudioChapter(
          id: 'c2',
          bookId: 'b1',
          title: 'Chapter 2',
          startOffsetSec: 1200.0,
          durationSec: 1200.0,
          chapterIndex: 1,
        ),
        AudioChapter(
          id: 'c3',
          bookId: 'b1',
          title: 'Chapter 3',
          startOffsetSec: 2400.0,
          durationSec: 1200.0,
          chapterIndex: 2,
        ),
      ],
    );

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      prefs = await SharedPreferences.getInstance();
      storageService = StorageService(prefs);
      apiService = MockApiService();
      repository = AudioRepository(
        apiService: apiService,
        storageService: storageService,
      );
      engine = SimulatedAudioPlayerEngine(initialDuration: const Duration(seconds: 3600));

      container = ProviderContainer(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          storageServiceProvider.overrideWithValue(storageService),
          apiServiceProvider.overrideWithValue(apiService),
          audioRepositoryProvider.overrideWithValue(repository),
          audioPlayerEngineProvider.overrideWithValue(engine),
        ],
      );
    });

    tearDown(() {
      container.dispose();
    });

    test('loadBook initializes player state and resolves active chapter', () async {
      final notifier = container.read(audioPlayerProvider.notifier);

      await notifier.loadBook(testBook, initialPosition: 1300.0);
      final state = container.read(audioPlayerProvider);

      expect(state.book?.id, 'b1');
      expect(state.status, AudioPlaybackStatus.ready);
      expect(state.position, const Duration(seconds: 1300));
      expect(state.duration, const Duration(seconds: 3600));
      expect(state.activeChapter?.id, 'c2');
      expect(state.currentChapterIndex, 1);
      expect(state.chapterPosition, const Duration(seconds: 100)); // 1300 - 1200
      expect(state.chapterDuration, const Duration(seconds: 1200));
    });

    test('play, pause, togglePlayPause state transitions', () async {
      final notifier = container.read(audioPlayerProvider.notifier);
      await notifier.loadBook(testBook);

      await notifier.play();
      expect(container.read(audioPlayerProvider).isPlaying, isTrue);

      await notifier.pause();
      expect(container.read(audioPlayerProvider).isPlaying, isFalse);
      expect(container.read(audioPlayerProvider).status, AudioPlaybackStatus.paused);

      await notifier.togglePlayPause();
      expect(container.read(audioPlayerProvider).isPlaying, isTrue);
    });

    test('seeking and chapter updates', () async {
      final notifier = container.read(audioPlayerProvider.notifier);
      await notifier.loadBook(testBook);

      // Seek into chapter 3
      await notifier.seek(const Duration(seconds: 2500));
      var state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 2500));
      expect(state.activeChapter?.id, 'c3');
      expect(state.currentChapterIndex, 2);

      // Skip backward 15s
      await notifier.skipBackward(15);
      state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 2485));

      // Skip forward 30s
      await notifier.skipForward(30);
      state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 2515));
    });

    test('chapter navigation (next, previous, seekToChapter)', () async {
      final notifier = container.read(audioPlayerProvider.notifier);
      await notifier.loadBook(testBook);

      await notifier.seekToChapter(1);
      var state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 1200));
      expect(state.activeChapter?.id, 'c2');

      await notifier.nextChapter();
      state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 2400));
      expect(state.activeChapter?.id, 'c3');

      await notifier.previousChapter();
      state = container.read(audioPlayerProvider);
      expect(state.position, const Duration(seconds: 1200));
      expect(state.activeChapter?.id, 'c2');
    });

    test('speed adjustment', () async {
      final notifier = container.read(audioPlayerProvider.notifier);
      await notifier.loadBook(testBook);

      await notifier.setSpeed(1.5);
      expect(container.read(audioPlayerProvider).speed, 1.5);
    });

    test('sleep timer setting and cancellation', () async {
      final notifier = container.read(audioPlayerProvider.notifier);
      await notifier.loadBook(testBook);

      notifier.setSleepTimer(SleepTimerOption.min15);
      var state = container.read(audioPlayerProvider);
      expect(state.sleepTimerOption, SleepTimerOption.min15);
      expect(state.sleepTimerRemaining, const Duration(minutes: 15));
      expect(state.volume, 1.0);

      notifier.cancelSleepTimer();
      state = container.read(audioPlayerProvider);
      expect(state.sleepTimerOption, SleepTimerOption.none);
      expect(state.sleepTimerRemaining, isNull);
      expect(state.volume, 1.0);
    });
  });

  group('Audiobook UI Widget Tests', () {
    late SharedPreferences prefs;
    late StorageService storageService;
    late MockApiService apiService;
    late AudioRepository repository;

    final audiobook = Book(
      id: 'audio-1',
      title: 'Dune: Special Edition',
      authors: const [Author(id: 'auth-1', name: 'Frank Herbert')],
      bookType: 'audiobook',
      durationSeconds: 7200.0,
      files: const [
        BookFile(
          id: 'f1',
          bookId: 'audio-1',
          filePath: 'dune.m4a',
          fileType: 'audio',
          durationSeconds: 7200.0,
        ),
      ],
      audioChapters: const [
        AudioChapter(
          id: 'ch1',
          bookId: 'audio-1',
          title: 'Part 1: Dune',
          startOffsetSec: 0.0,
          durationSec: 3600.0,
          chapterIndex: 0,
        ),
      ],
    );

    setUp(() async {
      SharedPreferences.setMockInitialValues({});
      prefs = await SharedPreferences.getInstance();
      storageService = StorageService(prefs);
      apiService = MockApiService();
      apiService.mockChapters = audiobook.audioChapters;
      repository = AudioRepository(
        apiService: apiService,
        storageService: storageService,
      );
    });

    testWidgets('AudiobookPlayerView renders standard Material Icons correctly', (tester) async {
      tester.view.physicalSize = const Size(400, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final container = ProviderContainer(
        overrides: [
          apiServiceProvider.overrideWithValue(apiService),
          storageServiceProvider.overrideWithValue(storageService),
          audioRepositoryProvider.overrideWithValue(repository),
          bookDetailProvider('audio-1').overrideWith(
            () => _MockAudioBookDetailNotifier(audiobook),
          ),
        ],
      );

      await container.read(audioPlayerProvider.notifier).loadBook(audiobook);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const AudiobookPlayerView(bookId: 'audio-1'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Verify all essential controls and standard icons are rendered
      expect(find.byKey(const Key('audio_player_back_button')), findsOneWidget);
      expect(find.byIcon(Icons.keyboard_arrow_down), findsOneWidget);

      expect(find.byKey(const Key('audio_player_chapters_menu_button')), findsOneWidget);
      expect(find.byIcon(Icons.format_list_bulleted), findsOneWidget);

      expect(find.byKey(const Key('audio_player_skip_back_button')), findsOneWidget);
      expect(find.byIcon(Icons.replay_10), findsOneWidget);

      expect(find.byKey(const Key('audio_player_play_pause_button')), findsOneWidget);
      expect(find.byIcon(Icons.play_arrow), findsOneWidget);

      expect(find.byKey(const Key('audio_player_skip_forward_button')), findsOneWidget);
      expect(find.byIcon(Icons.forward_30), findsOneWidget);

      expect(find.byKey(const Key('audio_player_speed_button')), findsOneWidget);
      expect(find.byIcon(Icons.speed), findsOneWidget);

      expect(find.byKey(const Key('audio_player_sleep_timer_button')), findsOneWidget);
      expect(find.byIcon(Icons.bedtime), findsOneWidget);

      expect(find.byKey(const Key('audio_player_add_bookmark_button')), findsOneWidget);
      expect(find.byIcon(Icons.bookmark_add_outlined), findsOneWidget);
    });

    testWidgets('MiniPlayerBar renders standard Material Icons correctly', (tester) async {
      final container = ProviderContainer(
        overrides: [
          apiServiceProvider.overrideWithValue(apiService),
          storageServiceProvider.overrideWithValue(storageService),
          audioRepositoryProvider.overrideWithValue(repository),
        ],
      );

      await container.read(audioPlayerProvider.notifier).loadBook(audiobook);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: const Scaffold(
              bottomNavigationBar: MiniPlayerBar(),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.headphones), findsOneWidget);
      expect(find.byKey(const Key('mini_player_skip_30')), findsOneWidget);
      expect(find.byIcon(Icons.forward_30), findsOneWidget);
      expect(find.byKey(const Key('mini_player_play_pause')), findsOneWidget);
      expect(find.byIcon(Icons.play_circle_filled), findsOneWidget);
      expect(find.byKey(const Key('mini_player_close')), findsOneWidget);
      expect(find.byIcon(Icons.close), findsOneWidget);
    });
  });
}

class _MockAudioBookDetailNotifier extends BookDetailNotifier {
  final Book mockBook;
  _MockAudioBookDetailNotifier(this.mockBook) : super('audio-1');

  @override
  BookDetailState build() {
    return BookDetailState(book: mockBook, isLoading: false);
  }
}
