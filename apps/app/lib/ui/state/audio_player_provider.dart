import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:just_audio/just_audio.dart';
import '../../data/models/audiobook.dart';
import '../../data/models/book.dart';
import '../../data/repositories/audio_repository.dart';
import 'audio_player_state.dart';
import 'providers.dart';

abstract class AudioPlayerEngine {
  Stream<Duration> get positionStream;
  Stream<Duration?> get durationStream;
  Stream<AudioPlaybackStatus> get statusStream;

  Future<void> setUrl(String url, {Duration? initialPosition});
  Future<void> play();
  Future<void> pause();
  Future<void> stop();
  Future<void> seek(Duration position);
  Future<void> setSpeed(double speed);
  Future<void> setVolume(double volume);
  Future<void> dispose();
}

class JustAudioPlayerEngine implements AudioPlayerEngine {
  final AudioPlayer _player;
  final _statusController = StreamController<AudioPlaybackStatus>.broadcast();
  StreamSubscription? _playerStateSub;

  JustAudioPlayerEngine({AudioPlayer? player}) : _player = player ?? AudioPlayer() {
    _playerStateSub = _player.playerStateStream.listen((playerState) {
      final processingState = playerState.processingState;
      if (processingState == ProcessingState.loading ||
          processingState == ProcessingState.buffering) {
        _statusController.add(AudioPlaybackStatus.loading);
      } else if (processingState == ProcessingState.completed) {
        _statusController.add(AudioPlaybackStatus.completed);
      } else if (processingState == ProcessingState.ready) {
        _statusController.add(
          playerState.playing ? AudioPlaybackStatus.playing : AudioPlaybackStatus.paused,
        );
      } else if (processingState == ProcessingState.idle) {
        _statusController.add(AudioPlaybackStatus.idle);
      }
    });
  }

  @override
  Stream<Duration> get positionStream => _player.positionStream;

  @override
  Stream<Duration?> get durationStream => _player.durationStream;

  @override
  Stream<AudioPlaybackStatus> get statusStream => _statusController.stream;

  @override
  Future<void> setUrl(String url, {Duration? initialPosition}) async {
    await _player.setUrl(url, initialPosition: initialPosition);
  }

  @override
  Future<void> play() => _player.play();

  @override
  Future<void> pause() => _player.pause();

  @override
  Future<void> stop() => _player.stop();

  @override
  Future<void> seek(Duration position) => _player.seek(position);

  @override
  Future<void> setSpeed(double speed) => _player.setSpeed(speed);

  @override
  Future<void> setVolume(double volume) => _player.setVolume(volume);

  @override
  Future<void> dispose() async {
    await _playerStateSub?.cancel();
    await _statusController.close();
    await _player.dispose();
  }
}

class SimulatedAudioPlayerEngine implements AudioPlayerEngine {
  final _positionController = StreamController<Duration>.broadcast();
  final _durationController = StreamController<Duration?>.broadcast();
  final _statusController = StreamController<AudioPlaybackStatus>.broadcast();
  Duration _currentPosition = Duration.zero;
  Duration _currentDuration = const Duration(hours: 1);
  AudioPlaybackStatus _currentStatus = AudioPlaybackStatus.idle;
  double _currentSpeed = 1.0;
  double _currentVolume = 1.0;
  Timer? _ticker;

  SimulatedAudioPlayerEngine({Duration? initialDuration}) {
    if (initialDuration != null) {
      _currentDuration = initialDuration;
    }
  }

  Duration get currentPosition => _currentPosition;
  double get currentVolume => _currentVolume;
  double get currentSpeed => _currentSpeed;

  @override
  Stream<Duration> get positionStream => _positionController.stream;

  @override
  Stream<Duration?> get durationStream => _durationController.stream;

  @override
  Stream<AudioPlaybackStatus> get statusStream => _statusController.stream;

