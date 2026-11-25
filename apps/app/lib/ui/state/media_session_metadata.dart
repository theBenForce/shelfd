import 'package:flutter/foundation.dart';
import '../../data/models/book.dart';
import '../../data/utils/url_normalizer.dart';
import 'audio_player_state.dart';

String? resolveAuthenticatedCoverUrl(
  Book book, {
  required String baseUrl,
  String? token,
  String? fallbackOrigin,
  bool isWeb = kIsWeb,
}) {
  final raw = book.coverUrl?.trim();
  if (raw == null || raw.isEmpty) return null;

  final effectiveOrigin = baseUrl.trim().isNotEmpty
      ? baseUrl.trim()
      : (fallbackOrigin?.trim() ?? '');

  var resolved = raw;
  if (resolved.startsWith('/') && !resolved.startsWith('//')) {
    if (effectiveOrigin.isNotEmpty) {
      final cleanOrigin = effectiveOrigin.endsWith('/')
          ? effectiveOrigin.substring(0, effectiveOrigin.length - 1)
          : effectiveOrigin;
      resolved = '$cleanOrigin$resolved';
    }
  }

  final uri = Uri.tryParse(resolved);
  if (uri == null) return resolved;

  final cleanToken = token?.trim();
  if (isWeb &&
      cleanToken != null &&
      cleanToken.isNotEmpty &&
      (uri.queryParameters['token'] ?? '').isEmpty &&
      (effectiveOrigin.isEmpty ||
          UrlNormalizer.isSameOrigin(effectiveOrigin, resolved))) {
    return uri.replace(
      queryParameters: {
        ...uri.queryParameters,
        'token': cleanToken,
      },
    ).toString();
  }

  return uri.toString();
}

Map<String, String>? resolveAuthenticatedArtworkHeaders({
  required String? artworkUrl,
  required String baseUrl,
  String? token,
  String? fallbackOrigin,
  bool isWeb = kIsWeb,
}) {
  if (isWeb || artworkUrl == null || artworkUrl.trim().isEmpty) {
    return null;
  }
  final cleanToken = token?.trim();
  if (cleanToken == null || cleanToken.isEmpty) {
    return null;
  }
  final effectiveOrigin = baseUrl.trim().isNotEmpty
      ? baseUrl.trim()
      : (fallbackOrigin?.trim() ?? '');
  if (effectiveOrigin.isEmpty ||
      !UrlNormalizer.isSameOrigin(effectiveOrigin, artworkUrl)) {
    return null;
  }
  return <String, String>{
    'Authorization': 'Bearer $cleanToken',
  };
}

String _formatSeriesAlbum(Book book) {
  final name = book.series?.name.trim() ?? '';
  if (name.isEmpty) return '';
  final seq = book.seriesSequence;
  if (seq == null) return name;
  final seqLabel =
      seq == seq.truncateToDouble() ? seq.toInt().toString() : seq.toString();
  return '$name #$seqLabel';
}

class AudioMediaMetadata {
  final String id;
  final String title;
  final String artist;
  final String album;
  final String? artworkUrl;
  final Map<String, String>? artworkHeaders;
  final Duration? duration;
  final bool isReadalong;

  const AudioMediaMetadata({
    required this.id,
    required this.title,
    required this.artist,
    required this.album,
    this.artworkUrl,
    this.artworkHeaders,
    this.duration,
    this.isReadalong = false,
  });

  Uri? get artworkUri {
    final url = artworkUrl;
    if (url == null || url.isEmpty) return null;
    return Uri.tryParse(url);
  }

  static AudioMediaMetadata? fromAudioState(
    AudioPlayerState state, {
    required String baseUrl,
    String? token,
    String? fallbackOrigin,
    bool isWeb = kIsWeb,
  }) {
    final book = state.book;
    if (book == null) return null;

    final isReadalong = state.playbackMode == AudioPlaybackMode.readalong;
    final chapterTitle = state.activeChapter?.title.trim() ?? '';
    final seriesLabel = _formatSeriesAlbum(book);
    final album = chapterTitle.isNotEmpty
        ? chapterTitle
        : (seriesLabel.isNotEmpty
            ? seriesLabel
            : (isReadalong ? 'Read-Along' : 'Audiobook'));

    final resolvedCoverUrl = resolveAuthenticatedCoverUrl(
      book,
      baseUrl: baseUrl,
      token: token,
      fallbackOrigin: fallbackOrigin,
      isWeb: isWeb,
    );
    final resolvedHeaders = resolveAuthenticatedArtworkHeaders(
      artworkUrl: resolvedCoverUrl,
      baseUrl: baseUrl,
      token: token,
      fallbackOrigin: fallbackOrigin,
      isWeb: isWeb,
    );

    return AudioMediaMetadata(
      id: book.id,
      title: book.title,
      artist: book.authorDisplay.isNotEmpty ? book.authorDisplay : 'Shelfd',
      album: album,
      artworkUrl: resolvedCoverUrl,
      artworkHeaders: resolvedHeaders,
      duration: state.duration > Duration.zero ? state.duration : null,
      isReadalong: isReadalong,
    );
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        other is AudioMediaMetadata &&
            other.id == id &&
            other.title == title &&
            other.artist == artist &&
            other.album == album &&
            other.artworkUrl == artworkUrl &&
            mapEquals(other.artworkHeaders, artworkHeaders) &&
            other.duration == duration &&
            other.isReadalong == isReadalong;
  }

  @override
  int get hashCode => Object.hash(
        id,
        title,
        artist,
        album,
        artworkUrl,
        artworkHeaders == null
            ? null
            : Object.hashAll(
                artworkHeaders!.entries
                    .map((e) => Object.hash(e.key, e.value)),
              ),
        duration,
        isReadalong,
      );
}

class MediaSessionActionHandlers {
  final void Function()? onPlay;
  final void Function()? onPause;
  final void Function()? onStop;
  final void Function(Duration position)? onSeek;
  final void Function(int seconds)? onSkipForward;
  final void Function(int seconds)? onSkipBackward;
  final void Function()? onNextChapter;
  final void Function()? onPreviousChapter;

  const MediaSessionActionHandlers({
    this.onPlay,
    this.onPause,
    this.onStop,
    this.onSeek,
    this.onSkipForward,
    this.onSkipBackward,
    this.onNextChapter,
    this.onPreviousChapter,
  });
}

abstract class MediaSessionBridge {
  void bindHandlers(MediaSessionActionHandlers handlers);

  void updateMetadata(AudioMediaMetadata metadata);

  void updatePlaybackState({
    required AudioPlaybackStatus status,
    required Duration position,
    required Duration duration,
    required double speed,
  });

  void clear();

  void dispose();
}

class NoopMediaSessionBridge implements MediaSessionBridge {
  @override
  void bindHandlers(MediaSessionActionHandlers handlers) {}

  @override
  void updateMetadata(AudioMediaMetadata metadata) {}

  @override
  void updatePlaybackState({
    required AudioPlaybackStatus status,
    required Duration position,
    required Duration duration,
    required double speed,
  }) {}

  @override
  void clear() {}

  @override
  void dispose() {}
}
