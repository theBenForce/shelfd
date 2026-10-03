import 'package:flutter/material.dart';
import '../../../core/tokens.dart';
import '../../../core/typography.dart';

/// Tactile scrubber bar for audiobook playback with chapter elapsed & remaining timestamps.
class ScrubberBar extends StatefulWidget {
  final Duration position;
  final Duration duration;
  final Duration? chapterPosition;
  final Duration? chapterDuration;
  final ValueChanged<Duration> onSeek;
  final bool isInteractive;

  const ScrubberBar({
    super.key,
    required this.position,
    required this.duration,
    this.chapterPosition,
    this.chapterDuration,
    required this.onSeek,
    this.isInteractive = true,
  });

  @override
  State<ScrubberBar> createState() => _ScrubberBarState();
}

class _ScrubberBarState extends State<ScrubberBar> {
  double? _dragValue;

  String _formatDuration(Duration d) {
    if (d < Duration.zero) d = Duration.zero;
    final hours = d.inHours;
    final minutes = d.inMinutes.remainder(60);
    final seconds = d.inSeconds.remainder(60);
    if (hours > 0) {
      return '${hours.toString().padLeft(2, '0')}:${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}';
    }
    return '${minutes.toString().padLeft(2, '0')}:${seconds.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final effectiveDuration = widget.duration > Duration.zero ? widget.duration : const Duration(seconds: 1);
    final currentPos = widget.position.inMilliseconds.clamp(0, effectiveDuration.inMilliseconds);
    final sliderValue = _dragValue ?? (currentPos / effectiveDuration.inMilliseconds).clamp(0.0, 1.0);

    final remainingDuration = effectiveDuration - widget.position;
    final remainingText = '-${_formatDuration(remainingDuration)}';
    final elapsedText = _formatDuration(widget.position);

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SliderTheme(
          data: SliderTheme.of(context).copyWith(
            trackHeight: 5,
            activeTrackColor: AppTokens.amberAccent,
            inactiveTrackColor: AppTokens.obsidianElevated,
            thumbColor: AppTokens.amberGlow,
            overlayColor: AppTokens.amberAccent.withValues(alpha: 0.2),
            thumbShape: const RoundSliderThumbShape(
              enabledThumbRadius: 7,
              pressedElevation: 4,
            ),
            trackShape: const RoundedRectSliderTrackShape(),
          ),
          child: Slider(
            value: sliderValue,
            min: 0.0,
            max: 1.0,
            onChanged: widget.isInteractive
                ? (val) {
                    setState(() {
                      _dragValue = val;
                    });
                  }
                : null,
            onChangeEnd: widget.isInteractive
                ? (val) {
                    final targetMs = (val * effectiveDuration.inMilliseconds).round();
                    setState(() {
                      _dragValue = null;
                    });
                    widget.onSeek(Duration(milliseconds: targetMs));
                  }
                : null,
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                elapsedText,
                style: AppTypography.captionSans(
                  fontSize: 12,
                  color: AppTokens.antiqueCream.withValues(alpha: 0.8),
                ),
              ),
              if (widget.chapterDuration != null && widget.chapterDuration! > Duration.zero) ...[
                Text(
                  'Chapter ${_formatDuration(widget.chapterPosition ?? Duration.zero)} / ${_formatDuration(widget.chapterDuration!)}',
                  style: AppTypography.captionSans(
                    fontSize: 11,
                    color: AppTokens.mutedCopy,
                  ),
                ),
              ],
              Text(
                remainingText,
                style: AppTypography.captionSans(
                  fontSize: 12,
                  color: AppTokens.antiqueCream.withValues(alpha: 0.8),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
