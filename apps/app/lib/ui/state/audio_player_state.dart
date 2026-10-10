import '../../data/models/book.dart';

enum AudioPlaybackStatus {
  idle,
  loading,
  ready,
  playing,
  paused,
  completed,
  error,
}

enum SleepTimerOption {
  none,
  min5,
  min10,
  min15,
  min30,
  min45,
  min60,
  endOfChapter;

  String get label {
    switch (this) {
      case SleepTimerOption.none:
        return 'Off';
      case SleepTimerOption.min5:
        return '5 minutes';
      case SleepTimerOption.min10:
        return '10 minutes';
      case SleepTimerOption.min15:
        return '15 minutes';
      case SleepTimerOption.min30:
        return '30 minutes';
      case SleepTimerOption.min45:
        return '45 minutes';
      case SleepTimerOption.min60:
        return '60 minutes';
      case SleepTimerOption.endOfChapter:
        return 'End of chapter';
    }
  }

  Duration? get duration {
    switch (this) {
      case SleepTimerOption.none:
      case SleepTimerOption.endOfChapter:
        return null;
      case SleepTimerOption.min5:
        return const Duration(minutes: 5);
      case SleepTimerOption.min10:
        return const Duration(minutes: 10);
      case SleepTimerOption.min15:
        return const Duration(minutes: 15);
      case SleepTimerOption.min30:
        return const Duration(minutes: 30);
      case SleepTimerOption.min45:
        return const Duration(minutes: 45);
      case SleepTimerOption.min60:
        return const Duration(minutes: 60);
    }
  }
}

class AudioPlayerState {
  final Book? book;
  final AudioPlaybackStatus status;
  final Duration position;
  final Duration duration;
  final double speed;
  final double volume;
  final AudioChapter? activeChapter;
  final List<AudioChapter> chapters;
  final bool isCompleted;
  final Duration? sleepTimerRemaining;
  final SleepTimerOption sleepTimerOption;
  final String? errorMessage;

  const AudioPlayerState({
    this.book,
    this.status = AudioPlaybackStatus.idle,
    this.position = Duration.zero,
    this.duration = Duration.zero,
    this.speed = 1.0,
    this.volume = 1.0,
    this.activeChapter,
    this.chapters = const [],
    this.isCompleted = false,
    this.sleepTimerRemaining,
    this.sleepTimerOption = SleepTimerOption.none,
    this.errorMessage,
  });

  bool get isPlaying => status == AudioPlaybackStatus.playing;
  bool get isLoading => status == AudioPlaybackStatus.loading;
  bool get hasTrack => book != null;

  double get progressFraction {
    if (duration.inMilliseconds <= 0) return 0.0;
    return (position.inMilliseconds / duration.inMilliseconds).clamp(0.0, 1.0);
  }

  int get currentChapterIndex {
    if (activeChapter == null) return -1;
    return chapters.indexOf(activeChapter!);
  }

  Duration get chapterPosition {
    if (activeChapter == null) return position;
    final startMs = (activeChapter!.startOffsetSec * 1000).toInt();
    final posMs = position.inMilliseconds - startMs;
    return Duration(milliseconds: posMs < 0 ? 0 : posMs);
  }

  Duration get chapterDuration {
    if (activeChapter == null) return duration;
    final durMs = (activeChapter!.durationSec * 1000).toInt();
    return Duration(milliseconds: durMs > 0 ? durMs : 0);
  }

  double get chapterProgressFraction {
    if (chapterDuration.inMilliseconds <= 0) return 0.0;
    return (chapterPosition.inMilliseconds / chapterDuration.inMilliseconds).clamp(0.0, 1.0);
  }

  AudioPlayerState copyWith({
    Book? book,
    AudioPlaybackStatus? status,
    Duration? position,
    Duration? duration,
    double? speed,
    double? volume,
    AudioChapter? activeChapter,
    bool clearActiveChapter = false,
    List<AudioChapter>? chapters,
    bool? isCompleted,
    Duration? sleepTimerRemaining,
    bool clearSleepTimer = false,
    SleepTimerOption? sleepTimerOption,
    String? errorMessage,
    bool clearError = false,
  }) {
    return AudioPlayerState(
      book: book ?? this.book,
      status: status ?? this.status,
      position: position ?? this.position,
      duration: duration ?? this.duration,
      speed: speed ?? this.speed,
      volume: volume ?? this.volume,
      activeChapter: clearActiveChapter ? null : (activeChapter ?? this.activeChapter),
      chapters: chapters ?? this.chapters,
      isCompleted: isCompleted ?? this.isCompleted,
      sleepTimerRemaining: clearSleepTimer ? null : (sleepTimerRemaining ?? this.sleepTimerRemaining),
      sleepTimerOption: sleepTimerOption ?? this.sleepTimerOption,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}
