import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/book.dart';
import '../features/reader/readalong_pip_bridge.dart';
import 'providers.dart';

export '../features/reader/readalong_pip_models.dart';

final readalongPipBridgeProvider = Provider<ReadalongPipBridge>((ref) {
  final bridge = createReadalongPipBridge();
  ref.onDispose(bridge.dispose);
  return bridge;
});

class ReadalongPipState {
  final bool isSupported;
  final bool isOpen;
  final String? bookId;

  const ReadalongPipState({
    this.isSupported = false,
    this.isOpen = false,
    this.bookId,
  });

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        other is ReadalongPipState &&
            other.isSupported == isSupported &&
            other.isOpen == isOpen &&
            other.bookId == bookId;
  }

  @override
  int get hashCode => Object.hash(isSupported, isOpen, bookId);
}

class ReadalongPipNotifier extends Notifier<ReadalongPipState> {
  static const List<double> _speedSteps = <double>[1.0, 1.25, 1.5, 2.0];

  Book? _book;
  ReadalongManifest? _manifest;
  SpineItem? _fallbackPage;
  int? _fallbackChapterIndex;
  String? _fallbackChapterTitle;
  bool _isDisposed = false;

  ReadalongPipBridge get _bridge => ref.read(readalongPipBridgeProvider);

  @override
  ReadalongPipState build() {
    _isDisposed = false;
    final bridge = ref.watch(readalongPipBridgeProvider);

    ref.listen<AudioPlayerState>(audioPlayerProvider, (prev, next) {
      if (_isDisposed || !state.isOpen) return;
      final book = _book;
      final manifest = _manifest;
      if (book == null || manifest == null) return;
      if (next.book != null && next.book!.id != book.id) return;
      _bridge.update(_buildSnapshot(book, manifest, next));
    });

    ref.onDispose(() {
      _isDisposed = true;
    });

    return ReadalongPipState(
      isSupported: bridge.isSupported,
      isOpen: false,
    );
  }

  Future<void> togglePopout({
    required Book book,
    required ReadalongManifest manifest,
    SpineItem? currentPage,
    int? currentChapterIndex,
    String? currentChapterTitle,
  }) async {
    if (!_bridge.isSupported) return;
    if (state.isOpen && state.bookId == book.id) {
      close();
      return;
    }

    _book = book;
    _manifest = manifest;
    _fallbackPage = currentPage;
    _fallbackChapterIndex = currentChapterIndex;
    _fallbackChapterTitle = currentChapterTitle;

    final audioState = ref.read(audioPlayerProvider);
    final initialSnapshot = _buildSnapshot(book, manifest, audioState);
    final opened = await _bridge.open(initialSnapshot, _buildCallbacks());
    if (!opened || _isDisposed) return;

    state = ReadalongPipState(
      isSupported: _bridge.isSupported,
      isOpen: true,
      bookId: book.id,
    );

    final latestAudio = ref.read(audioPlayerProvider);
    final needsLoad = latestAudio.book?.id != book.id ||
        latestAudio.status == AudioPlaybackStatus.idle ||
        latestAudio.status == AudioPlaybackStatus.error;
    if (needsLoad) {
      final notifier = ref.read(audioPlayerProvider.notifier);
      notifier.setPlaybackMode(AudioPlaybackMode.readalong);
      double? initialPos;
      final chapterIdx = currentPage?.chapterIndex ?? currentChapterIndex;
      if (chapterIdx != null && chapterIdx > 0) {
        final firstSeg = manifest.firstSegmentForChapter(chapterIdx);
        if (firstSeg != null && firstSeg.globalBegin > 0) {
          initialPos = firstSeg.globalBegin;
        }
      }
      unawaited(
        notifier.loadBook(book, initialPosition: initialPos, autoPlay: false),
      );
    }
  }

  void close() {
    _bridge.close();
    _book = null;
    _manifest = null;
    _fallbackPage = null;
    _fallbackChapterIndex = null;
    _fallbackChapterTitle = null;
    if (!_isDisposed) {
      state = ReadalongPipState(
        isSupported: _bridge.isSupported,
        isOpen: false,
      );
    }
  }

