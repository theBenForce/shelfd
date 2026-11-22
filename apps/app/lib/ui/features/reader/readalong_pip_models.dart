String buildFixedLayoutPageUrl({
  required String baseUrl,
  required String bookId,
  required String chapterIdentifier,
  String? token,
  String? fallbackOrigin,
}) {
  final rawOrigin = baseUrl.trim().isNotEmpty
      ? baseUrl.trim()
      : (fallbackOrigin?.trim() ?? '');
  final cleanOrigin = rawOrigin.endsWith('/')
      ? rawOrigin.substring(0, rawOrigin.length - 1)
      : rawOrigin;
  final path = '/api/v1/books/$bookId/chapters/$chapterIdentifier/html';
  final rawUrl = cleanOrigin.isNotEmpty ? '$cleanOrigin$path' : path;
  final uri = Uri.parse(rawUrl);
  final cleanToken = token?.trim();
  final params = <String, String>{
    ...uri.queryParameters,
    'fit': 'scale',
    if (cleanToken != null && cleanToken.isNotEmpty) 'token': cleanToken,
  };
  return uri.replace(queryParameters: params).toString();
}

class ReadalongPipSegment {
  final String text;
  final double globalBegin;
  final String fragmentId;
  final int chapterIndex;

  const ReadalongPipSegment({
    required this.text,
    required this.globalBegin,
    this.fragmentId = '',
    this.chapterIndex = 0,
  });

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        other is ReadalongPipSegment &&
            other.text == text &&
            other.globalBegin == globalBegin &&
            other.fragmentId == fragmentId &&
            other.chapterIndex == chapterIndex;
  }

  @override
  int get hashCode => Object.hash(text, globalBegin, fragmentId, chapterIndex);
}

class ReadalongPipSnapshot {
  final String bookId;
  final String bookTitle;
  final String authorDisplay;
  final String? coverUrl;
  final String chapterTitle;
  final bool isFixedLayout;
  final String? pageUrl;
  final String chapterHref;
  final String fragmentId;
  final String activeClass;
  final String playbackActiveClass;
  final int activeSegmentIndex;
  final List<ReadalongPipSegment> segments;
  final bool isPlaying;
  final Duration position;
  final Duration duration;
  final double speed;

  const ReadalongPipSnapshot({
    required this.bookId,
    required this.bookTitle,
    this.authorDisplay = '',
    this.coverUrl,
    this.chapterTitle = '',
    this.isFixedLayout = false,
    this.pageUrl,
    this.chapterHref = '',
    this.fragmentId = '',
    this.activeClass = '-epub-media-overlay-active',
    this.playbackActiveClass = '-epub-media-overlay-playing',
    this.activeSegmentIndex = -1,
    this.segments = const [],
    this.isPlaying = false,
    this.position = Duration.zero,
    this.duration = Duration.zero,
    this.speed = 1.0,
  });
}

class ReadalongPipCallbacks {
  final void Function() onTogglePlayPause;
  final void Function() onSkipBackward;
  final void Function() onSkipForward;
  final void Function(Duration position) onSeek;
  final void Function() onCycleSpeed;
  final void Function() onClosed;

  const ReadalongPipCallbacks({
    required this.onTogglePlayPause,
    required this.onSkipBackward,
    required this.onSkipForward,
    required this.onSeek,
    required this.onCycleSpeed,
    required this.onClosed,
  });
}

abstract class ReadalongPipBridge {
  bool get isSupported;
  bool get isOpen;

  Future<bool> open(
    ReadalongPipSnapshot initial,
    ReadalongPipCallbacks callbacks,
  );

  void update(ReadalongPipSnapshot snapshot);

  void close();

  void dispose();
}
