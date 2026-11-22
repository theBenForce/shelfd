import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'package:web/web.dart' as web;
import 'audio_player_state.dart';
import 'media_session_metadata.dart';

MediaSessionBridge createMediaSessionBridge() => WebMediaSessionBridge();

class WebMediaSessionBridge implements MediaSessionBridge {
  static const List<String> _actions = <String>[
    'play',
    'pause',
    'stop',
    'seekbackward',
    'seekforward',
    'seekto',
    'previoustrack',
    'nexttrack',
  ];

  MediaSessionActionHandlers? _handlers;
  bool _disposed = false;

  web.MediaSession? get _mediaSession {
    try {
      final nav = web.window.navigator;
      if (!nav.has('mediaSession')) return null;
      return nav.mediaSession;
    } catch (_) {
      return null;
    }
  }

  void _setHandler(web.MediaSession session, String action, JSFunction? fn) {
    try {
      session.setActionHandler(action, fn);
    } catch (_) {
      // Some browsers reject individual MediaSession actions.
    }
  }

  double? _readNumericField(JSAny? details, String field) {
    if (details == null || details.isUndefinedOrNull) return null;
    try {
      final obj = details as JSObject;
      if (!obj.has(field)) return null;
      final value = obj.getProperty<JSAny?>(field.toJS);
      if (value == null || value.isUndefinedOrNull) return null;
      if (value.isA<JSNumber>()) {
        final d = (value as JSNumber).toDartDouble;
        return d.isFinite ? d : null;
      }
    } catch (_) {}
    return null;
  }

  @override
  void bindHandlers(MediaSessionActionHandlers handlers) {
    if (_disposed) return;
    _handlers = handlers;
    final session = _mediaSession;
    if (session == null) return;

    _setHandler(
      session,
      'play',
      ((JSAny? _) {
        _handlers?.onPlay?.call();
      }).toJS,
    );
    _setHandler(
      session,
      'pause',
      ((JSAny? _) {
        _handlers?.onPause?.call();
      }).toJS,
    );
    _setHandler(
      session,
      'stop',
      ((JSAny? _) {
        _handlers?.onStop?.call();
      }).toJS,
    );
    _setHandler(
      session,
      'seekbackward',
      ((JSAny? details) {
        final offset = _readNumericField(details, 'seekOffset');
        final seconds = (offset != null && offset > 0) ? offset.round() : 15;
        _handlers?.onSkipBackward?.call(seconds);
      }).toJS,
    );
    _setHandler(
      session,
      'seekforward',
      ((JSAny? details) {
        final offset = _readNumericField(details, 'seekOffset');
        final seconds = (offset != null && offset > 0) ? offset.round() : 30;
        _handlers?.onSkipForward?.call(seconds);
      }).toJS,
    );
    _setHandler(
      session,
      'seekto',
      ((JSAny? details) {
        final seekTime = _readNumericField(details, 'seekTime');
        if (seekTime != null && seekTime >= 0) {
          _handlers?.onSeek?.call(
            Duration(milliseconds: (seekTime * 1000).round()),
          );
        }
      }).toJS,
    );
    _setHandler(
      session,
      'previoustrack',
      ((JSAny? _) {
        _handlers?.onPreviousChapter?.call();
      }).toJS,
    );
    _setHandler(
      session,
      'nexttrack',
      ((JSAny? _) {
        _handlers?.onNextChapter?.call();
      }).toJS,
    );
  }

  @override
  void updateMetadata(AudioMediaMetadata metadata) {
    if (_disposed) return;
    final session = _mediaSession;
    if (session == null) return;

    try {
      final artworkUrl = metadata.artworkUrl;
      final init = (artworkUrl != null && artworkUrl.isNotEmpty)
          ? web.MediaMetadataInit(
              title: metadata.title,
              artist: metadata.artist,
              album: metadata.album,
              artwork: <web.MediaImage>[
                web.MediaImage(src: artworkUrl),
              ].toJS,
            )
          : web.MediaMetadataInit(
              title: metadata.title,
              artist: metadata.artist,
              album: metadata.album,
            );
      session.metadata = web.MediaMetadata(init);
    } catch (_) {}
  }

  @override
  void updatePlaybackState({
    required AudioPlaybackStatus status,
    required Duration position,
    required Duration duration,
    required double speed,
  }) {
    if (_disposed) return;
    final session = _mediaSession;
    if (session == null) return;

    try {
      final playbackState = switch (status) {
        AudioPlaybackStatus.playing => 'playing',
        AudioPlaybackStatus.paused ||
        AudioPlaybackStatus.ready ||
        AudioPlaybackStatus.loading ||
        AudioPlaybackStatus.completed =>
          'paused',
        AudioPlaybackStatus.idle || AudioPlaybackStatus.error => 'none',
      };
      session.playbackState = playbackState;

      if (duration > Duration.zero &&
          speed > 0 &&
          status != AudioPlaybackStatus.idle &&
          status != AudioPlaybackStatus.error) {
        final durationSec = duration.inMilliseconds / 1000.0;
        final clampedPosMs = position.inMilliseconds.clamp(0, duration.inMilliseconds);
        final positionSec = clampedPosMs / 1000.0;
        session.setPositionState(
          web.MediaPositionState(
            duration: durationSec,
            playbackRate: speed,
            position: positionSec,
          ),
        );
      }
    } catch (_) {}
  }

  @override
  void clear() {
    final session = _mediaSession;
    if (session == null) return;

    try {
      session.metadata = null;
      session.playbackState = 'none';
    } catch (_) {}

    for (final action in _actions) {
      _setHandler(session, action, null);
    }
  }

  @override
  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _handlers = null;
    clear();
  }
}