  ReadalongPipCallbacks _buildCallbacks() {
    return ReadalongPipCallbacks(
      onTogglePlayPause: () {
        final notifier = ref.read(audioPlayerProvider.notifier);
        final audioState = ref.read(audioPlayerProvider);
        final book = _book;
        if (book != null &&
            (audioState.book?.id != book.id ||
                audioState.status == AudioPlaybackStatus.idle ||
                audioState.status == AudioPlaybackStatus.error)) {
          notifier.setPlaybackMode(AudioPlaybackMode.readalong);
          unawaited(notifier.loadBook(book, autoPlay: true));
        } else {
          unawaited(notifier.togglePlayPause());
        }
      },
      onSkipBackward: () {
        unawaited(ref.read(audioPlayerProvider.notifier).skipBackward());
      },
      onSkipForward: () {
        unawaited(ref.read(audioPlayerProvider.notifier).skipForward());
      },
      onSeek: (position) {
        unawaited(ref.read(audioPlayerProvider.notifier).seek(position));
      },
      onCycleSpeed: () {
        final currentSpeed = ref.read(audioPlayerProvider).speed;
        var nextSpeed = _speedSteps.first;
        for (final step in _speedSteps) {
          if (step > currentSpeed + 0.01) {
            nextSpeed = step;
            break;
          }
        }
        unawaited(ref.read(audioPlayerProvider.notifier).setSpeed(nextSpeed));
      },
      onClosed: () {
        _book = null;
        _manifest = null;
        _fallbackPage = null;
        _fallbackChapterIndex = null;
        _fallbackChapterTitle = null;
        if (!_isDisposed) {
          state = ReadalongPipState(
            isSupported: _bridge.isSupported,
            isOpen: false,
          );
        }
      },
    );
  }

  static String _normalizeHref(String raw) {
    var s = raw.trim().replaceAll('\\', '/');
    final hashIdx = s.indexOf('#');
    if (hashIdx >= 0) {
      s = s.substring(0, hashIdx);
    }
    while (s.startsWith('./')) {
      s = s.substring(2);
    }
    while (s.startsWith('/')) {
      s = s.substring(1);
    }
    return s.toLowerCase();
  }

  SpineItem? _findPageForSegment(Book book, ReadalongSegment? seg) {
    if (seg == null || book.spine.isEmpty) return null;
    final segHref = _normalizeHref(seg.chapterHref);
    if (segHref.isNotEmpty) {
      for (final item in book.spine) {
        final itemHref = _normalizeHref(item.href ?? '');
        if (itemHref.isNotEmpty && itemHref == segHref) {
          return item;
        }
      }
      final segBase = segHref.split('/').last;
      if (segBase.isNotEmpty) {
        final matches = book.spine.where((item) {
          final itemHref = _normalizeHref(item.href ?? '');
          return itemHref.isNotEmpty && itemHref.split('/').last == segBase;
        }).toList();
        if (matches.length == 1) {
          return matches.first;
        }
      }
    }
    if (seg.chapterIndex > 0) {
      for (final item in book.spine) {
        if (item.chapterIndex == seg.chapterIndex) {
          return item;
        }
      }
      if (seg.chapterIndex - 1 < book.spine.length) {
        return book.spine[seg.chapterIndex - 1];
      }
    }
    return null;
  }