  @override
  Future<void> setUrl(String url, {Duration? initialPosition}) async {
    _currentPosition = initialPosition ?? Duration.zero;
    _currentStatus = AudioPlaybackStatus.ready;
    _statusController.add(_currentStatus);
    _positionController.add(_currentPosition);
    _durationController.add(_currentDuration);
  }

  @override
  Future<void> play() async {
    _currentStatus = AudioPlaybackStatus.playing;
    _statusController.add(_currentStatus);
    _ticker?.cancel();
    _ticker = Timer.periodic(const Duration(milliseconds: 100), (_) {
      if (_currentStatus == AudioPlaybackStatus.playing) {
        _currentPosition += Duration(milliseconds: (100 * _currentSpeed).round());
        if (_currentPosition >= _currentDuration) {
          _currentPosition = _currentDuration;
          _currentStatus = AudioPlaybackStatus.completed;
          _statusController.add(_currentStatus);
          _ticker?.cancel();
        }
        _positionController.add(_currentPosition);
      }
    });
  }

  @override
  Future<void> pause() async {
    _currentStatus = AudioPlaybackStatus.paused;
    _statusController.add(_currentStatus);
    _ticker?.cancel();
  }

  @override
  Future<void> stop() async {
    _currentStatus = AudioPlaybackStatus.idle;
    _statusController.add(_currentStatus);
    _ticker?.cancel();
  }

  @override
  Future<void> seek(Duration position) async {
    _currentPosition = position < Duration.zero
        ? Duration.zero
        : (position > _currentDuration ? _currentDuration : position);
    _positionController.add(_currentPosition);
  }

  @override
  Future<void> setSpeed(double speed) async {
    _currentSpeed = speed;
  }

  @override
  Future<void> setVolume(double volume) async {
    _currentVolume = volume.clamp(0.0, 1.0);
  }

  @override
  Future<void> dispose() async {
    _ticker?.cancel();
    await _positionController.close();
    await _durationController.close();
    await _statusController.close();
  }
}

final audioPlayerEngineProvider = Provider<AudioPlayerEngine>((ref) {
  final engine = JustAudioPlayerEngine();
  ref.onDispose(() => engine.dispose());
  return engine;
});

class AudioPlayerNotifier extends Notifier<AudioPlayerState> {
  Timer? _syncTimer;
  Timer? _sleepTimerTicker;
  StreamSubscription? _posSub;
  StreamSubscription? _durSub;
  StreamSubscription? _statusSub;

  AudioPlayerEngine get _engine => ref.read(audioPlayerEngineProvider);
  AudioRepository get _repository => ref.read(audioRepositoryProvider);

  @override
  AudioPlayerState build() {
    ref.onDispose(() {
      _syncTimer?.cancel();
      _sleepTimerTicker?.cancel();
      _posSub?.cancel();
      _durSub?.cancel();
      _statusSub?.cancel();
    });
    return const AudioPlayerState();
  }

  void _initSubscriptions() {
    _posSub?.cancel();
    _durSub?.cancel();
    _statusSub?.cancel();

    _posSub = _engine.positionStream.listen((pos) {
      if (state.status == AudioPlaybackStatus.playing || state.status == AudioPlaybackStatus.ready) {
        final newActive = _resolveChapter(pos, state.chapters);
        state = state.copyWith(position: pos, activeChapter: newActive);
      }
    });

    _durSub = _engine.durationStream.listen((dur) {
      if (dur != null && dur > Duration.zero) {
        state = state.copyWith(duration: dur);
      }
    });

    _statusSub = _engine.statusStream.listen((status) {
      if (status == AudioPlaybackStatus.completed) {
        _onPlaybackCompleted();
      } else {
        state = state.copyWith(status: status);
      }
    });
  }

