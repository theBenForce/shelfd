import 'dart:async';

import '../../data/models/cast_models.dart';
import '../../data/services/api_service.dart';
import 'audio_player_provider.dart';
import 'audio_player_state.dart';

/// AudioPlayerEngine backed by a server-side Shelfd CastV2 session.
///
/// Interpolates playback position at 100 ms intervals using a monotonic
/// [Stopwatch] between server SSE [CastSessionSnapshot] anchors so read-along
/// word highlights and page turns remain smooth.
class CastAudioPlayerEngine implements AudioPlayerEngine {
  final ApiService _api;
  final Duration tickInterval;

  final _positionController = StreamController<Duration>.broadcast();
  final _durationController = StreamController<Duration?>.broadcast();
  final _statusController = StreamController<AudioPlaybackStatus>.broadcast();
  final Stopwatch _clock = Stopwatch();

  Timer? _tickTimer;
  Duration _anchorPosition = Duration.zero;
  Duration _lastEmittedPosition = Duration.zero;
  Duration? _duration;
  double _speed = 1.0;
  AudioPlaybackStatus _status = AudioPlaybackStatus.idle;
  bool _disposed = false;

  CastAudioPlayerEngine(
    this._api, {
    this.tickInterval = const Duration(milliseconds: 100),
  }) {
    _tickTimer = Timer.periodic(tickInterval, (_) => _onTick());
  }

  void _onTick() {
    if (_disposed || _status != AudioPlaybackStatus.playing) return;
    final pos = currentInterpolatedPosition();
    _lastEmittedPosition = pos;
    _positionController.add(pos);
  }

  Duration currentInterpolatedPosition() {
    if (_status != AudioPlaybackStatus.playing) {
      return _anchorPosition;
    }
    final elapsedMs = (_clock.elapsedMilliseconds * _speed).round();
    var targetMs = _anchorPosition.inMilliseconds + elapsedMs;
    if (_duration != null && _duration!.inMilliseconds > 0) {
      targetMs = targetMs.clamp(0, _duration!.inMilliseconds);
    }
    if (targetMs < 0) targetMs = 0;
    return Duration(milliseconds: targetMs);
  }

  /// Applies an authoritative snapshot from REST or SSE.
  void applyServerSnapshot(
    CastSessionSnapshot snapshot, {
    bool forceSnap = false,
  }) {
    if (_disposed) return;

    if (snapshot.durationSeconds > 0) {
      _duration =
          Duration(milliseconds: (snapshot.durationSeconds * 1000).round());
      _durationController.add(_duration);
    }

    _speed = snapshot.speed > 0 ? snapshot.speed : 1.0;
    final nextStatus = snapshot.toAudioPlaybackStatus();
    final serverPos =
        Duration(milliseconds: (snapshot.positionSeconds * 1000).round());

    final prevInterpolated = currentInterpolatedPosition();
    final driftMs = (serverPos.inMilliseconds - prevInterpolated.inMilliseconds).abs();
    final stateChanged = nextStatus != _status;

    // Absorb minor steady-state network jitter (< 250 ms) without backwards jumps;
    // snap immediately on seeks, state changes, or larger drift.
    final shouldSnap = forceSnap ||
        stateChanged ||
        nextStatus != AudioPlaybackStatus.playing ||
        driftMs >= 250 ||
        serverPos > prevInterpolated;

    _status = nextStatus;
    if (shouldSnap) {
      _anchorPosition = serverPos;
      _clock
        ..reset()
        ..start();
      _lastEmittedPosition = serverPos;
      _positionController.add(serverPos);
    } else if (_lastEmittedPosition != prevInterpolated) {
      _positionController.add(prevInterpolated);
    }

    _statusController.add(_status);
  }

  @override
  Stream<Duration> get positionStream => _positionController.stream;

  @override
  Stream<Duration?> get durationStream => _durationController.stream;

  @override
  Stream<AudioPlaybackStatus> get statusStream => _statusController.stream;

  @override
  Future<void> setUrl(
    String url, {
    Duration? initialPosition,
    AudioMediaMetadata? metadata,
  }) async {
    if (initialPosition != null) {
      _anchorPosition = initialPosition;
      _lastEmittedPosition = initialPosition;
      _clock.reset();
      if (!_disposed) {
        _positionController.add(initialPosition);
      }
    }
  }

  @override
  Future<void> play() async {
    final snap = await _api.controlCastSession(action: 'play');
    if (snap != null) {
      applyServerSnapshot(snap, forceSnap: true);
    }
  }

  @override
  Future<void> pause() async {
    final snap = await _api.controlCastSession(action: 'pause');
    if (snap != null) {
      applyServerSnapshot(snap, forceSnap: true);
    }
  }

  @override
  Future<void> stop() async {
    await _api.stopCastSession();
    _status = AudioPlaybackStatus.idle;
    _clock.stop();
    if (!_disposed) {
      _statusController.add(_status);
    }
  }

  @override
  Future<void> seek(Duration position) async {
    _anchorPosition = position;
    _lastEmittedPosition = position;
    _clock
      ..reset()
      ..start();
    if (!_disposed) {
      _positionController.add(position);
    }
    final snap = await _api.controlCastSession(
      action: 'seek',
      positionSeconds: position.inMilliseconds / 1000.0,
    );
    if (snap != null) {
      applyServerSnapshot(snap, forceSnap: true);
    }
  }

  @override
  Future<void> setSpeed(double speed) async {
    _anchorPosition = currentInterpolatedPosition();
    _speed = speed > 0 ? speed : 1.0;
    _clock
      ..reset()
      ..start();
    final snap = await _api.controlCastSession(action: 'speed', speed: _speed);
    if (snap != null) {
      applyServerSnapshot(snap, forceSnap: true);
    }
  }

  @override
  Future<void> setVolume(double volume) async {
    await _api.controlCastSession(
      action: 'volume',
      volume: volume.clamp(0.0, 1.0),
    );
  }

  @override
  Future<void> dispose() async {
    _disposed = true;
    _tickTimer?.cancel();
    _clock.stop();
    await _positionController.close();
    await _durationController.close();
    await _statusController.close();
  }
}