  ReadalongPipSnapshot _buildSnapshot(
    Book book,
    ReadalongManifest manifest,
    AudioPlayerState audioState,
  ) {
    final apiService = ref.read(apiServiceProvider);
    final fallbackOrigin = kIsWeb ? Uri.base.origin : null;
    final isSameBook = audioState.book?.id == book.id;

    final position = isSameBook ? audioState.position : Duration.zero;
    final duration = isSameBook && audioState.duration > Duration.zero
        ? audioState.duration
        : (book.durationSeconds != null && book.durationSeconds! > 0
            ? Duration(milliseconds: (book.durationSeconds! * 1000).round())
            : Duration.zero);
    final isPlaying = isSameBook && audioState.isPlaying;
    final speed = isSameBook ? audioState.speed : 1.0;

    final posSec = position.inMilliseconds / 1000.0;
    final activeSeg = isSameBook ? manifest.segmentAt(posSec) : null;

    final coverUrl = resolveAuthenticatedCoverUrl(
      book,
      baseUrl: apiService.baseUrl,
      token: apiService.token,
      fallbackOrigin: fallbackOrigin,
    );

    if (book.isFixedLayout) {
      final page = _findPageForSegment(book, activeSeg) ??
          _fallbackPage ??
          (book.spine.isNotEmpty ? book.spine.first : null);
      final chapterIdentifier = page != null
          ? (page.id.isNotEmpty ? page.id : page.chapterIndex.toString())
          : (activeSeg != null && activeSeg.chapterIndex > 0
              ? activeSeg.chapterIndex.toString()
              : '1');
      final pageUrl = buildFixedLayoutPageUrl(
        baseUrl: apiService.baseUrl,
        bookId: book.id,
        chapterIdentifier: chapterIdentifier,
        token: apiService.token,
        fallbackOrigin: fallbackOrigin,
      );
      final chapterHref = (page?.href?.isNotEmpty ?? false)
          ? page!.href!
          : (activeSeg?.chapterHref ?? '');
      final normPageHref = _normalizeHref(chapterHref);

      final matchingSegments = manifest.segments.where((seg) {
        if (normPageHref.isNotEmpty && _normalizeHref(seg.chapterHref) == normPageHref) {
          return true;
        }
        if (page != null && page.chapterIndex > 0 && seg.chapterIndex == page.chapterIndex) {
          return true;
        }
        return false;
      }).toList();
      final effectiveSegments = matchingSegments.isNotEmpty ? matchingSegments : manifest.segments;
      final pipSegments = effectiveSegments
          .map(
            (s) => ReadalongPipSegment(
              text: s.text,
              globalBegin: s.globalBegin,
              fragmentId: s.fragmentId,
              chapterIndex: s.chapterIndex,
            ),
          )
          .toList();

      final chapterTitle = (page != null && page.title.trim().isNotEmpty)
          ? page.title.trim()
          : (page != null && page.chapterIndex > 0
              ? 'Page ${page.chapterIndex}'
              : (audioState.activeChapter?.title.trim().isNotEmpty == true
                  ? audioState.activeChapter!.title.trim()
                  : (_fallbackChapterTitle ?? '')));

      return ReadalongPipSnapshot(
        bookId: book.id,
        bookTitle: book.title,
        authorDisplay: book.authorDisplay,
        coverUrl: coverUrl,
        chapterTitle: chapterTitle,
        isFixedLayout: true,
        pageUrl: pageUrl,
        chapterHref: chapterHref,
        fragmentId: activeSeg?.fragmentId ?? '',
        activeClass: manifest.activeClass,
        playbackActiveClass: manifest.playbackActiveClass,
        activeSegmentIndex: -1,
        segments: pipSegments,
        isPlaying: isPlaying,
        position: position,
        duration: duration,
        speed: speed,
      );
    }

    final targetChapterIndex = (activeSeg != null && activeSeg.chapterIndex > 0)
        ? activeSeg.chapterIndex
        : (_fallbackChapterIndex != null && _fallbackChapterIndex! > 0
            ? _fallbackChapterIndex!
            : (audioState.activeChapter != null
                ? audioState.activeChapter!.chapterIndex + 1
                : (manifest.segments.isNotEmpty
                    ? manifest.segments.first.chapterIndex
                    : 1)));

    final chapterSegs = manifest.segments
        .where((s) => s.chapterIndex == targetChapterIndex && s.text.trim().isNotEmpty)
        .toList();
    final rawSegs = chapterSegs.isNotEmpty
        ? chapterSegs
        : manifest.segments.where((s) => s.text.trim().isNotEmpty).toList();

    final pipSegments = rawSegs
        .map(
          (s) => ReadalongPipSegment(
            text: s.text,
            globalBegin: s.globalBegin,
            fragmentId: s.fragmentId,
            chapterIndex: s.chapterIndex,
          ),
        )
        .toList();

    var activeIdx = -1;
    if (activeSeg != null) {
      for (var i = 0; i < rawSegs.length; i++) {
        final s = rawSegs[i];
        if ((activeSeg.fragmentId.isNotEmpty && s.fragmentId == activeSeg.fragmentId) ||
            (s.globalBegin == activeSeg.globalBegin && s.text == activeSeg.text)) {
          activeIdx = i;
          break;
        }
      }
    }

    String chapterTitle = '';
    if (isSameBook && (audioState.activeChapter?.title.trim().isNotEmpty ?? false)) {
      chapterTitle = audioState.activeChapter!.title.trim();
    } else {
      for (final spineItem in book.spine) {
        if (spineItem.chapterIndex == targetChapterIndex &&
            spineItem.title.trim().isNotEmpty) {
          chapterTitle = spineItem.title.trim();
          break;
        }
      }
    }
    if (chapterTitle.isEmpty) {
      chapterTitle = (_fallbackChapterTitle?.trim().isNotEmpty ?? false)
          ? _fallbackChapterTitle!.trim()
          : 'Chapter $targetChapterIndex';
    }

    return ReadalongPipSnapshot(
      bookId: book.id,
      bookTitle: book.title,
      authorDisplay: book.authorDisplay,
      coverUrl: coverUrl,
      chapterTitle: chapterTitle,
      isFixedLayout: false,
      chapterHref: activeSeg?.chapterHref ?? '',
      fragmentId: activeSeg?.fragmentId ?? '',
      activeClass: manifest.activeClass,
      playbackActiveClass: manifest.playbackActiveClass,
      activeSegmentIndex: activeIdx,
      segments: pipSegments,
      isPlaying: isPlaying,
      position: position,
      duration: duration,
      speed: speed,
    );
  }
}

final readalongPipProvider =
    NotifierProvider<ReadalongPipNotifier, ReadalongPipState>(
  ReadalongPipNotifier.new,
);
