import 'package:flutter/material.dart';
import '../../../../data/models/book.dart';
import '../../../core/tokens.dart';
import '../../../core/typography.dart';

/// Animated equalizer wave bars for the active audio chapter
class EqualizerBars extends StatefulWidget {
  final bool isPlaying;
  final Color color;

  const EqualizerBars({
    super.key,
    this.isPlaying = true,
    this.color = AppTokens.amberAccent,
  });

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
    if (widget.isPlaying) {
      _controller.repeat(reverse: true);
    }
  }

  @override
  void didUpdateWidget(covariant EqualizerBars oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.isPlaying && !_controller.isAnimating) {
      _controller.repeat(reverse: true);
    } else if (!widget.isPlaying && _controller.isAnimating) {
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
class ChapterDrawerSheet extends StatefulWidget {
  final Book book;
  final List<AudioChapter> chapters;
  final int currentChapterIndex;
  final Duration currentPosition;
  final Duration? sleepTimerRemaining;
  final bool isPlaying;
  final ValueChanged<int> onChapterSelected;
  final ValueChanged<Duration> onSeekToPosition;
  final ValueChanged<Duration?> onSetSleepTimer;
  final VoidCallback onAddBookmark;

  const ChapterDrawerSheet({
    super.key,
    required this.book,
    required this.chapters,
    required this.currentChapterIndex,
    required this.currentPosition,
    this.sleepTimerRemaining,
    required this.isPlaying,
    required this.onChapterSelected,
    required this.onSeekToPosition,
    required this.onSetSleepTimer,
    required this.onAddBookmark,
  });

  @override
  State<ChapterDrawerSheet> createState() => _ChapterDrawerSheetState();
}

class _ChapterDrawerSheetState extends State<ChapterDrawerSheet> {
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
                      child: widget.book.coverUrl != null
                          ? Image.network(
                              widget.book.coverUrl!,
                              fit: BoxFit.cover,
                              errorBuilder: (context, error, stackTrace) =>
                                  const Icon(Icons.headphones_rounded, color: AppTokens.amberAccent, size: 24),
                            )
                          : const Icon(Icons.headphones_rounded, color: AppTokens.amberAccent, size: 24),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          widget.book.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: 16,
                            color: Colors.white,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        Text(
                          widget.book.authorDisplay,
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
                    icon: const Icon(Icons.close_rounded, color: Colors.white70),
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
                  _tabChip(0, 'Chapters (${widget.chapters.length})', Icons.format_list_bulleted_rounded),
                  const SizedBox(width: AppTokens.space8),
                  _tabChip(1, 'Bookmarks (${widget.book.bookmarks.length})', Icons.bookmark_outline_rounded),
                  const SizedBox(width: AppTokens.space8),
                  _tabChip(
                    2,
                    widget.sleepTimerRemaining != null
                        ? '${_formatDuration(widget.sleepTimerRemaining!)}'
                        : 'Timer',
                    Icons.bedtime_outlined,
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
                child: _buildTabBody(context),
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

  Widget _buildTabBody(BuildContext context) {
    switch (_selectedTab) {
      case 0:
        return _buildChaptersList();
      case 1:
        return _buildBookmarksList();
      case 2:
      default:
        return _buildSleepTimerList();
    }
  }

  Widget _buildChaptersList() {
    if (widget.chapters.isEmpty) {
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
      itemCount: widget.chapters.length,
      separatorBuilder: (context, index) => const SizedBox(height: 6),
      itemBuilder: (context, index) {
        final chapter = widget.chapters[index];
        final isActive = index == widget.currentChapterIndex;
        final isCompleted = index < widget.currentChapterIndex;
        final duration = Duration(seconds: chapter.durationSec.round());
        final startOffset = Duration(seconds: chapter.startOffsetSec.round());

        return InkWell(
          key: Key('chapter_tile_$index'),
          onTap: () {
            widget.onChapterSelected(index);
            Navigator.of(context).pop();
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
                    child: EqualizerBars(isPlaying: widget.isPlaying),
                  )
                else if (isCompleted)
                  const Padding(
                    padding: EdgeInsets.only(right: 10),
                    child: Icon(Icons.check_circle_rounded, color: AppTokens.amberAccent, size: 18),
                  )
                else
                  Padding(
                    padding: const EdgeInsets.only(right: 10),
                    child: Text(
                      '${index + 1}',
                      style: AppTypography.bodySans(
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
                        style: AppTypography.captionSans(
                          fontSize: 11,
                          color: AppTokens.antiqueCream.withValues(alpha: 0.6),
                        ),
                      ),
                    ],
                  ),
                ),
                Text(
                  _formatDuration(duration),
                  style: AppTypography.captionSans(
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

  Widget _buildBookmarksList() {
    final bookmarks = widget.book.bookmarks;

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
                widget.onAddBookmark();
                Navigator.of(context).pop();
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
                final targetSeconds = (bm.progress * (widget.book.durationSeconds ?? 0)).round();
                final targetPos = Duration(seconds: targetSeconds);

                return Material(
                  color: Colors.transparent,
                  child: ListTile(
                    tileColor: AppTokens.obsidianContainer,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      side: const BorderSide(color: AppTokens.obsidianBorder),
                    ),
                    leading: const Icon(Icons.bookmark_rounded, color: AppTokens.amberAccent),
                    title: Text(
                      bm.title,
                      style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600),
                    ),
                    subtitle: Text(
                      'Position: ${_formatDuration(targetPos)}',
                      style: const TextStyle(color: Colors.white54, fontSize: 12),
                    ),
                    trailing: const Icon(Icons.play_circle_outline_rounded, color: AppTokens.amberAccent),
                    onTap: () {
                      widget.onSeekToPosition(targetPos);
                      Navigator.of(context).pop();
                    },
                  ),
                );
              },
            ),
          ),
      ],
    );
  }

  Widget _buildSleepTimerList() {
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
        final isSelected = (dur == null && widget.sleepTimerRemaining == null);

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
              dur == null ? Icons.timer_off_outlined : Icons.bedtime_outlined,
              color: isSelected ? AppTokens.amberAccent : Colors.white60,
            ),
            title: Text(
              label,
              style: TextStyle(
                color: isSelected ? AppTokens.amberGlow : Colors.white,
                fontSize: 14,
                fontWeight: isSelected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
            trailing: isSelected
                ? const Icon(Icons.check_rounded, color: AppTokens.amberAccent)
                : null,
            onTap: () {
              widget.onSetSleepTimer(dur);
              Navigator.of(context).pop();
            },
          ),
        );
      },
    );
  }
}
