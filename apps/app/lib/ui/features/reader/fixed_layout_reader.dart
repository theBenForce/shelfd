import 'dart:async';
import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../audiobook/widgets/cast_device_sheet.dart';
import 'html_view.dart';

enum SpreadMode {
  auto,
  twoPage,
  singlePage,
}

class FixedSpread {
  final int spreadIndex;
  final SpineItem? leftPage;
  final SpineItem rightPage;
  final bool isSingle;
  final bool isGatefold;

  const FixedSpread({
    required this.spreadIndex,
    this.leftPage,
    required this.rightPage,
    required this.isSingle,
    this.isGatefold = false,
  });

  String get label {
    if (isSingle) {
      final idx = rightPage.chapterIndex > 0 ? rightPage.chapterIndex : spreadIndex + 1;
      return 'Page $idx';
    }
    final leftIdx = leftPage!.chapterIndex > 0 ? leftPage!.chapterIndex : spreadIndex * 2;
    final rightIdx = rightPage.chapterIndex > 0 ? rightPage.chapterIndex : leftIdx + 1;
    return 'Pages $leftIdx–$rightIdx';
  }

  bool containsChapter(dynamic identifier) {
    if (identifier == null) return false;
    final idStr = identifier.toString();
    if (rightPage.id == idStr || rightPage.chapterIndex.toString() == idStr) return true;
    if (leftPage != null && (leftPage!.id == idStr || leftPage!.chapterIndex.toString() == idStr)) return true;
    return false;
  }

  bool containsChapterIndex(int chapterIndex, List<SpineItem> spine) {
    if (chapterIndex <= 0) return false;
    if (rightPage.chapterIndex == chapterIndex) return true;
    if (leftPage != null && leftPage!.chapterIndex == chapterIndex) return true;
    final rightPos = spine.indexOf(rightPage);
    if (rightPos >= 0 && (rightPos + 1) == chapterIndex) return true;
    if (leftPage != null) {
      final leftPos = spine.indexOf(leftPage!);
      if (leftPos >= 0 && (leftPos + 1) == chapterIndex) return true;
    }
    return false;
  }
}

class FixedLayoutReader extends ConsumerStatefulWidget {
  final Book book;
  final dynamic initialChapterIdentifier;
  final VoidCallback? onClose;
  final ReadalongManifest? initialReadalongManifest;

  const FixedLayoutReader({
    super.key,
    required this.book,
    this.initialChapterIdentifier,
    this.onClose,
    this.initialReadalongManifest,
  });

  @override
  ConsumerState<FixedLayoutReader> createState() => _FixedLayoutReaderState();
}

class _FixedLayoutReaderState extends ConsumerState<FixedLayoutReader> {
  SpreadMode _spreadMode = SpreadMode.auto;
  int _currentSpreadIndex = 0;
  final FocusNode _focusNode = FocusNode();
  bool _controlsVisible = true;
  ReadalongManifest? _readalongManifest;
  ReadalongSegment? _activeSegment;
  bool _autoTurnPages = true;
  late bool _cookieReady;
  String? _cookieError;
  Timer? _mediaTokenRefreshTimer;

  String? _lastHighlightedViewKey;
  String? _lastHighlightedFragmentId;
  bool? _lastHighlightedPlaying;

  bool get _hasReadalong =>
      widget.book.canReadAlong ||
      (_readalongManifest?.hasReadalong ?? false) ||
      (_readalongManifest?.segments.isNotEmpty ?? false);

  @override
  void initState() {
    super.initState();
    _readalongManifest = widget.initialReadalongManifest;
    _cookieReady = !requiresNativeMediaCookie;
    _resolveInitialSpread();
    if (!_cookieReady) {
      Future.microtask(_prepareNativeMediaCookie);
    }
    Future.microtask(_loadReadalongManifest);
  }