  AudioChapter? _resolveChapter(Duration pos, List<AudioChapter> chapters) {
    if (chapters.isEmpty) return null;
    final posSec = pos.inMilliseconds / 1000.0;
    for (int i = 0; i < chapters.length; i++) {
      final ch = chapters[i];
      final start = ch.startOffsetSec;
      final end = start + ch.durationSec;
      if (posSec >= start && (posSec < end || i == chapters.length - 1)) {
        return ch;
      }
    }
    return chapters.first;
  }

  Future<void> loadBook(Book book, {double? initialPosition, bool autoPlay = false}) async {
    try {
      state = state.copyWith(
        book: book,
        status: AudioPlaybackStatus.loading,
        clearError: true,
      );

      _initSubscriptions();

      List<AudioChapter> chapters = book.audioChapters;
      if (chapters.isEmpty) {
        chapters = await _repository.getChapters(book.id);
      }

      final progress = await _repository.getProgress(book.id);
      final double targetPosSec = initialPosition ?? progress.positionSeconds;
      final Duration initialDuration = book.durationSeconds != null && book.durationSeconds! > 0
          ? Duration(milliseconds: (book.durationSeconds! * 1000).round())
          : (chapters.isNotEmpty
              ? Duration(
                  milliseconds: ((chapters.last.startOffsetSec + chapters.last.durationSec) * 1000).round(),
                )
              : Duration.zero);

      final streamUrl = _repository.getStreamUrl(book.id);
      final startPos = Duration(milliseconds: (targetPosSec * 1000).round());

      await _engine.setUrl(streamUrl, initialPosition: startPos);
      if (progress.speed != 1.0) {
        await _engine.setSpeed(progress.speed);
      }

      final activeChapter = _resolveChapter(startPos, chapters);

      state = state.copyWith(
        book: book,
        status: AudioPlaybackStatus.ready,
        position: startPos,
        duration: initialDuration,
        speed: progress.speed,
        chapters: chapters,
        activeChapter: activeChapter,
      );

      if (autoPlay) {
        await play();
      }
    } catch (e) {
      state = state.copyWith(
        status: AudioPlaybackStatus.error,
        errorMessage: 'Failed to load audiobook: $e',
      );
    }
  }

  Future<void> play() async {
    if (state.book == null) return;
    try {
      state = state.copyWith(status: AudioPlaybackStatus.playing);
      await _engine.play();
      _startPeriodicSync();
    } catch (e) {
      state = state.copyWith(
        status: AudioPlaybackStatus.error,
        errorMessage: 'Failed to start playback: $e',
      );
    }
  }

  Future<void> pause() async {
    if (state.book == null) return;
    try {
      await _engine.pause();
      state = state.copyWith(status: AudioPlaybackStatus.paused);
      _syncTimer?.cancel();
      await _syncProgressToServer();
    } catch (e) {
      state = state.copyWith(
        status: AudioPlaybackStatus.error,
        errorMessage: 'Failed to pause playback: $e',
      );
    }
  }

  Future<void> togglePlayPause() async {
    if (state.isPlaying) {
      await pause();
    } else {
      await play();
    }
  }

  Future<void> seek(Duration position) async {
    if (state.book == null) return;
    final clamped = position < Duration.zero
        ? Duration.zero
        : (state.duration > Duration.zero && position > state.duration
            ? state.duration
            : position);

    await _engine.seek(clamped);
    final newActive = _resolveChapter(clamped, state.chapters);
    state = state.copyWith(position: clamped, activeChapter: newActive);
    await _syncProgressToServer();
  }

  Future<void> skipForward([int seconds = 30]) async {
    await seek(state.position + Duration(seconds: seconds));
  }

  Future<void> skipBackward([int seconds = 15]) async {
    await seek(state.position - Duration(seconds: seconds));
  }

  Future<void> setSpeed(double speed) async {
    if (state.book == null) return;
    await _engine.setSpeed(speed);
    state = state.copyWith(speed: speed);
    await _syncProgressToServer();
  }

