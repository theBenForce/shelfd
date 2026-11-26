import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

Future<void> showPreviouslyOnSheet(
  BuildContext context, {
  required String bookId,
  required String bookTitle,
  required int chapterIndex,
  int paragraphOffset = 0,
}) {
  final colors = ShelfdColors.of(context);
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: colors.surface,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(
        top: Radius.circular(AppTokens.radiusLg),
      ),
    ),
    builder: (context) => PreviouslyOnSheet(
      bookId: bookId,
      bookTitle: bookTitle,
      chapterIndex: chapterIndex,
      paragraphOffset: paragraphOffset,
    ),
  );
}

class PreviouslyOnSheet extends ConsumerStatefulWidget {
  final String bookId;
  final String bookTitle;
  final int chapterIndex;
  final int paragraphOffset;

  const PreviouslyOnSheet({
    super.key,
    required this.bookId,
    required this.bookTitle,
    required this.chapterIndex,
    this.paragraphOffset = 0,
  });

  @override
  ConsumerState<PreviouslyOnSheet> createState() => _PreviouslyOnSheetState();
}

class _PreviouslyOnSheetState extends ConsumerState<PreviouslyOnSheet> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(bookRecapProvider(widget.bookId).notifier).loadRecap(
            chapterIndex: widget.chapterIndex,
            paragraphOffset: widget.paragraphOffset,
          );
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final recapState = ref.watch(bookRecapProvider(widget.bookId));
    final maxHeight = MediaQuery.of(context).size.height * 0.82;
    final ceilingChapter = recapState.recap?.chapterIndex ?? widget.chapterIndex;

    return SafeArea(
      top: false,
      child: ConstrainedBox(
        constraints: BoxConstraints(maxHeight: maxHeight),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppTokens.space20,
            AppTokens.space12,
            AppTokens.space20,
            AppTokens.space20,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(
                  width: 36,
                  height: 4,
                  decoration: BoxDecoration(
                    color: colors.subtleBorder,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: AppTokens.space16),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Previously On',
                          style: AppTypography.titleSerif(
                            fontSize: 20,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        if (widget.bookTitle.isNotEmpty) ...[
                          const SizedBox(height: 2),
                          Text(
                            widget.bookTitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.bodySans(
                              fontSize: 13,
                              color: colors.secondaryInk,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  IconButton(
                    tooltip: 'Close',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: Icon(Icons.close_rounded, color: colors.secondaryInk),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space12),
              Wrap(
                spacing: AppTokens.space8,
                runSpacing: AppTokens.space8,
                children: [
                  _BadgePill(
                    key: const Key('previously_on_ceiling_badge'),
                    icon: Icons.shield_outlined,
                    label: 'Up to Chapter $ceilingChapter',
                    backgroundColor: colors.accentContainer,
                    foregroundColor: colors.accentPrimary,
                  ),
                  if (recapState.recap?.cached == true)
                    _BadgePill(
                      key: const Key('previously_on_cached_badge'),
                      icon: Icons.bolt_rounded,
                      label: 'Cached',
                      backgroundColor: colors.containerLow,
                      foregroundColor: colors.secondaryInk,
                    ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Divider(height: 1, color: colors.subtleBorder),
              const SizedBox(height: AppTokens.space16),
              Flexible(
                child: _buildBody(context, colors, recapState),
              ),
              const SizedBox(height: AppTokens.space16),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  OutlinedButton.icon(
                    key: const Key('previously_on_regenerate_button'),
                    onPressed: recapState.isLoading
                        ? null
                        : () => ref
                            .read(bookRecapProvider(widget.bookId).notifier)
                            .regenerate(),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.ink,
                      side: BorderSide(color: colors.border),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      ),
                    ),
                    icon: const Icon(Icons.refresh_rounded, size: 18),
                    label: const Text('Regenerate'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    ShelfdColors colors,
    BookRecapState recapState,
  ) {
    if (recapState.isLoading && recapState.recap == null) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppTokens.space32),
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircularProgressIndicator(color: colors.accentPrimary),
              const SizedBox(height: AppTokens.space12),
              Text(
                'Building spoiler-free recap...',
                style: AppTypography.bodySans(
                  fontSize: 14,
                  color: colors.secondaryInk,
                ),
              ),
            ],
          ),
        ),
      );
    }

    if (recapState.error != null) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppTokens.space24),
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline_rounded,
                size: 32,
                color: Theme.of(context).colorScheme.error,
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                'Could not generate recap',
                style: AppTypography.titleSerif(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
              const SizedBox(height: AppTokens.space4),
              Text(
                recapState.error!,
                textAlign: TextAlign.center,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  color: colors.secondaryInk,
                ),
              ),
              const SizedBox(height: AppTokens.space16),
              FilledButton.icon(
                key: const Key('previously_on_retry_button'),
                onPressed: () => ref
                    .read(bookRecapProvider(widget.bookId).notifier)
                    .retry(),
                style: FilledButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                ),
                icon: const Icon(Icons.replay_rounded, size: 18),
                label: const Text('Retry'),
              ),
            ],
          ),
        ),
      );
    }

    final recap = recapState.recap;
    if (recap == null) {
      return const SizedBox.shrink();
    }

    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (recapState.isLoading)
            Padding(
              padding: const EdgeInsets.only(bottom: AppTokens.space12),
              child: LinearProgressIndicator(
                color: colors.accentPrimary,
                backgroundColor: colors.containerLow,
              ),
            ),
          SelectableText(
            recap.recap,
            style: AppTypography.bodySans(
              fontSize: 15,
              color: colors.ink,
            ).copyWith(height: 1.6),
          ),
        ],
      ),
    );
  }
}

class _BadgePill extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color backgroundColor;
  final Color foregroundColor;

  const _BadgePill({
    super.key,
    required this.icon,
    required this.label,
    required this.backgroundColor,
    required this.foregroundColor,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: foregroundColor),
          const SizedBox(width: 6),
          Text(
            label,
            style: AppTypography.mono(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: foregroundColor,
            ),
          ),
        ],
      ),
    );
  }
}