  Future<void> _prepareNativeMediaCookie() async {
    _mediaTokenRefreshTimer?.cancel();
    if (mounted && _cookieError != null) {
      setState(() {
        _cookieError = null;
      });
    }
    try {
      final apiService = ref.read(apiServiceProvider);
      final mediaToken = await apiService.mintBookMediaToken(widget.book.id);
      await seedHtmlAuthCookie(
        serverBaseUrl: apiService.baseUrl,
        mediaJwt: mediaToken.token,
      );
      final refreshIn = mediaToken.expiresAt
              .toUtc()
              .difference(DateTime.now().toUtc()) -
          const Duration(minutes: 5);
      final clampedRefresh = refreshIn > const Duration(minutes: 1)
          ? refreshIn
          : const Duration(minutes: 55);
      _mediaTokenRefreshTimer = Timer(clampedRefresh, () {
        if (mounted) {
          _prepareNativeMediaCookie();
        }
      });
      if (mounted) {
        setState(() {
          _cookieReady = true;
          _cookieError = null;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _cookieReady = false;
          _cookieError =
              'Unable to authorize book media. Check your server connection and try again.';
        });
      }
    }
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

  String _pageViewKey(SpineItem page) {
    return 'fixed_${widget.book.id}_${page.chapterIndex}';
  }

  SpineItem? _findPageForSegment(ReadalongSegment? seg) {
    if (seg == null) return null;
    final spine = widget.book.spine;
    if (spine.isEmpty) return null;

    final segHref = _normalizeHref(seg.chapterHref);
    if (segHref.isNotEmpty) {
      for (final item in spine) {
        final itemHref = _normalizeHref(item.href ?? '');
        if (itemHref.isNotEmpty && itemHref == segHref) {
          return item;
        }
      }
      final segBase = segHref.split('/').last;
      if (segBase.isNotEmpty) {
        final baseMatches = spine.where((item) {
          final itemHref = _normalizeHref(item.href ?? '');
          return itemHref.isNotEmpty && itemHref.split('/').last == segBase;
        }).toList();
        if (baseMatches.length == 1) {
          return baseMatches.first;
        }
      }
    }

    if (seg.chapterIndex > 0) {
      for (final item in spine) {
        if (item.chapterIndex == seg.chapterIndex) {
          return item;
        }
      }
      if (seg.chapterIndex - 1 < spine.length) {
        return spine[seg.chapterIndex - 1];
      }
    }
    return null;
  }

  @override
  void didUpdateWidget(covariant FixedLayoutReader oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.book.id != widget.book.id && requiresNativeMediaCookie) {
      _cookieReady = false;
      Future.microtask(_prepareNativeMediaCookie);
    }
    if (widget.initialReadalongManifest != null && _readalongManifest == null) {
      _readalongManifest = widget.initialReadalongManifest;
      final audioState = ref.read(audioPlayerProvider);
      if (audioState.book?.id == widget.book.id && audioState.status != AudioPlaybackStatus.idle) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            _onAudioStateChanged(
              ref.read(audioPlayerProvider),
              forceSyncSpread: widget.initialChapterIdentifier == null,
            );
          }
        });
      }
    }
  }

  Future<void> _loadReadalongManifest() async {
    final initialAudio = ref.read(audioPlayerProvider);
    if (initialAudio.book?.id == widget.book.id && initialAudio.status != AudioPlaybackStatus.idle) {
      final notifier = ref.read(audioPlayerProvider.notifier);
      notifier.setPlaybackMode(AudioPlaybackMode.readalong);
      notifier.syncBookMetadata(widget.book);
    }

    if (_readalongManifest != null) {
      if (mounted) {
        final audioState = ref.read(audioPlayerProvider);
        if (audioState.book?.id == widget.book.id && audioState.status != AudioPlaybackStatus.idle) {
          _onAudioStateChanged(
            audioState,
            forceSyncSpread: widget.initialChapterIdentifier == null,
          );
        }
      }
      return;
    }
    try {
      final manifest = await ref.read(bookRepositoryProvider).getBookReadalong(widget.book.id);
      if (mounted && (manifest.hasReadalong || manifest.segments.isNotEmpty)) {
        setState(() {
          _readalongManifest = manifest;
        });
        final audioState = ref.read(audioPlayerProvider);
        if (audioState.book?.id == widget.book.id && audioState.status != AudioPlaybackStatus.idle) {
          _onAudioStateChanged(
            audioState,
            forceSyncSpread: widget.initialChapterIdentifier == null,
          );
        }
      }
    } catch (_) {}
  }

  void _emitDeduplicatedHighlight({
    String? targetViewKey,
    required String bookId,
    String? chapterHref,
    required String fragmentId,
    required String activeClass,
    required String playbackActiveClass,
    required bool playing,
  }) {
    if (_lastHighlightedViewKey == targetViewKey &&
        _lastHighlightedFragmentId == fragmentId &&
        _lastHighlightedPlaying == playing) {
      return;
    }
    _lastHighlightedViewKey = targetViewKey;
    _lastHighlightedFragmentId = fragmentId;
    _lastHighlightedPlaying = playing;
    highlightHtmlFrame(
      targetViewKey: targetViewKey,
      bookId: bookId,
      chapterHref: chapterHref,
      fragmentId: fragmentId,
      activeClass: activeClass,
      playbackActiveClass: playbackActiveClass,
      playing: playing,
    );
  }

  void _onAudioStateChanged(AudioPlayerState audioState, {bool forceSyncSpread = false}) {
    if (audioState.book?.id != widget.book.id) {
      if (_activeSegment != null) {
        _activeSegment = null;
        _emitDeduplicatedHighlight(
          bookId: widget.book.id,
          fragmentId: '',
          activeClass: _readalongManifest?.activeClass ?? '-epub-media-overlay-active',
          playbackActiveClass: _readalongManifest?.playbackActiveClass ?? '-epub-media-overlay-playing',
          playing: false,
        );
      }
      return;
    }
    final manifest = _readalongManifest;
    if (manifest == null || manifest.segments.isEmpty) return;

    final posSec = audioState.position.inMilliseconds / 1000.0;
    final seg = manifest.segmentAt(posSec);
    _activeSegment = seg;

    final targetPage = _findPageForSegment(seg);
    final targetViewKey = targetPage != null ? _pageViewKey(targetPage) : null;
    final effectiveChapterHref = (targetPage?.href?.isNotEmpty ?? false)
        ? targetPage!.href!
        : (seg?.chapterHref ?? '');

    if (((_autoTurnPages && audioState.isPlaying) || forceSyncSpread) &&
        seg != null &&
        seg.chapterIndex > 0) {
      final spreads = _currentSpreads;
      if (spreads.isNotEmpty && _currentSpreadIndex < spreads.length) {
        final curSpread = spreads[_currentSpreadIndex];
        final onCurrentSpread = targetPage != null && targetPage.id.isNotEmpty
            ? curSpread.containsChapter(targetPage.id)
            : curSpread.containsChapterIndex(seg.chapterIndex, widget.book.spine);
        if (!onCurrentSpread) {
          final targetIdx = spreads.indexWhere(
            (s) => targetPage != null && targetPage.id.isNotEmpty
                ? s.containsChapter(targetPage.id)
                : s.containsChapterIndex(seg.chapterIndex, widget.book.spine),
          );
          if (targetIdx >= 0 && targetIdx != _currentSpreadIndex) {
            _goToSpread(targetIdx, spreads.length);
          }
        }
      }
    }

    final resolvedFragment =
        seg != null && (targetPage != null || widget.book.spine.isEmpty)
            ? seg.fragmentId
            : '';
    _emitDeduplicatedHighlight(
      targetViewKey: targetViewKey,
      bookId: widget.book.id,
      chapterHref: effectiveChapterHref,
      fragmentId: resolvedFragment,
      activeClass: manifest.activeClass,
      playbackActiveClass: manifest.playbackActiveClass,
      playing: audioState.isPlaying,
    );
  }

  Future<void> _handleReadalongClick(String fragmentId) async {
    final manifest = _readalongManifest;
    if (manifest == null || manifest.segments.isEmpty || fragmentId.isEmpty) {
      _toggleControls();
      return;
    }
    ReadalongSegment? matched;
    if (_currentSpreads.isNotEmpty && _currentSpreadIndex < _currentSpreads.length) {
      final curSpread = _currentSpreads[_currentSpreadIndex];
      for (final seg in manifest.segments) {
        if (seg.fragmentId == fragmentId &&
            curSpread.containsChapterIndex(seg.chapterIndex, widget.book.spine)) {
          matched = seg;
          break;
        }
      }
    }
    matched ??= manifest.segments
        .where((seg) => seg.fragmentId == fragmentId)
        .firstOrNull;
    if (matched == null) {
      _toggleControls();
      return;
    }

    final audioState = ref.read(audioPlayerProvider);
    final notifier = ref.read(audioPlayerProvider.notifier);
    notifier.setPlaybackMode(AudioPlaybackMode.readalong);
    notifier.syncBookMetadata(widget.book);
    if (audioState.book?.id != widget.book.id ||
        audioState.status == AudioPlaybackStatus.idle ||
        audioState.status == AudioPlaybackStatus.error) {
      await notifier.loadBook(
        widget.book,
        initialPosition: matched.globalBegin,
        autoPlay: true,
      );
    } else {
      await notifier.seek(
        Duration(milliseconds: (matched.globalBegin * 1000).round()),
      );
      if (!audioState.isPlaying) {
        await notifier.play();
      }
    }
  }

  void _switchToAudiobook(BuildContext context) {
    ref.read(audioPlayerProvider.notifier).setPlaybackMode(AudioPlaybackMode.audiobook);
    final target = '/books/${widget.book.id}/listen';
    if (context.canPop()) {
      context.pushReplacement(target);
    } else {
      context.go(target);
    }
  }

  Future<void> _toggleReadalongPlayback() async {
    final audioState = ref.read(audioPlayerProvider);
    final notifier = ref.read(audioPlayerProvider.notifier);
    notifier.setPlaybackMode(AudioPlaybackMode.readalong);
    notifier.syncBookMetadata(widget.book);
    if (audioState.book?.id != widget.book.id ||
        audioState.status == AudioPlaybackStatus.idle ||
        audioState.status == AudioPlaybackStatus.error) {
      double? initialPos;
      if (_currentSpreads.isNotEmpty && _currentSpreadIndex < _currentSpreads.length) {
        final curSpread = _currentSpreads[_currentSpreadIndex];
        final firstPage = curSpread.leftPage ?? curSpread.rightPage;
        final firstChIdx = firstPage.chapterIndex > 0 ? firstPage.chapterIndex : (_currentSpreadIndex + 1);
        var firstSeg = _readalongManifest?.firstSegmentForChapter(firstChIdx);
        if (firstSeg == null && curSpread.leftPage != null) {
          final rightChIdx = curSpread.rightPage.chapterIndex;
          if (rightChIdx > 0) {
            firstSeg = _readalongManifest?.firstSegmentForChapter(rightChIdx);
          }
        }
        if (firstSeg != null && firstSeg.globalBegin > 0) {
          initialPos = firstSeg.globalBegin;
        }
      }
      await notifier.loadBook(widget.book, initialPosition: initialPos, autoPlay: true);
    } else {
      await notifier.togglePlayPause();
    }
  }

  void _toggleControls() {
    setState(() {
      _controlsVisible = !_controlsVisible;
    });
  }

  @override
  void dispose() {
    _mediaTokenRefreshTimer?.cancel();
    _focusNode.dispose();
    super.dispose();
  }

  void _resolveInitialSpread() {
    final spine = widget.book.spine;
    if (spine.isEmpty) return;

    final spreads = _getSpreads(allowTwoPage: true);
    if (widget.initialChapterIdentifier != null) {
      final matchIdx = spreads.indexWhere((s) => s.containsChapter(widget.initialChapterIdentifier));
      if (matchIdx >= 0) {
        _currentSpreadIndex = matchIdx;
        return;
      }
    }
    _currentSpreadIndex = 0;
  }

  List<FixedSpread> _getSpreads({required bool allowTwoPage}) {
    final spine = widget.book.spine;
    if (spine.isEmpty) return [];

    if (!allowTwoPage) {
      return List.generate(
        spine.length,
        (i) => FixedSpread(
          spreadIndex: i,
          rightPage: spine[i],
          isSingle: true,
          isGatefold: spine[i].isGatefold,
        ),
      );
    }

    final spreads = <FixedSpread>[];
    // Cover is always standalone recto (Page 1 / Index 0)
    spreads.add(FixedSpread(
      spreadIndex: 0,
      rightPage: spine[0],
      isSingle: true,
      isGatefold: spine[0].isGatefold,
    ));

    int i = 1;
    while (i < spine.length) {
      final current = spine[i];
      if (current.isGatefold) {
        spreads.add(FixedSpread(
          spreadIndex: spreads.length,
          rightPage: current,
          isSingle: true,
          isGatefold: true,
        ));
        i++;
        continue;
      }

      if (i + 1 < spine.length) {
        final next = spine[i + 1];
        if (next.isGatefold) {
          spreads.add(FixedSpread(
            spreadIndex: spreads.length,
            rightPage: current,
            isSingle: true,
          ));
          i++;
        } else {
          spreads.add(FixedSpread(
            spreadIndex: spreads.length,
            leftPage: current,
            rightPage: next,
            isSingle: false,
          ));
          i += 2;
        }
      } else {
        spreads.add(FixedSpread(
          spreadIndex: spreads.length,
          rightPage: current,
          isSingle: true,
        ));
        i++;
      }
    }

    return spreads;
  }

  void _goToSpread(int index, int totalSpreads) {
    if (index >= 0 && index < totalSpreads) {
      setState(() {
        _currentSpreadIndex = index;
      });
      _saveProgress(index, totalSpreads);
    }
  }

  void _saveProgress(int index, int totalSpreads) {
    if (totalSpreads <= 0) return;
    final progress = (index + 1) / totalSpreads;
    ref.read(readerRepositoryProvider).updateProgress(widget.book.id, progress);
  }

  String _buildPageUrl(SpineItem page) {
    final apiService = ref.read(apiServiceProvider);
    final baseUrl = apiService.baseUrl;
    final cleanBase = (baseUrl.isNotEmpty && baseUrl.endsWith('/'))
        ? baseUrl.substring(0, baseUrl.length - 1)
        : baseUrl;
    final identifier = page.id.isNotEmpty ? page.id : page.chapterIndex;
    if (cleanBase.isNotEmpty) {
      return '$cleanBase/api/v1/books/${widget.book.id}/chapters/$identifier/html?fit=scale';
    }
    return '/api/v1/books/${widget.book.id}/chapters/$identifier/html?fit=scale';
  }

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;

    final spreads = _currentSpreads;
    if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
      if (_currentSpreadIndex > 0) {
        _goToSpread(_currentSpreadIndex - 1, spreads.length);
        return KeyEventResult.handled;
      }
    } else if (event.logicalKey == LogicalKeyboardKey.arrowRight ||
        event.logicalKey == LogicalKeyboardKey.space) {
      if (_currentSpreadIndex < spreads.length - 1) {
        _goToSpread(_currentSpreadIndex + 1, spreads.length);
        return KeyEventResult.handled;
      }
    }
    return KeyEventResult.ignored;
  }

  List<FixedSpread> _currentSpreads = const [];

  @override
  Widget build(BuildContext context) {
    ref.listen<AudioPlayerState>(audioPlayerProvider, (prev, next) {
      _onAudioStateChanged(next);
    });
    final audioState = ref.watch(audioPlayerProvider);
    final isThisBookPlaying =
        audioState.book?.id == widget.book.id && audioState.isPlaying;

    return LayoutBuilder(
      builder: (context, constraints) {
        final screenWidth = constraints.maxWidth;
        final screenHeight = constraints.maxHeight;

        final isWide = screenWidth >= 900 && screenWidth > screenHeight;
        final bool allowTwoPage;
        switch (_spreadMode) {
          case SpreadMode.auto:
            allowTwoPage = isWide;
            break;
          case SpreadMode.twoPage:
            allowTwoPage = true;
            break;
          case SpreadMode.singlePage:
            allowTwoPage = false;
            break;
        }

        final spreads = _getSpreads(allowTwoPage: allowTwoPage);
        _currentSpreads = spreads;

        if (_currentSpreadIndex >= spreads.length) {
          _currentSpreadIndex = max(0, spreads.length - 1);
        }

        final currentSpread = spreads.isNotEmpty ? spreads[_currentSpreadIndex] : null;
        final canPrev = _currentSpreadIndex > 0;
        final canNext = _currentSpreadIndex < spreads.length - 1;

        final colors = ShelfdColors.of(context);

        return Focus(
          focusNode: _focusNode,
          autofocus: true,
          onKeyEvent: _handleKeyEvent,
          child: Scaffold(
            backgroundColor: colors.background,
            body: SafeArea(
              child: Stack(
                children: [
                  // Stage / Reading Viewport
                  Positioned.fill(
                    child: currentSpread == null
                        ? Center(
                            child: Text(
                              'No pages found',
                              style: AppTypography.bodySans(color: colors.mutedInk),
                            ),
                          )
                        : GestureDetector(
                            behavior: HitTestBehavior.translucent,
                            onTap: _toggleControls,
                            child: _buildSpreadStage(
                              context,
                              currentSpread,
                              screenWidth,
                              screenHeight - (_controlsVisible ? 120 : 0),
                            ),
                          ),
                  ),

                  // Left & Right Edge Chevrons
                  if (canPrev)
                    Positioned(
                      left: 16,
                      top: 0,
                      bottom: 0,
                      child: Center(
                        child: Material(
                          color: Colors.black54,
                          shape: const CircleBorder(),
                          clipBehavior: Clip.antiAlias,
                          child: IconButton(
                            iconSize: 28,
                            padding: const EdgeInsets.all(12),
                            icon: const Icon(Icons.chevron_left_rounded, color: Colors.white),
                            tooltip: 'Previous Page',
                            onPressed: () => _goToSpread(_currentSpreadIndex - 1, spreads.length),
                          ),
                        ),
                      ),
                    ),
                  if (canNext)
                    Positioned(
                      right: 16,
                      top: 0,
                      bottom: 0,
                      child: Center(
                        child: Material(
                          color: Colors.black54,
                          shape: const CircleBorder(),
                          clipBehavior: Clip.antiAlias,
                          child: IconButton(
                            iconSize: 28,
                            padding: const EdgeInsets.all(12),
                            icon: const Icon(Icons.chevron_right_rounded, color: Colors.white),
                            tooltip: 'Next Page',
                            onPressed: () => _goToSpread(_currentSpreadIndex + 1, spreads.length),
                          ),
                        ),
                      ),
                    ),

                  // Top Header Bar
                  AnimatedPositioned(
                    duration: const Duration(milliseconds: 200),
                    top: _controlsVisible ? 0 : -70,
                    left: 0,
                    right: 0,
                    child: _buildTopBar(context, currentSpread, spreads.length, isThisBookPlaying),
                  ),

                  // Bottom Navigation Bar
                  AnimatedPositioned(
                    duration: const Duration(milliseconds: 200),
                    bottom: _controlsVisible ? 0 : -80,
                    left: 0,
                    right: 0,
                    child: _buildBottomBar(context, currentSpread, spreads.length),
                  ),
                ],
              ),
            ),
            endDrawer: _buildTocDrawer(context, spreads),
          ),
        );
      },
    );
  }

  Widget _buildTopBar(
    BuildContext context,
    FixedSpread? spread,
    int totalSpreads,
    bool isThisBookPlaying,
  ) {
    final colors = ShelfdColors.of(context);
    final pipState = ref.watch(readalongPipProvider);
    final isPipOpen = pipState.isOpen && pipState.bookId == widget.book.id;

    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: colors.surface.withValues(alpha: 0.95),
        border: Border(bottom: BorderSide(color: colors.subtleBorder)),
      ),
      child: Row(
        children: [
          IconButton(
            icon: Icon(Icons.arrow_back_rounded, color: colors.ink),
            tooltip: 'Back to Library',
            onPressed: () {
              if (widget.onClose != null) {
                widget.onClose!();
              } else if (context.canPop()) {
                context.pop();
              } else {
                context.go('/books/${widget.book.id}');
              }
            },
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.book.title,
                  style: AppTypography.titleSerif(fontSize: 15, color: colors.ink),
                  overflow: TextOverflow.ellipsis,
                ),
                if (spread != null)
                  Text(
                    '${spread.label} of ${widget.book.spine.length}',
                    style: AppTypography.mono(fontSize: 12, color: colors.secondaryInk),
                  ),
              ],
            ),
          ),

          if (_hasReadalong) ...[
            IconButton(
              key: const Key('readalong-play-pause-button'),
              icon: Icon(
                isThisBookPlaying
                    ? Icons.pause_circle_filled_rounded
                    : Icons.play_circle_fill_rounded,
                color: colors.accentPrimary,
              ),
              tooltip: isThisBookPlaying ? 'Pause Read-Along' : 'Play Read-Along',
              onPressed: _toggleReadalongPlayback,
            ),
            IconButton(
              key: const Key('readalong-autoturn-toggle'),
              icon: Icon(
                _autoTurnPages ? Icons.sync_rounded : Icons.sync_disabled_rounded,
                color: _autoTurnPages ? colors.accentPrimary : colors.mutedInk,
              ),
              tooltip: _autoTurnPages ? 'Auto-Turn Pages: On' : 'Auto-Turn Pages: Off',
              onPressed: () {
                setState(() {
                  _autoTurnPages = !_autoTurnPages;
                });
              },
            ),
            if (pipState.isSupported)
              IconButton(
                key: const Key('readalong-pip-button'),
                icon: Icon(
                  isPipOpen
                      ? Icons.picture_in_picture_rounded
                      : Icons.picture_in_picture_alt_rounded,
                  color: isPipOpen ? colors.accentPrimary : colors.ink,
                ),
                tooltip: isPipOpen ? 'Close Pop-out Read-Along' : 'Pop out Read-Along',
                onPressed: () {
                  final manifest = _readalongManifest ??
                      ReadalongManifest(bookId: widget.book.id, hasReadalong: true);
                  final currentPage = spread?.leftPage ?? spread?.rightPage;
                  ref.read(readalongPipProvider.notifier).togglePopout(
                        book: widget.book,
                        manifest: manifest,
                        currentPage: currentPage,
                        currentChapterIndex: currentPage?.chapterIndex,
                        currentChapterTitle: spread?.label,
                      );
                },
              ),
            IconButton(
              key: const Key('readalong-cast-button'),
              icon: Icon(
                ref.watch(audioPlayerProvider).isCasting
                    ? Icons.cast_connected
                    : Icons.cast,
                color: ref.watch(audioPlayerProvider).isCasting
                    ? colors.accentPrimary
                    : colors.ink,
              ),
              tooltip: 'Cast audio',
              onPressed: () => showCastDeviceSheet(
                context,
                book: widget.book,
                hasReadalong: true,
              ),
            ),
            IconButton(
              key: const Key('switch_to_audiobook_button'),
              icon: Icon(Icons.headphones_rounded, color: colors.ink),
              tooltip: 'Switch to Audiobook Player',
              onPressed: () => _switchToAudiobook(context),
            ),
          ],

          // Spread Mode Toggle
          IconButton(
            icon: Icon(
              _spreadMode == SpreadMode.auto
                  ? Icons.auto_stories_rounded
                  : _spreadMode == SpreadMode.twoPage
                      ? Icons.menu_book_rounded
                      : Icons.crop_portrait_rounded,
              color: colors.ink,
            ),
            tooltip: _spreadMode == SpreadMode.auto
                ? 'Spread Mode: Auto (Responsive)'
                : _spreadMode == SpreadMode.twoPage
                    ? 'Spread Mode: Two Pages (Facing)'
                    : 'Spread Mode: Single Page',
            onPressed: () {
              setState(() {
                if (_spreadMode == SpreadMode.auto) {
                  _spreadMode = SpreadMode.twoPage;
                } else if (_spreadMode == SpreadMode.twoPage) {
                  _spreadMode = SpreadMode.singlePage;
                } else {
                  _spreadMode = SpreadMode.auto;
                }
              });
            },
          ),

          // Bookmark Button
          IconButton(
            icon: Icon(Icons.bookmark_add_outlined, color: colors.ink),
            tooltip: 'Bookmark Page',
            onPressed: () {
              if (spread == null) return;
              final title = spread.label;
              final progress = (spread.spreadIndex + 1) / totalSpreads;
              ref.read(bookDetailProvider(widget.book.id).notifier).addBookmark(
                    title: title,
                    progress: progress,
                    chapterId: spread.rightPage.id,
                  );
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text('Bookmarked "$title"'),
                  duration: const Duration(seconds: 2),
                  behavior: SnackBarBehavior.floating,
                ),
              );
            },
          ),

          // Table of Contents
          Builder(
            builder: (ctx) => IconButton(
              icon: Icon(Icons.list_rounded, color: colors.ink),
              tooltip: 'Table of Contents',
              onPressed: () => Scaffold.of(ctx).openEndDrawer(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBottomBar(BuildContext context, FixedSpread? spread, int totalSpreads) {
    final colors = ShelfdColors.of(context);
    final canPrev = _currentSpreadIndex > 0;
    final canNext = _currentSpreadIndex < totalSpreads - 1;

    return Container(
      height: 64,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: colors.surface.withValues(alpha: 0.95),
        border: Border(top: BorderSide(color: colors.subtleBorder)),
      ),
      child: Row(
        children: [
          IconButton(
            icon: Icon(Icons.chevron_left_rounded, color: colors.ink),
            tooltip: 'Previous',
            onPressed: canPrev ? () => _goToSpread(_currentSpreadIndex - 1, totalSpreads) : null,
          ),
          Expanded(
            child: SliderTheme(
              data: SliderTheme.of(context).copyWith(
                activeTrackColor: colors.accentPrimary,
                inactiveTrackColor: colors.border,
                thumbColor: colors.accentPrimary,
                trackHeight: 3,
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 6),
              ),
              child: Slider(
                value: _currentSpreadIndex.toDouble(),
                min: 0,
                max: max(0, totalSpreads - 1).toDouble(),
                divisions: totalSpreads > 1 ? totalSpreads - 1 : 1,
                onChanged: (val) => _goToSpread(val.round(), totalSpreads),
              ),
            ),
          ),
          IconButton(
            icon: Icon(Icons.chevron_right_rounded, color: colors.ink),
            tooltip: 'Next',
            onPressed: canNext ? () => _goToSpread(_currentSpreadIndex + 1, totalSpreads) : null,
          ),
          const SizedBox(width: 8),
          Text(
            spread != null ? spread.label : '',
            style: AppTypography.mono(fontSize: 12, color: colors.secondaryInk),
          ),
        ],
      ),
    );
  }

  Widget _buildSpreadStage(
    BuildContext context,
    FixedSpread spread,
    double availWidth,
    double availHeight,
  ) {
    if (!_cookieReady) {
      final colors = ShelfdColors.of(context);
      if (_cookieError != null) {
        return Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  _cookieError!,
                  textAlign: TextAlign.center,
                  style: AppTypography.bodySans(color: colors.ink),
                ),
                const SizedBox(height: 12),
                ElevatedButton(
                  onPressed: _prepareNativeMediaCookie,
                  child: const Text('Retry'),
                ),
              ],
            ),
          ),
        );
      }
      return Center(
        child: CircularProgressIndicator(color: colors.accentPrimary),
      );
    }

    final horizMargin = availWidth < 600 ? 16.0 : 80.0;
    final vertMargin = availHeight < 600 ? 16.0 : 40.0;
    final stageW = max(40.0, availWidth - horizMargin);
    final stageH = max(40.0, availHeight - vertMargin);

    if (spread.isSingle || spread.leftPage == null) {
      final page = spread.rightPage;
      final pw = page.pageWidth ?? 1000.0;
      final ph = page.pageHeight ?? 1000.0;

      final scale = min(stageW / pw, stageH / ph).clamp(0.05, 2.0);

      final displayW = pw * scale;
      final displayH = ph * scale;
      final viewKey = _pageViewKey(page);
      final url = _buildPageUrl(page);

      return Center(
        child: Container(
          width: displayW,
          height: displayH,
          decoration: BoxDecoration(
            color: Colors.white,
            boxShadow: const [
              BoxShadow(
                color: Colors.black87,
                blurRadius: 24,
                spreadRadius: 4,
                offset: Offset(0, 8),
              ),
            ],
            borderRadius: BorderRadius.circular(4),
          ),
          clipBehavior: Clip.antiAlias,
          child: buildHtmlFrame(
            viewKey: viewKey,
            url: url,
            width: displayW,
            height: displayH,
            onReadalongClick: _handleReadalongClick,
            onToggleControls: _toggleControls,
          ),
        ),
      );
    } else {
      final leftPage = spread.leftPage!;
      final rightPage = spread.rightPage;

      final leftW = leftPage.pageWidth ?? 1000.0;
      final leftH = leftPage.pageHeight ?? 1000.0;
      final rightW = rightPage.pageWidth ?? 1000.0;
      final rightH = rightPage.pageHeight ?? 1000.0;

      final combinedW = leftW + rightW;
      final combinedH = max(leftH, rightH);

      final scale = min(stageW / combinedW, stageH / combinedH).clamp(0.05, 2.0);

      final leftDisplayW = leftW * scale;
      final rightDisplayW = rightW * scale;
      final displayH = combinedH * scale;

      final leftViewKey = _pageViewKey(leftPage);
      final rightViewKey = _pageViewKey(rightPage);
      final leftUrl = _buildPageUrl(leftPage);
      final rightUrl = _buildPageUrl(rightPage);

      return Center(
        child: Container(
          decoration: BoxDecoration(
            boxShadow: const [
              BoxShadow(
                color: Colors.black87,
                blurRadius: 28,
                spreadRadius: 6,
                offset: Offset(0, 10),
              ),
            ],
            borderRadius: BorderRadius.circular(4),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: leftDisplayW,
                height: displayH,
                decoration: const BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.horizontal(left: Radius.circular(4)),
                ),
                clipBehavior: Clip.antiAlias,
                child: buildHtmlFrame(
                  viewKey: leftViewKey,
                  url: leftUrl,
                  width: leftDisplayW,
                  height: displayH,
                  onReadalongClick: _handleReadalongClick,
                  onToggleControls: _toggleControls,
                ),
              ),
              Container(
                width: 2,
                height: displayH,
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    colors: [
                      Colors.black54,
                      Colors.black12,
                      Colors.black54,
                    ],
                  ),
                ),
              ),
              Container(
                width: rightDisplayW,
                height: displayH,
                decoration: const BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.horizontal(right: Radius.circular(4)),
                ),
                clipBehavior: Clip.antiAlias,
                child: buildHtmlFrame(
                  viewKey: rightViewKey,
                  url: rightUrl,
                  width: rightDisplayW,
                  height: displayH,
                  onReadalongClick: _handleReadalongClick,
                  onToggleControls: _toggleControls,
                ),
              ),
            ],
          ),
        ),
      );
    }
  }

  Widget _buildTocDrawer(BuildContext context, List<FixedSpread> spreads) {
    final colors = ShelfdColors.of(context);
    final spine = widget.book.spine;
    final namedSpine = widget.book.namedSpine;
    final displayItems = namedSpine.isNotEmpty ? namedSpine : spine;

    return Drawer(
      backgroundColor: colors.surface,
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.all(AppTokens.space20),
              child: Text(
                namedSpine.isNotEmpty ? 'Table of Contents' : 'Pages',
                style: AppTypography.titleSerif(fontSize: 20, color: colors.ink),
              ),
            ),
            Divider(color: colors.subtleBorder, height: 1),
            Expanded(
              child: ListView.builder(
                itemCount: displayItems.length,
                itemBuilder: (context, idx) {
                  final item = displayItems[idx];
                  final matchingSpreadIdx = spreads.indexWhere((s) => s.containsChapter(item.id));
                  final isCurrent = matchingSpreadIdx == _currentSpreadIndex;

                  return ListTile(
                    selected: isCurrent,
                    selectedTileColor: colors.container,
                    title: Text(
                      item.title.isNotEmpty ? item.title : 'Page ${item.chapterIndex}',
                      style: isCurrent
                          ? AppTypography.titleSerif(fontSize: 14, color: colors.accentPrimary)
                          : AppTypography.bodySans(fontSize: 14, color: colors.ink),
                    ),
                    subtitle: Text(
                      namedSpine.isNotEmpty
                          ? 'Page ${item.chapterIndex}'
                          : (item.isGatefold ? 'Gatefold Spread' : (item.pageSpread ?? 'Standard Page')),
                      style: AppTypography.mono(fontSize: 11, color: colors.mutedInk),
                    ),
                    trailing: isCurrent
                        ? Icon(Icons.check_rounded, size: 18, color: colors.accentPrimary)
                        : null,
                    onTap: () {
                      Navigator.of(context).pop();
                      if (matchingSpreadIdx >= 0) {
                        _goToSpread(matchingSpreadIdx, spreads.length);
                      }
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
