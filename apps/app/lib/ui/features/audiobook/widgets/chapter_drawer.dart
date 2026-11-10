import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../data/models/book.dart';
import '../../../core/tokens.dart';
import '../../../core/typography.dart';
import '../../../state/providers.dart';

/// Animated equalizer wave bars for the active audio chapter
class EqualizerBars extends StatefulWidget {
  final bool isPlaying;
  final bool? isAnimating;
  final Color color;

  const EqualizerBars({
    super.key,
    this.isPlaying = true,
    this.isAnimating,
    this.color = AppTokens.amberAccent,
  });

  bool get effectiveAnimating => isAnimating ?? isPlaying;

  @override
  State<EqualizerBars> createState() => _EqualizerBarsState();
}

class _EqualizerBarsState extends State<EqualizerBars> with SingleTickerProviderStateMixin {
  late AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 800),
    );
    if (widget.effectiveAnimating) {
      _controller.repeat(reverse: true);
    }
  }

  @override
  void didUpdateWidget(covariant EqualizerBars oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.effectiveAnimating && !_controller.isAnimating) {
      _controller.repeat(reverse: true);
    } else if (!widget.effectiveAnimating && _controller.isAnimating) {
      _controller.stop();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        final v = _controller.value;
        return Row(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            _bar(8 + 8 * v),
            const SizedBox(width: 2),
            _bar(16 - 10 * v),
            const SizedBox(width: 2),
            _bar(10 + 6 * (1.0 - v)),
          ],
        );
      },
    );
  }

  Widget _bar(double height) {
    return Container(
      width: 3,
      height: height.clamp(4.0, 16.0),
      decoration: BoxDecoration(
        color: widget.color,
        borderRadius: BorderRadius.circular(1.5),
      ),
    );
  }
}

/// Chapters & Bookmarks Modal Bottom Sheet
class ChapterDrawerSheet extends ConsumerStatefulWidget {
  final String? bookId;
  final Book? book;
  final List<AudioChapter>? chapters;
  final int? currentChapterIndex;
  final Duration? currentPosition;
  final Duration? sleepTimerRemaining;
  final bool? isPlaying;
  final ValueChanged<int>? onChapterSelected;
  final ValueChanged<Duration>? onSeekToPosition;
  final ValueChanged<Duration?>? onSetSleepTimer;
  final VoidCallback? onAddBookmark;

  const ChapterDrawerSheet({
    super.key,
    this.bookId,
    this.book,
    this.chapters,
    this.currentChapterIndex,
    this.currentPosition,
    this.sleepTimerRemaining,
    this.isPlaying,
    this.onChapterSelected,
    this.onSeekToPosition,
    this.onSetSleepTimer,
    this.onAddBookmark,
  });

  @override
  ConsumerState<ChapterDrawerSheet> createState() => _ChapterDrawerSheetState();
}

class _ChapterDrawerSheetState extends ConsumerState<ChapterDrawerSheet> {
  int _selectedTab = 0; // 0: Chapters, 1: Bookmarks, 2: Sleep Timer

  String _formatDuration(Duration d) {
    final hours = d.inHours;
    final minutes = d.inMinutes.remainder(60);
    final seconds = d.inSeconds.remainder(60);
    if (hours > 0) {
      return '${hours}h ${minutes}m';
    }
    if (minutes > 0) {
      return '${minutes}m ${seconds}s';
    }
    return '${seconds}s';
  }

