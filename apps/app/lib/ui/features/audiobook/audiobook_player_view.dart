import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'widgets/chapter_drawer.dart';
import 'widgets/scrubber_bar.dart';

class AudiobookPlayerView extends ConsumerStatefulWidget {
  final String bookId;

  const AudiobookPlayerView({super.key, required this.bookId});

  @override
  ConsumerState<AudiobookPlayerView> createState() => _AudiobookPlayerViewState();
}

class _AudiobookPlayerViewState extends ConsumerState<AudiobookPlayerView> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final playerNotifier = ref.read(audioPlayerProvider.notifier);
      final detailState = ref.read(bookDetailProvider(widget.bookId));
      if (detailState.book != null) {
        playerNotifier.loadBook(detailState.book!);
      } else {
        ref.read(bookDetailProvider(widget.bookId).notifier).loadBook().then((_) {
          final loadedBook = ref.read(bookDetailProvider(widget.bookId)).book;
          if (loadedBook != null) {
            playerNotifier.loadBook(loadedBook);
          }
        });
      }
    });
  }

  void _showSpeedPicker(BuildContext context, double currentSpeed) {
    const speeds = [0.75, 1.0, 1.25, 1.5, 1.75, 2.0];
    showModalBottomSheet(
      context: context,
      backgroundColor: AppTokens.obsidianBackground,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space24, vertical: AppTokens.space16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Playback Speed',
                style: AppTypography.titleSerif(fontSize: 18, color: Colors.white),
              ),
              const SizedBox(height: AppTokens.space16),
              Wrap(
                spacing: AppTokens.space12,
                runSpacing: AppTokens.space12,
                children: speeds.map((s) {
                  final isSelected = (currentSpeed - s).abs() < 0.01;
                  return ChoiceChip(
                    key: Key('speed_chip_${s}x'),
                    label: Text('${s.toStringAsFixed(s.truncateToDouble() == s ? 0 : 2)}x'),
                    selected: isSelected,
                    selectedColor: AppTokens.amberAccent,
                    backgroundColor: AppTokens.obsidianElevated,
                    labelStyle: TextStyle(
                      color: isSelected ? AppTokens.charcoalInk : Colors.white,
                      fontWeight: FontWeight.w600,
                    ),
                    onSelected: (_) {
                      ref.read(audioPlayerProvider.notifier).setSpeed(s);
                      Navigator.of(ctx).pop();
                    },
                  );
                }).toList(),
              ),
              const SizedBox(height: AppTokens.space16),
            ],
          ),
        ),
      ),
    );
  }

  void _showChapterDrawer(BuildContext context, Book book) {
    final playerState = ref.read(audioPlayerProvider);
    final playerNotifier = ref.read(audioPlayerProvider.notifier);

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (ctx) => ChapterDrawerSheet(
        book: book,
        chapters: playerState.chapters,
        currentChapterIndex: playerState.currentChapterIndex,
        currentPosition: playerState.position,
        sleepTimerRemaining: playerState.sleepTimerRemaining,
        isPlaying: playerState.isPlaying,
        onChapterSelected: (idx) => playerNotifier.seekToChapter(idx),
        onSeekToPosition: (pos) => playerNotifier.seek(pos),
        onSetSleepTimer: (dur) => playerNotifier.setSleepTimerDuration(dur),
        onAddBookmark: () => _addAudioBookmark(book, playerState.position),
      ),
    );
  }

  void _addAudioBookmark(Book book, Duration position) {
    final progress = book.durationSeconds != null && book.durationSeconds! > 0
        ? position.inSeconds / book.durationSeconds!
        : 0.0;
    final mins = position.inMinutes;
    final secs = position.inSeconds.remainder(60);
    final title = 'Audio Mark ${mins}m ${secs}s';
    ref.read(bookDetailProvider(widget.bookId).notifier).addBookmark(
          title: title,
          progress: progress,
        );
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Added bookmark at ${mins}m ${secs}s'),
        backgroundColor: AppTokens.obsidianElevated,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final detailState = ref.watch(bookDetailProvider(widget.bookId));
    final playerState = ref.watch(audioPlayerProvider);
    final playerNotifier = ref.read(audioPlayerProvider.notifier);

    final book = playerState.book ?? detailState.book;

    if (book == null) {
      return const Scaffold(
        backgroundColor: AppTokens.obsidianBackground,
        body: Center(
          child: CircularProgressIndicator(color: AppTokens.amberAccent),
        ),
      );
    }

    final currentChapter = playerState.activeChapter;
    final chapterTitle = currentChapter?.title ?? 'Full Audiobook';

    return Scaffold(
      backgroundColor: AppTokens.obsidianBackground,
      appBar: AppBar(
        backgroundColor: AppTokens.obsidianBackground,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        leading: IconButton(
          key: const Key('audio_player_back_button'),
          icon: const Icon(Icons.keyboard_arrow_down, color: Colors.white, size: 28),
          tooltip: 'Minimize Player',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/books/${widget.bookId}');
            }
          },
        ),
        title: Column(
          children: [
            Text(
              'PLAYING AUDIOBOOK',
              style: AppTypography.labelCaps(
                fontSize: 10,
                color: AppTokens.antiqueCream.withValues(alpha: 0.6),
              ),
            ),
            Text(
              book.title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.titleSerif(
                fontSize: 15,
                color: Colors.white,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
        centerTitle: true,
        actions: [
          IconButton(
            key: const Key('audio_player_chapters_menu_button'),
            icon: const Icon(Icons.format_list_bulleted, color: Colors.white),
            tooltip: 'Chapters & Bookmarks',
            onPressed: () => _showChapterDrawer(context, book),
          ),
        ],
      ),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            final isCompact = constraints.maxHeight < 600;

            return SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space24),
              child: ConstrainedBox(
                constraints: BoxConstraints(minHeight: constraints.maxHeight),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    const SizedBox(height: AppTokens.space8),

                    // Hero Cover Artwork with ambient glow
                    Center(
                      child: Container(
                        width: isCompact ? 180 : 240,
                        height: isCompact ? 270 : 360,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
                          boxShadow: [
                            BoxShadow(
                              color: AppTokens.amberAccent.withValues(alpha: 0.15),
                              blurRadius: 36,
                              spreadRadius: 4,
                              offset: const Offset(0, 12),
                            ),
                          ],
                        ),
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
                          child: book.coverUrl != null
                              ? Image.network(
                                  book.coverUrl!,
                                  fit: BoxFit.cover,
                                  errorBuilder: (context, error, stackTrace) =>
                                      _buildCoverFallback(book.title),
                                )
                              : _buildCoverFallback(book.title),
                        ),
                      ),
                    ),

                    const SizedBox(height: AppTokens.space16),

                    // Titles & Chapter Name
                    Column(
                      children: [
                        Text(
                          chapterTitle,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.center,
                          style: AppTypography.titleSerif(
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                            color: Colors.white,
                          ),
                        ),
                        const SizedBox(height: AppTokens.space4),
                        Text(
                          book.authorDisplay,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.center,
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            color: AppTokens.antiqueCream.withValues(alpha: 0.8),
                          ),
                        ),
                      ],
                    ),

                    const SizedBox(height: AppTokens.space16),

                    // Tactile Scrubber Bar
                    ScrubberBar(
                      position: playerState.position,
                      duration: playerState.duration,
                      chapterPosition: playerState.chapterPosition,
                      chapterDuration: playerState.chapterDuration,
                      onSeek: (pos) => playerNotifier.seek(pos),
                      isInteractive: playerState.duration > Duration.zero,
                    ),

                    const SizedBox(height: AppTokens.space16),

                    // Primary Transport Controls (15s Skip, Play/Pause Orb, 30s Skip)
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        // Skip Backward 15s
                        IconButton(
                          key: const Key('audio_player_skip_back_button'),
                          iconSize: 36,
                          icon: const Icon(Icons.replay_10, color: Colors.white),
                          tooltip: 'Skip backward 15 seconds',
                          onPressed: () => playerNotifier.skipBackward(),
                        ),
                        const SizedBox(width: AppTokens.space24),

                        // Play/Pause circular Amber Orb (64px)
                        Container(
                          width: 68,
                          height: 68,
                          decoration: BoxDecoration(
                            color: AppTokens.amberAccent,
                            shape: BoxShape.circle,
                            boxShadow: [
                              BoxShadow(
                                color: AppTokens.amberAccent.withValues(alpha: 0.35),
                                blurRadius: 20,
                                spreadRadius: 2,
                                offset: const Offset(0, 4),
                              ),
                            ],
                          ),
                          child: playerState.isLoading
                              ? const Center(
                                  child: SizedBox(
                                    width: 28,
                                    height: 28,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 3,
                                      color: AppTokens.charcoalInk,
                                    ),
                                  ),
                                )
                              : IconButton(
                                  key: const Key('audio_player_play_pause_button'),
                                  iconSize: 36,
                                  color: AppTokens.charcoalInk,
                                  icon: Icon(
                                    playerState.isPlaying
                                        ? Icons.pause
                                        : Icons.play_arrow,
                                  ),
                                  onPressed: () => playerNotifier.togglePlayPause(),
                                ),
                        ),
                        const SizedBox(width: AppTokens.space24),

                        // Skip Forward 30s
                        IconButton(
                          key: const Key('audio_player_skip_forward_button'),
                          iconSize: 36,
                          icon: const Icon(Icons.forward_30, color: Colors.white),
                          tooltip: 'Skip forward 30 seconds',
                          onPressed: () => playerNotifier.skipForward(),
                        ),
                      ],
                    ),

                    const SizedBox(height: AppTokens.space20),

                    // Bottom Utility Controls (Speed, Sleep Timer, Chapters, Bookmark)
                    Padding(
                      padding: const EdgeInsets.only(bottom: AppTokens.space16),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                        children: [
                          // Playback Speed Button
                          TextButton.icon(
                            key: const Key('audio_player_speed_button'),
                            style: TextButton.styleFrom(
                              foregroundColor: AppTokens.antiqueCream,
                              backgroundColor: AppTokens.obsidianElevated,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                              ),
                            ),
                            icon: const Icon(Icons.speed, size: 16),
                            label: Text(
                              '${playerState.speed.toStringAsFixed(playerState.speed.truncateToDouble() == playerState.speed ? 0 : 2)}x',
                              style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                            ),
                            onPressed: () => _showSpeedPicker(context, playerState.speed),
                          ),

                          // Sleep Timer Trigger
                          TextButton.icon(
                            key: const Key('audio_player_sleep_timer_button'),
                            style: TextButton.styleFrom(
                              foregroundColor: playerState.sleepTimerRemaining != null
                                  ? AppTokens.amberAccent
                                  : AppTokens.antiqueCream,
                              backgroundColor: AppTokens.obsidianElevated,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                              ),
                            ),
                            icon: Icon(
                              playerState.sleepTimerRemaining != null
                                  ? Icons.bedtime
                                  : Icons.bedtime,
                              size: 16,
                            ),
                            label: Text(
                              playerState.sleepTimerRemaining != null
                                  ? '${playerState.sleepTimerRemaining!.inMinutes}m'
                                  : 'Sleep',
                              style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                            ),
                            onPressed: () => _showChapterDrawer(context, book),
                          ),

                          // Add Bookmark Button
                          IconButton(
                            key: const Key('audio_player_add_bookmark_button'),
                            icon: const Icon(Icons.bookmark_add_outlined, color: AppTokens.antiqueCream),
                            tooltip: 'Add Bookmark',
                            onPressed: () => _addAudioBookmark(book, playerState.position),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildCoverFallback(String title) {
    return Container(
      color: AppTokens.obsidianContainer,
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.headphones, size: 54, color: AppTokens.amberAccent),
            const SizedBox(height: AppTokens.space12),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16),
              child: Text(
                title,
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.titleSerif(fontSize: 14, color: Colors.white70),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
