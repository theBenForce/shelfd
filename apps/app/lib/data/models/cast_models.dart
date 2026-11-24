import '../../ui/state/audio_player_state.dart';

enum CastSessionPhase {
  idle,
  connecting,
  launching,
  loading,
  playing,
  paused,
  buffering,
  error;

  static CastSessionPhase fromString(String? raw) {
    switch (raw?.toLowerCase()) {
      case 'connecting':
        return CastSessionPhase.connecting;
      case 'launching':
        return CastSessionPhase.launching;
      case 'loading':
        return CastSessionPhase.loading;
      case 'playing':
        return CastSessionPhase.playing;
      case 'paused':
        return CastSessionPhase.paused;
      case 'buffering':
        return CastSessionPhase.buffering;
      case 'error':
        return CastSessionPhase.error;
      default:
        return CastSessionPhase.idle;
    }
  }
}

class CastDevice {
  final String id;
  final String name;
  final String model;
  final String host;
  final int port;
  final bool hasVideo;
  final bool hasAudio;
  final bool isStatic;

  const CastDevice({
    required this.id,
    required this.name,
    this.model = '',
    required this.host,
    this.port = 8009,
    this.hasVideo = false,
    this.hasAudio = true,
    this.isStatic = false,
  });

  factory CastDevice.fromJson(Map<String, dynamic> json) {
    return CastDevice(
      id: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? 'Cast Device',
      model: json['model']?.toString() ?? '',
      host: json['host']?.toString() ?? '',
      port: (json['port'] as num?)?.toInt() ?? 8009,
      hasVideo: json['has_video'] as bool? ?? false,
      hasAudio: json['has_audio'] as bool? ?? true,
      isStatic: json['is_static'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'model': model,
        'host': host,
        'port': port,
        'has_video': hasVideo,
        'has_audio': hasAudio,
        'is_static': isStatic,
      };

  @override
  bool operator ==(Object other) => other is CastDevice && other.id == id;

  @override
  int get hashCode => id.hashCode;
}

class CastSessionSnapshot {
  final String sessionId;
  final String userId;
  final CastDevice device;
  final String bookId;
  final String? fileId;
  final CastSessionPhase phase;
  final double positionSeconds;
  final double durationSeconds;
  final double speed;
  final double volume;
  final bool muted;
  final bool captionsEnabled;
  final String? errorMessage;
  final DateTime? updatedAt;

  const CastSessionSnapshot({
    required this.sessionId,
    required this.userId,
    required this.device,
    required this.bookId,
    this.fileId,
    required this.phase,
    required this.positionSeconds,
    required this.durationSeconds,
    this.speed = 1.0,
    this.volume = 1.0,
    this.muted = false,
    this.captionsEnabled = false,
    this.errorMessage,
    this.updatedAt,
  });

  factory CastSessionSnapshot.fromJson(Map<String, dynamic> json) {
    return CastSessionSnapshot(
      sessionId: json['session_id']?.toString() ?? '',
      userId: json['user_id']?.toString() ?? '',
      device: CastDevice.fromJson(
        (json['device'] as Map<String, dynamic>?) ?? const {},
      ),
      bookId: json['book_id']?.toString() ?? '',
      fileId: json['file_id']?.toString(),
      phase: CastSessionPhase.fromString(json['phase']?.toString()),
      positionSeconds: (json['position_seconds'] as num?)?.toDouble() ?? 0.0,
      durationSeconds: (json['duration_seconds'] as num?)?.toDouble() ?? 0.0,
      speed: (json['speed'] as num?)?.toDouble() ?? 1.0,
      volume: (json['volume'] as num?)?.toDouble() ?? 1.0,
      muted: json['muted'] as bool? ?? false,
      captionsEnabled: json['captions_enabled'] as bool? ?? false,
      errorMessage: json['error_message']?.toString(),
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString())
          : null,
    );
  }

  bool get isActive =>
      phase != CastSessionPhase.idle && phase != CastSessionPhase.error;

  AudioPlaybackStatus toAudioPlaybackStatus() {
    switch (phase) {
      case CastSessionPhase.connecting:
      case CastSessionPhase.launching:
      case CastSessionPhase.loading:
      case CastSessionPhase.buffering:
        return AudioPlaybackStatus.loading;
      case CastSessionPhase.playing:
        return AudioPlaybackStatus.playing;
      case CastSessionPhase.paused:
        return AudioPlaybackStatus.paused;
      case CastSessionPhase.error:
        return AudioPlaybackStatus.error;
      case CastSessionPhase.idle:
        return AudioPlaybackStatus.idle;
    }
  }
}
