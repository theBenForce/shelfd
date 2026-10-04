import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/tokens.dart';
import '../../../core/typography.dart';
import '../../../state/audio_player_state.dart';
import '../../../state/providers.dart';

/// Floating mini-player bar docked directly above the navigation bar
class MiniPlayerBar extends ConsumerWidget {
  const MiniPlayerBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final playerState = ref.watch(audioPlayerProvider);
    final playerNotifier = ref.read(audioPlayerProvider.notifier);

    if (playerState.status == AudioPlaybackStatus.idle || playerState.book == null) {
      return const SizedBox.shrink();
    }

    final book = playerState.book!;
    final progress = playerState.duration.inMilliseconds > 0
        ? (playerState.position.inMilliseconds / playerState.duration.inMilliseconds).clamp(0.0, 1.0)
        : 0.0;
    final currentChapter = playerState.activeChapter;

    return Container(
      decoration: BoxDecoration(
        color: AppTokens.obsidianBackground,
        border: const Border(
          top: BorderSide(color: AppTokens.obsidianBorder, width: 1),
          bottom: BorderSide(color: AppTokens.obsidianBorder, width: 1),
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.3),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Thin linear progress bar at top edge
          LinearProgressIndicator(
            value: progress,
            backgroundColor: AppTokens.obsidianElevated,
            valueColor: const AlwaysStoppedAnimation<Color>(AppTokens.amberAccent),
            minHeight: 2.5,
          ),
          InkWell(
            key: const Key('mini_player_tap_area'),
            onTap: () {
              context.push('/books/${book.id}/listen');
            },
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12, vertical: 8),
              child: Row(
                children: [
                  // Book cover thumbnail
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    child: Container(
                      width: 40,
                      height: 40,
                      color: AppTokens.obsidianContainer,
                      child: book.coverUrl != null
                          ? Image.network(
                              book.coverUrl!,
                              fit: BoxFit.cover,
                              errorBuilder: (context, error, stackTrace) =>
                                  const Icon(Icons.headphones, color: AppTokens.amberAccent, size: 20),
                            )
                          : const Icon(Icons.headphones, color: AppTokens.amberAccent, size: 20),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),

                  // Title & Chapter Info
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          book.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: 13,
                            fontWeight: FontWeight.w700,
                            color: Colors.white,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          currentChapter?.title ?? book.authorDisplay,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 11,
                            color: AppTokens.antiqueCream.withValues(alpha: 0.7),
                          ),
                        ),
                      ],
                    ),
                  ),

                  // 30s Skip Forward
                  IconButton(
                    key: const Key('mini_player_skip_30'),
                    iconSize: 22,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
                    icon: const Icon(Icons.forward_30, color: Colors.white70),
                    tooltip: 'Skip forward 30 seconds',
                    onPressed: () => playerNotifier.skipForward(),
                  ),

                  // Play/Pause button
                  IconButton(
                    key: const Key('mini_player_play_pause'),
                    iconSize: 28,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(minWidth: 40, minHeight: 40),
                    icon: Icon(
                      playerState.isPlaying
                          ? Icons.pause_circle_filled
                          : Icons.play_circle_filled,
                      color: AppTokens.amberAccent,
                    ),
                    onPressed: () => playerNotifier.togglePlayPause(),
                  ),

                  // Dismiss / Stop button
                  IconButton(
                    key: const Key('mini_player_close'),
                    iconSize: 18,
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(minWidth: 32, minHeight: 32),
                    icon: const Icon(Icons.close, color: Colors.white38),
                    tooltip: 'Close player',
                    onPressed: () => playerNotifier.stop(),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