  Future<void> setVolume(double volume) async {
    await _engine.setVolume(volume);
    state = state.copyWith(volume: volume);
  }

  Future<void> seekToChapter(int index) async {
    if (index < 0 || index >= state.chapters.length) return;
    final ch = state.chapters[index];
    final target = Duration(milliseconds: (ch.startOffsetSec * 1000).round());
    await seek(target);
  }

  Future<void> nextChapter() async {
    final idx = state.currentChapterIndex;
    if (idx >= 0 && idx + 1 < state.chapters.length) {
      await seekToChapter(idx + 1);
    }
  }

  Future<void> previousChapter() async {
    final idx = state.currentChapterIndex;
    if (idx > 0) {
      if (state.chapterPosition > const Duration(seconds: 3)) {
        await seekToChapter(idx);
      } else {
        await seekToChapter(idx - 1);
      }
    } else if (idx == 0) {
      await seekToChapter(0);
    }
  }

  void setSleepTimer(SleepTimerOption option) {
    _sleepTimerTicker?.cancel();
    if (option == SleepTimerOption.none) {
      _engine.setVolume(1.0);
      state = state.copyWith(
        sleepTimerOption: SleepTimerOption.none,
        clearSleepTimer: true,
        volume: 1.0,
      );
      return;
    }

    Duration totalDuration;
    if (option == SleepTimerOption.endOfChapter) {
      final remaining = state.chapterDuration - state.chapterPosition;
      totalDuration = remaining > Duration.zero ? remaining : const Duration(minutes: 5);
    } else {
      totalDuration = option.duration ?? const Duration(minutes: 15);
    }

    state = state.copyWith(
      sleepTimerOption: option,
      sleepTimerRemaining: totalDuration,
      volume: 1.0,
    );
    _engine.setVolume(1.0);

    _sleepTimerTicker = Timer.periodic(const Duration(seconds: 1), (timer) async {
      final remaining = state.sleepTimerRemaining;
      if (remaining == null || remaining <= const Duration(seconds: 1)) {
        timer.cancel();
        await pause();
        _engine.setVolume(1.0);
        state = state.copyWith(
          sleepTimerOption: SleepTimerOption.none,
          clearSleepTimer: true,
          volume: 1.0,
        );
      } else {
        final nextRemaining = remaining - const Duration(seconds: 1);
        double volume = 1.0;
        if (nextRemaining.inSeconds <= 30) {
          volume = (nextRemaining.inSeconds / 30.0).clamp(0.0, 1.0);
          await _engine.setVolume(volume);
        }
        state = state.copyWith(
          sleepTimerRemaining: nextRemaining,
          volume: volume,
        );
      }
    });
  }

  void cancelSleepTimer() {
    setSleepTimer(SleepTimerOption.none);
  }

  void _startPeriodicSync() {
    _syncTimer?.cancel();
    _syncTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      if (state.isPlaying) {
        _syncProgressToServer();
      }
    });
  }

  Future<void> _syncProgressToServer() async {
    final book = state.book;
    if (book == null) return;
    final posSec = state.position.inMilliseconds / 1000.0;
    await _repository.saveProgress(
      book.id,
      positionSeconds: posSec,
      speed: state.speed,
      isCompleted: state.isCompleted,
    );
  }

  Future<void> _onPlaybackCompleted() async {
    _syncTimer?.cancel();
    _sleepTimerTicker?.cancel();
    state = state.copyWith(
      status: AudioPlaybackStatus.completed,
      isCompleted: true,
      position: state.duration,
    );
    final book = state.book;
    if (book != null) {
      await _repository.saveProgress(
        book.id,
        positionSeconds: state.duration.inMilliseconds / 1000.0,
        speed: state.speed,
        isCompleted: true,
      );
    }
  }
}

final audioPlayerProvider = NotifierProvider<AudioPlayerNotifier, AudioPlayerState>(
  () => AudioPlayerNotifier(),
);