  @override
  Widget build(BuildContext context) {
    final playerState = ref.watch(audioPlayerProvider);
    final effectiveBookId = widget.bookId ?? widget.book?.id ?? playerState.book?.id ?? '';
    final detailBook = (widget.book == null && effectiveBookId.isNotEmpty)
        ? ref.watch(bookDetailProvider(effectiveBookId)).book
        : null;

    final effectiveBook = widget.book ??
        detailBook ??
        playerState.book ??
        Book(
          id: effectiveBookId,
          title: 'Audiobook',
          authors: const [],
        );
    final effectiveChapters = widget.chapters ??
        (playerState.chapters.isNotEmpty ? playerState.chapters : effectiveBook.audioChapters);
    final effectiveChapterIndex = widget.currentChapterIndex ?? playerState.currentChapterIndex;
    final effectivePosition = widget.currentPosition ?? playerState.position;
    final effectiveSleepTimer = widget.sleepTimerRemaining ?? playerState.sleepTimerRemaining;
    final effectiveIsPlaying = widget.isPlaying ?? playerState.isPlaying;

    return Container(
      decoration: const BoxDecoration(
        color: AppTokens.obsidianBackground,
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
        border: Border(
          top: BorderSide(color: AppTokens.obsidianBorder, width: 1.5),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Drag handle
            Center(
              child: Container(
                margin: const EdgeInsets.only(top: 12, bottom: 8),
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: AppTokens.obsidianElevated,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),

            // Header with Cover & Title
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space20, vertical: AppTokens.space8),
              child: Row(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    child: Container(
                      width: 44,
                      height: 44,
                      color: AppTokens.obsidianSurface,
                      child: effectiveBook.coverUrl != null
                          ? Image.network(
                              effectiveBook.coverUrl!,
                              fit: BoxFit.cover,
                              errorBuilder: (context, error, stackTrace) =>
                                  const Icon(Icons.headphones, color: AppTokens.amberAccent, size: 24),
                            )
                          : const Icon(Icons.headphones, color: AppTokens.amberAccent, size: 24),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          effectiveBook.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: 16,
                            color: Colors.white,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        Text(
                          effectiveBook.authorDisplay,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            color: AppTokens.antiqueCream.withValues(alpha: 0.7),
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, color: Colors.white70),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),

            const Divider(color: AppTokens.obsidianBorder, height: 1),

            // Segmented pill tab buttons
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space12),
              child: Row(
                children: [
                  _tabChip(0, 'Chapters (${effectiveChapters.length})', Icons.format_list_bulleted),
                  const SizedBox(width: AppTokens.space8),
                  _tabChip(1, 'Bookmarks (${effectiveBook.bookmarks.length})', Icons.bookmark_outline),
                  const SizedBox(width: AppTokens.space8),
                  _tabChip(
                    2,
                    effectiveSleepTimer != null
                        ? _formatDuration(effectiveSleepTimer)
                        : 'Timer',
                    Icons.bedtime,
                  ),
                ],
              ),
            ),

            // Tab Content
            Flexible(
              child: ConstrainedBox(
                constraints: BoxConstraints(
                  maxHeight: MediaQuery.of(context).size.height * 0.55,
                ),
                child: _buildTabBody(
                  context,
                  effectiveBook: effectiveBook,
                  effectiveChapters: effectiveChapters,
                  effectiveChapterIndex: effectiveChapterIndex,
                  effectivePosition: effectivePosition,
                  effectiveSleepTimer: effectiveSleepTimer,
                  effectiveIsPlaying: effectiveIsPlaying,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _tabChip(int index, String label, IconData icon) {
    final isSelected = _selectedTab == index;
    return Expanded(
      child: Material(
        color: isSelected ? AppTokens.obsidianElevated : AppTokens.obsidianContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
        child: InkWell(
          key: Key('chapter_drawer_tab_$index'),
          borderRadius: BorderRadius.circular(AppTokens.radiusPill),
          onTap: () => setState(() => _selectedTab = index),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 8),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppTokens.radiusPill),
              border: Border.all(
                color: isSelected ? AppTokens.amberAccent.withValues(alpha: 0.6) : AppTokens.obsidianBorder,
              ),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  icon,
                  size: 15,
                  color: isSelected ? AppTokens.amberAccent : Colors.white60,
                ),
                const SizedBox(width: 6),
                Flexible(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                      color: isSelected ? AppTokens.amberAccent : Colors.white70,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTabBody(
    BuildContext context, {
    required Book effectiveBook,
    required List<AudioChapter> effectiveChapters,
    required int effectiveChapterIndex,
    required Duration effectivePosition,
    required Duration? effectiveSleepTimer,
    required bool effectiveIsPlaying,
  }) {
    switch (_selectedTab) {
      case 0:
        return _buildChaptersList(
          effectiveChapters: effectiveChapters,
          effectiveChapterIndex: effectiveChapterIndex,
          effectiveIsPlaying: effectiveIsPlaying,
        );
      case 1:
        return _buildBookmarksList(
          effectiveBook: effectiveBook,
          effectivePosition: effectivePosition,
        );
      case 2:
      default:
        return _buildSleepTimerList(effectiveSleepTimer: effectiveSleepTimer);
    }
  }

  Widget _buildChaptersList({
    required List<AudioChapter> effectiveChapters,
    required int effectiveChapterIndex,
    required bool effectiveIsPlaying,
  }) {
    if (effectiveChapters.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space32),
          child: Text(
            'No individual chapter markers available.\nAudio plays as full track.',
            textAlign: TextAlign.center,
            style: AppTypography.bodySans(color: Colors.white60, fontSize: 13),
          ),
        ),
      );
    }

    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space8),
      itemCount: effectiveChapters.length,
      separatorBuilder: (context, index) => const SizedBox(height: 6),
      itemBuilder: (context, index) {
        final chapter = effectiveChapters[index];
        final isActive = index == effectiveChapterIndex;
        final isCompleted = index < effectiveChapterIndex;
        final duration = Duration(seconds: chapter.durationSec.round());
        final startOffset = Duration(seconds: chapter.startOffsetSec.round());

        return InkWell(
          key: Key('chapter_tile_$index'),
          onTap: () {
            if (widget.onChapterSelected != null) {
              widget.onChapterSelected!(index);
            } else {
              ref.read(audioPlayerProvider.notifier).seekToChapter(index);
            }
            if (Navigator.of(context).canPop()) {
              Navigator.of(context).pop();
            }
          },
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12, vertical: AppTokens.space12),
            decoration: BoxDecoration(
              color: isActive ? AppTokens.obsidianElevated : AppTokens.obsidianContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: Border.all(
                color: isActive ? AppTokens.amberAccent : AppTokens.obsidianBorder,
                width: isActive ? 1.5 : 1.0,
              ),
            ),
            child: Row(
              children: [
                if (isActive)
                  Padding(
                    padding: const EdgeInsets.only(right: 10),
                    child: EqualizerBars(isPlaying: effectiveIsPlaying),
                  )
                else if (isCompleted)
                  const Padding(
                    padding: EdgeInsets.only(right: 10),
                    child: Icon(Icons.check_circle, color: AppTokens.amberAccent, size: 18),
                  )
                else
                  Padding(
                    padding: const EdgeInsets.only(right: 10),
                    child: Text(
                      '${index + 1}',
                      style: AppTypography.mono(
                        fontSize: 13,
                        color: Colors.white38,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        chapter.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.titleSerif(
                          fontSize: 14,
                          fontWeight: isActive ? FontWeight.w700 : FontWeight.w500,
                          color: isActive ? AppTokens.amberGlow : Colors.white,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'Start: ${_formatDuration(startOffset)}',
                        style: AppTypography.mono(
                          fontSize: 11,
                          color: AppTokens.antiqueCream.withValues(alpha: 0.6),
                        ),
                      ),
                    ],
                  ),
                ),
                Text(
                  _formatDuration(duration),
                  style: AppTypography.mono(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: isActive ? AppTokens.amberAccent : Colors.white54,
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildBookmarksList({
    required Book effectiveBook,
    required Duration effectivePosition,
  }) {
    final bookmarks = effectiveBook.bookmarks;

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space8),
          child: SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: AppTokens.amberAccent,
                side: const BorderSide(color: AppTokens.amberAccent),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusMd)),
              ),
              icon: const Icon(Icons.bookmark_add_outlined, size: 18),
              label: const Text('Add Audio Bookmark at Current Time'),
              onPressed: () {
                if (widget.onAddBookmark != null) {
                  widget.onAddBookmark!();
                } else if (effectiveBook.id.isNotEmpty) {
                  final progress = effectiveBook.durationSeconds != null && effectiveBook.durationSeconds! > 0
                      ? effectivePosition.inSeconds / effectiveBook.durationSeconds!
                      : 0.0;
                  final mins = effectivePosition.inMinutes;
                  final secs = effectivePosition.inSeconds.remainder(60);
                  ref.read(bookDetailProvider(effectiveBook.id).notifier).addBookmark(
                        title: 'Audio Mark ${mins}m ${secs}s',
                        progress: progress,
                      );
                }
                if (Navigator.of(context).canPop()) {
                  Navigator.of(context).pop();
                }
              },
            ),
          ),
        ),
        if (bookmarks.isEmpty)
          Expanded(
            child: Center(
              child: Text(
                'No audio bookmarks created yet.',
                style: AppTypography.bodySans(color: Colors.white60, fontSize: 13),
              ),
            ),
          )
        else
          Expanded(
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space4),
              itemCount: bookmarks.length,
              separatorBuilder: (context, index) => const SizedBox(height: 6),
              itemBuilder: (context, index) {
                final bm = bookmarks[index];
                final targetSeconds = (bm.progress * (effectiveBook.durationSeconds ?? 0)).round();
                final targetPos = Duration(seconds: targetSeconds);

                return Material(
                  color: Colors.transparent,
                  child: ListTile(
                    tileColor: AppTokens.obsidianContainer,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      side: const BorderSide(color: AppTokens.obsidianBorder),
                    ),
                    leading: const Icon(Icons.bookmark, color: AppTokens.amberAccent),
                    title: Text(
                      bm.title,
                      style: AppTypography.bodySans(
                        color: Colors.white,
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    subtitle: Text(
                      'Position: ${_formatDuration(targetPos)}',
                      style: AppTypography.mono(color: Colors.white54, fontSize: 12),
                    ),
                    trailing: const Icon(Icons.play_circle_outline, color: AppTokens.amberAccent),
                    onTap: () {
                      if (widget.onSeekToPosition != null) {
                        widget.onSeekToPosition!(targetPos);
                      } else {
                        ref.read(audioPlayerProvider.notifier).seek(targetPos);
                      }
                      if (Navigator.of(context).canPop()) {
                        Navigator.of(context).pop();
                      }
                    },
                  ),
                );
              },
            ),
          ),
      ],
    );
  }

