import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/models/cast_models.dart';
import '../../data/services/api_service.dart';
import 'audio_player_provider.dart';
import 'cast_audio_player_engine.dart';
import 'providers.dart';

class CastState {
  final List<CastDevice> devices;
  final CastSessionSnapshot? session;
  final bool isDiscovering;
  final bool isBusy;
  final String? errorMessage;

  const CastState({
    this.devices = const [],
    this.session,
    this.isDiscovering = false,
    this.isBusy = false,
    this.errorMessage,
  });

  bool get isConnected => session != null && session!.isActive;
  CastDevice? get activeDevice => isConnected ? session!.device : null;

  CastState copyWith({
    List<CastDevice>? devices,
    CastSessionSnapshot? session,
    bool clearSession = false,
    bool? isDiscovering,
    bool? isBusy,
    String? errorMessage,
    bool clearError = false,
  }) {
    return CastState(
      devices: devices ?? this.devices,
      session: clearSession ? null : (session ?? this.session),
      isDiscovering: isDiscovering ?? this.isDiscovering,
      isBusy: isBusy ?? this.isBusy,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}

final castAudioEngineProvider = Provider<CastAudioPlayerEngine>((ref) {
  final api = ref.read(apiServiceProvider);
  final engine = CastAudioPlayerEngine(api);
  ref.onDispose(() {
    unawaited(engine.dispose());
  });
  return engine;
});

class CastNotifier extends Notifier<CastState> {
  Timer? _reconcileTimer;
  bool _isDisposed = false;

  ApiService get _api => ref.read(apiServiceProvider);
  CastAudioPlayerEngine get _castEngine => ref.read(castAudioEngineProvider);

  @override
  CastState build() {
    _isDisposed = false;
    // Keep SSE connected even when mounted on fullscreen /listen or /read routes outside AppShell.
    ref.watch(queueProvider);

    ref.onDispose(() {
      _isDisposed = true;
      _reconcileTimer?.cancel();
    });

    unawaited(_bootstrap());
    _reconcileTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (!_isDisposed && state.isConnected) {
        unawaited(refreshSession());
      }
    });

    return const CastState();
  }

  Future<void> _bootstrap() async {
    await refreshSession();
    await refreshDevices(refresh: false);
  }

  Future<void> refreshDevices({bool refresh = true}) async {
    if (_isDisposed) return;
    state = state.copyWith(isDiscovering: true, clearError: true);
    try {
      final devices = await _api.getCastDevices(refresh: refresh);
      if (_isDisposed) return;
      state = state.copyWith(devices: devices, isDiscovering: false);
    } catch (e) {
      if (_isDisposed) return;
      state = state.copyWith(
        isDiscovering: false,
        errorMessage: 'Failed to scan Cast devices: $e',
      );
    }
  }

  Future<void> refreshSession() async {
    if (_isDisposed) return;
    try {
      final snap = await _api.getCastSession();
      if (_isDisposed) return;
      if (snap != null && snap.isActive) {
        handleSessionEvent(snap);
      } else if (state.isConnected) {
        state = state.copyWith(clearSession: true);
        await ref.read(audioPlayerProvider.notifier).handoffToLocal();
      }
    } catch (_) {
      // Ignore transient poll errors; SSE or next poll will reconcile.
    }
  }

  void handleDevicesEvent(List<CastDevice> devices) {
    if (_isDisposed) return;
    state = state.copyWith(devices: devices, isDiscovering: false);
  }

  void handleSessionEvent(CastSessionSnapshot snapshot) {
    if (_isDisposed) return;
    if (!snapshot.isActive) {
      final wasConnected = state.isConnected;
      state = state.copyWith(
        clearSession: true,
        errorMessage: snapshot.errorMessage,
      );
      if (wasConnected) {
        unawaited(ref.read(audioPlayerProvider.notifier).handoffToLocal());
      }
      return;
    }

    state = state.copyWith(session: snapshot, clearError: true);
    _castEngine.applyServerSnapshot(snapshot);
    unawaited(ref.read(audioPlayerProvider.notifier).adoptCastSession(snapshot));
  }

  Future<void> connectToDevice(
    CastDevice device, {
    bool? captionsEnabled,
  }) async {
    if (_isDisposed) return;
    final playerState = ref.read(audioPlayerProvider);
    final book = playerState.book;
    if (book == null) return;

    state = state.copyWith(isBusy: true, clearError: true);
    try {
      final wantCaptions = captionsEnabled ??
          (device.hasVideo &&
              (book.hasReadalong ||
                  playerState.playbackMode == AudioPlaybackMode.readalong));
      final snap = await _api.startCastSession(
        deviceId: device.id,
        bookId: book.id,
        positionSeconds: playerState.position.inMilliseconds / 1000.0,
        speed: playerState.speed,
        captionsEnabled: wantCaptions,
      );
      if (_isDisposed) return;
      state = state.copyWith(session: snap, isBusy: false);
      _castEngine.applyServerSnapshot(snap, forceSnap: true);
      await ref
          .read(audioPlayerProvider.notifier)
          .handoffToCast(device, snapshot: snap);
    } catch (e) {
      if (_isDisposed) return;
      state = state.copyWith(
        isBusy: false,
        errorMessage: 'Failed to connect to ${device.name}: $e',
      );
    }
  }

  Future<void> disconnectToLocal() async {
    if (_isDisposed) return;
    state = state.copyWith(isBusy: true, clearError: true);
    try {
      await _api.stopCastSession();
    } catch (_) {
      // Always restore local engine even if receiver already dropped.
    }
    if (_isDisposed) return;
    state = state.copyWith(clearSession: true, isBusy: false);
    await ref.read(audioPlayerProvider.notifier).handoffToLocal();
  }

  Future<void> setCastVolume(double volume) async {
    if (_isDisposed || !state.isConnected) return;
    try {
      final snap = await _api.controlCastSession(
        action: 'volume',
        volume: volume.clamp(0.0, 1.0),
      );
      if (_isDisposed || snap == null) return;
      state = state.copyWith(session: snap);
    } catch (_) {}
  }

  Future<void> setCaptionsEnabled(bool enabled) async {
    if (_isDisposed || !state.isConnected) return;
    try {
      final snap = await _api.controlCastSession(
        action: 'captions',
        captionsEnabled: enabled,
      );
      if (_isDisposed || snap == null) return;
      state = state.copyWith(session: snap);
      _castEngine.applyServerSnapshot(snap);
    } catch (_) {}
  }
}

final castProvider = NotifierProvider<CastNotifier, CastState>(
  () => CastNotifier(),
);