  Widget _buildSleepTimerList({required Duration? effectiveSleepTimer}) {
    final timerOptions = [
      {'label': 'Off', 'duration': null},
      {'label': '15 minutes', 'duration': const Duration(minutes: 15)},
      {'label': '30 minutes', 'duration': const Duration(minutes: 30)},
      {'label': '45 minutes', 'duration': const Duration(minutes: 45)},
      {'label': '60 minutes', 'duration': const Duration(minutes: 60)},
      {'label': 'End of Chapter', 'duration': const Duration(minutes: 999)},
    ];

    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space8),
      itemCount: timerOptions.length,
      separatorBuilder: (context, index) => const SizedBox(height: 6),
      itemBuilder: (context, index) {
        final opt = timerOptions[index];
        final label = opt['label'] as String;
        final dur = opt['duration'] as Duration?;
        final isSelected = (dur == null && effectiveSleepTimer == null);

        return Material(
          color: Colors.transparent,
          child: ListTile(
            key: Key('sleep_timer_option_$index'),
            tileColor: isSelected ? AppTokens.obsidianElevated : AppTokens.obsidianContainer,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              side: BorderSide(
                color: isSelected ? AppTokens.amberAccent : AppTokens.obsidianBorder,
              ),
            ),
            leading: Icon(
              dur == null ? Icons.timer_off : Icons.bedtime,
              color: isSelected ? AppTokens.amberAccent : Colors.white60,
            ),
            title: Text(
              label,
              style: AppTypography.bodySans(
                color: isSelected ? AppTokens.amberGlow : Colors.white,
                fontSize: 14,
                fontWeight: isSelected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
            trailing: isSelected
                ? const Icon(Icons.check, color: AppTokens.amberAccent)
                : null,
            onTap: () {
              if (widget.onSetSleepTimer != null) {
                widget.onSetSleepTimer!(dur);
              } else {
                ref.read(audioPlayerProvider.notifier).setSleepTimerDuration(dur);
              }
              if (Navigator.of(context).canPop()) {
                Navigator.of(context).pop();
              }
            },
          ),
        );
      },
    );
  }
}
