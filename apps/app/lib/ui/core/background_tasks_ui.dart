import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../data/models/background_job.dart';
import '../state/providers.dart';
import 'responsive.dart';
import 'theme.dart';
import 'tokens.dart';
import 'typography.dart';

IconData _iconForJobKind(String kind) {
  switch (kind) {
    case BackgroundJobKind.plugin:
      return Icons.extension_rounded;
    case BackgroundJobKind.libraryScan:
      return Icons.sync_rounded;
    case BackgroundJobKind.aiIndexing:
      return Icons.auto_awesome_rounded;
    case BackgroundJobKind.uploadIngest:
      return Icons.cloud_upload_rounded;
    default:
      return Icons.task_alt_rounded;
  }
}

String _formatRelativeTime(DateTime? dt) {
  if (dt == null) return '';
  final diff = DateTime.now().toUtc().difference(dt.toUtc());
  if (diff.inSeconds < 60) return 'Just now';
  if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
  if (diff.inHours < 24) return '${diff.inHours}h ago';
  return '${diff.inDays}d ago';
}

/// Opens the standardized Background Tasks panel (dialog on desktop, bottom sheet on mobile).
Future<void> showBackgroundTasksPanel(BuildContext context) {
  final isDesktop = Responsive.isDesktop(context);
  final colors = ShelfdColors.of(context);
  final router = GoRouter.maybeOf(context);

  if (isDesktop) {
    return showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return Dialog(
          backgroundColor: colors.surface,
          surfaceTintColor: Colors.transparent,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusLg),
            side: BorderSide(color: colors.border),
          ),
          insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 520, maxHeight: 620),
            child: BackgroundTasksPanelContent(
              onNavigate: (route) {
                Navigator.of(dialogContext).pop();
                router?.go(route);
              },
            ),
          ),
        );
      },
    );
  }

  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: colors.surface,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(
        top: Radius.circular(AppTokens.radiusLg),
      ),
    ),
    builder: (sheetContext) {
      return DraggableScrollableSheet(
        initialChildSize: 0.65,
        minChildSize: 0.35,
        maxChildSize: 0.9,
        expand: false,
        builder: (_, scrollController) {
          return BackgroundTasksPanelContent(
            scrollController: scrollController,
            onNavigate: (route) {
              Navigator.of(sheetContext).pop();
              router?.go(route);
            },
          );
        },
      );
    },
  );
}

/// Compact activity indicator placed in the desktop [ShelfdSideNav] footer.
class BackgroundTasksSidebarButton extends ConsumerWidget {
  const BackgroundTasksSidebarButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final state = ref.watch(backgroundJobsProvider);
    final activeCount = state.activeJobs.length;
    final failedCount = state.failedJobs.length;
    final completedCount = state.completedJobs.length;
    final primary = state.primaryActiveJob;
    final progress = state.aggregateProgress;

    if (activeCount > 0 && primary != null) {
      final headerText = activeCount == 1
          ? primary.displayTitle
          : '$activeCount tasks running';
      final pctText = progress != null ? '${(progress * 100).round()}%' : null;
      final subtext = activeCount == 1
          ? (primary.itemTitle != null && primary.itemTitle!.trim().isNotEmpty
                ? '${primary.itemTitle!.trim()} • ${primary.displayStatusMessage}'
                : primary.displayStatusMessage)
          : primary.displayTitle;

      return Material(
        color: Colors.transparent,
        child: InkWell(
          key: const Key('background_tasks_sidebar_button'),
          onTap: () => showBackgroundTasksPanel(context),
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          child: Container(
            padding: const EdgeInsets.all(AppTokens.space12),
            decoration: BoxDecoration(
              color: colors.surface,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: Border.all(color: colors.border),
              boxShadow: [
                BoxShadow(
                  color: colors.cardShadowColor,
                  blurRadius: 8,
                  offset: const Offset(0, 2),
                ),
              ],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: colors.accentPrimary,
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    Expanded(
                      child: Text(
                        headerText,
                        style: AppTypography.bodySans(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (pctText != null) ...[
                      const SizedBox(width: AppTokens.space4),
                      Text(
                        pctText,
                        style: AppTypography.mono(
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                          color: colors.accentPrimary,
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: AppTokens.space8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(2),
                  child: LinearProgressIndicator(
                    key: const Key('background_tasks_sidebar_progress'),
                    value: progress,
                    minHeight: 4,
                    backgroundColor: colors.container,
                    valueColor: AlwaysStoppedAnimation<Color>(
                      colors.accentPrimary,
                    ),
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  subtext,
                  style: AppTypography.bodySans(
                    fontSize: 10,
                    color: colors.mutedInk,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ),
      );
    }

    final bool hasErrors = failedCount > 0;
    final bool hasCompleted = completedCount > 0;
    final Color bg = hasErrors
        ? colors.errorBg
        : colors.container.withValues(alpha: 0.65);
    final Color borderColor = hasErrors
        ? colors.errorBorder
        : colors.subtleBorder;
    final Color iconColor = hasErrors
        ? colors.errorText
        : (hasCompleted ? colors.accentSecondary : colors.mutedInk);
    final IconData icon = hasErrors
        ? Icons.error_outline_rounded
        : (hasCompleted
              ? Icons.check_circle_outline_rounded
              : Icons.checklist_rounded);
    final String label = hasErrors
        ? (failedCount == 1 ? '1 task failed' : '$failedCount tasks failed')
        : (hasCompleted
              ? (completedCount == 1
                    ? '1 task completed'
                    : '$completedCount tasks completed')
              : 'Background Tasks');

    return Material(
      color: Colors.transparent,
      child: InkWell(
        key: const Key('background_tasks_sidebar_button'),
        onTap: () => showBackgroundTasksPanel(context),
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: borderColor),
          ),
          child: Row(
            children: [
              Icon(icon, size: 15, color: iconColor),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  label,
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    fontWeight: hasErrors ? FontWeight.w600 : FontWeight.w500,
                    color: hasErrors ? colors.errorText : colors.secondaryInk,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              Icon(
                Icons.open_in_new_rounded,
                size: 13,
                color: hasErrors ? colors.errorText : colors.mutedInk,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Compact activity icon button with badge/progress for [ShelfdTopBar] on mobile/tablet.
class BackgroundTasksBadgeButton extends ConsumerWidget {
  const BackgroundTasksBadgeButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final state = ref.watch(backgroundJobsProvider);
    final activeCount = state.activeJobs.length;
    final failedCount = state.failedJobs.length;
    final totalCount = state.jobs.length;

    final bool showBadge = activeCount > 0 || failedCount > 0 || totalCount > 0;
    final int badgeCount = activeCount > 0
        ? activeCount
        : (failedCount > 0 ? failedCount : totalCount);
    final Color badgeColor = failedCount > 0 && activeCount == 0
        ? colors.errorText
        : colors.accentPrimary;

    final String tooltip = activeCount > 0
        ? '$activeCount active background ${activeCount == 1 ? 'task' : 'tasks'}'
        : (failedCount > 0
              ? '$failedCount failed ${failedCount == 1 ? 'task' : 'tasks'}'
              : 'Background Tasks');

    return IconButton(
      key: const Key('background_tasks_badge_button'),
      tooltip: tooltip,
      icon: Badge(
        isLabelVisible: showBadge,
        label: Text(
          '$badgeCount',
          style: AppTypography.mono(
            fontWeight: FontWeight.w700,
            fontSize: 10,
            color: colors.onAccent,
          ),
        ),
        backgroundColor: badgeColor,
        textColor: colors.onAccent,
        child: activeCount > 0
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2.2,
                  color: colors.accentPrimary,
                ),
              )
            : Icon(
                failedCount > 0
                    ? Icons.error_outline_rounded
                    : Icons.checklist_rounded,
                size: 22,
                color: failedCount > 0 ? colors.errorText : colors.ink,
              ),
      ),
      onPressed: () => showBackgroundTasksPanel(context),
    );
  }
}

/// Non-visual listener mounted in [ShelfdAdaptiveScaffold] that surfaces error toasts
/// when any background task fails, with a quick action to open the Tasks panel.
class BackgroundTasksErrorListener extends ConsumerWidget {
  const BackgroundTasksErrorListener({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.listen<BackgroundJobsState>(backgroundJobsProvider, (previous, next) {
      final err = next.latestError;
      if (err != null && err.id != previous?.latestError?.id) {
        final messenger = ScaffoldMessenger.maybeOf(context);
        final colors = ShelfdColors.of(context);
        messenger?.showSnackBar(
          SnackBar(
            key: const Key('background_task_error_toast'),
            content: Text(
              '${err.displayTitle} failed: ${err.displayStatusMessage}',
              style: AppTypography.bodySans(fontSize: 13, color: Colors.white),
            ),
            backgroundColor: Colors.red.shade800,
            behavior: SnackBarBehavior.floating,
            action: SnackBarAction(
              label: 'View',
              textColor: colors.onAccent,
              onPressed: () {
                if (context.mounted) {
                  showBackgroundTasksPanel(context);
                }
              },
            ),
          ),
        );
        ref.read(backgroundJobsProvider.notifier).acknowledgeError(err.id);
      }
    });

    return const SizedBox.shrink();
  }
}

/// Standardized scrollable list of active and recent background tasks.
class BackgroundTasksPanelContent extends ConsumerWidget {
  final ScrollController? scrollController;
  final ValueChanged<String>? onNavigate;

  const BackgroundTasksPanelContent({
    super.key,
    this.scrollController,
    this.onNavigate,
  });

  void _navigateToRoute(BuildContext context, String route) {
    if (onNavigate != null) {
      onNavigate!(route);
      return;
    }
    final router = GoRouter.maybeOf(context);
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    }
    router?.go(route);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final state = ref.watch(backgroundJobsProvider);
    final active = state.activeJobs;
    final finished = state.finishedJobs;

    return Column(
      key: const Key('background_tasks_panel'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Header
        Padding(
          padding: const EdgeInsets.fromLTRB(
            AppTokens.space20,
            AppTokens.space16,
            AppTokens.space12,
            AppTokens.space12,
          ),
          child: Row(
            children: [
              Icon(
                Icons.checklist_rounded,
                size: 20,
                color: colors.accentPrimary,
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Background Tasks',
                  style: AppTypography.titleSerif(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: colors.ink,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (finished.isNotEmpty)
                TextButton(
                  key: const Key('background_tasks_clear_finished'),
                  onPressed: () =>
                      ref.read(backgroundJobsProvider.notifier).clearFinished(),
                  child: Text(
                    'Clear finished',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: colors.accentPrimary,
                    ),
                  ),
                ),
              IconButton(
                tooltip: 'Close',
                icon: Icon(Icons.close_rounded, size: 20, color: colors.mutedInk),
                onPressed: () => Navigator.of(context).maybePop(),
              ),
            ],
          ),
        ),
        Divider(color: colors.subtleBorder, height: 1),

        // Body
        Flexible(
          child: state.jobs.isEmpty
              ? Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space24,
                    vertical: AppTokens.space32,
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.check_circle_outline_rounded,
                        size: 36,
                        color: colors.mutedInk.withValues(alpha: 0.6),
                      ),
                      const SizedBox(height: AppTokens.space12),
                      Text(
                        'All quiet',
                        style: AppTypography.titleSerif(
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Library rescans, AI indexing, uploads, and plugin exports will appear here.',
                        textAlign: TextAlign.center,
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          color: colors.mutedInk,
                        ),
                      ),
                    ],
                  ),
                )
              : ListView(
                  controller: scrollController,
                  shrinkWrap: true,
                  padding: const EdgeInsets.all(AppTokens.space16),
                  children: [
                    if (active.isNotEmpty) ...[
                      Padding(
                        padding: const EdgeInsets.only(
                          left: AppTokens.space4,
                          bottom: AppTokens.space8,
                        ),
                        child: Text(
                          'ACTIVE (${active.length})',
                          style: AppTypography.labelCaps(
                            fontSize: 10,
                            color: colors.mutedInk,
                            letterSpacing: 1.0,
                          ),
                        ),
                      ),
                      for (final job in active) ...[
                        _BackgroundTaskCard(job: job),
                        const SizedBox(height: AppTokens.space8),
                      ],
                    ],
                    if (finished.isNotEmpty) ...[
                      if (active.isNotEmpty)
                        const SizedBox(height: AppTokens.space8),
                      Padding(
                        padding: const EdgeInsets.only(
                          left: AppTokens.space4,
                          bottom: AppTokens.space8,
                        ),
                        child: Text(
                          'RECENT (${finished.length})',
                          style: AppTypography.labelCaps(
                            fontSize: 10,
                            color: colors.mutedInk,
                            letterSpacing: 1.0,
                          ),
                        ),
                      ),
                      for (final job in finished) ...[
                        _BackgroundTaskCard(
                          job: job,
                          onReviewQueue: job.kind == BackgroundJobKind.plugin &&
                                  job.metadata?['action'] == 'catalog_stage' &&
                                  job.isCompleted
                              ? () => _navigateToRoute(context, '/uploads')
                              : null,
                          onDismiss: () => ref
                              .read(backgroundJobsProvider.notifier)
                              .dismissJob(job.id),
                        ),
                        const SizedBox(height: AppTokens.space8),
                      ],
                    ],
                  ],
                ),
        ),
      ],
    );
  }
}

class _BackgroundTaskCard extends StatelessWidget {
  final BackgroundJob job;
  final VoidCallback? onReviewQueue;
  final VoidCallback? onDismiss;

  const _BackgroundTaskCard({
    required this.job,
    this.onReviewQueue,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final pct = (job.progressFraction * 100).round();
    final timeStr = _formatRelativeTime(job.completedAt ?? job.updatedAt);

    final Color iconBg = job.isFailed
        ? colors.errorBg
        : (job.isCompleted
              ? colors.accentSecondaryContainer
              : colors.accentPrimary.withValues(alpha: 0.14));
    final Color iconColor = job.isFailed
        ? colors.errorText
        : (job.isCompleted ? colors.accentSecondary : colors.accentPrimary);

    final String? counterText = job.isActive
        ? (job.total > 1
              ? '${job.current}/${job.total} ($pct%)'
              : (!job.isIndeterminate ? '$pct%' : null))
        : null;

    return Container(
      key: Key('background_task_card_${job.id}'),
      padding: const EdgeInsets.all(AppTokens.space12),
      decoration: BoxDecoration(
        color: colors.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(
          color: job.isFailed ? colors.errorBorder : colors.subtleBorder,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(6),
                decoration: BoxDecoration(
                  color: iconBg,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
                child: Icon(
                  job.isFailed
                      ? Icons.error_outline_rounded
                      : (job.isCompleted
                            ? Icons.check_circle_outline_rounded
                            : _iconForJobKind(job.kind)),
                  size: 16,
                  color: iconColor,
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            job.displayTitle,
                            style: AppTypography.bodySans(
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                              color: colors.ink,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        const SizedBox(width: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: colors.container,
                            borderRadius: BorderRadius.circular(
                              AppTokens.radiusPill,
                            ),
                          ),
                          child: Text(
                            job.kindLabel,
                            style: AppTypography.mono(
                              fontSize: 10,
                              fontWeight: FontWeight.w600,
                              color: colors.secondaryInk,
                            ),
                          ),
                        ),
                      ],
                    ),
                    if (job.itemTitle != null &&
                        job.itemTitle!.trim().isNotEmpty &&
                        job.itemTitle!.trim() != job.displayTitle) ...[
                      const SizedBox(height: 2),
                      Text(
                        job.itemTitle!.trim(),
                        style: AppTypography.bodySans(
                          fontSize: 11,
                          fontWeight: FontWeight.w500,
                          color: colors.secondaryInk,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ],
                ),
              ),
              if (onDismiss != null)
                IconButton(
                  key: Key('dismiss_background_task_${job.id}'),
                  tooltip: 'Dismiss',
                  visualDensity: VisualDensity.compact,
                  icon: Icon(
                    Icons.close_rounded,
                    size: 16,
                    color: colors.mutedInk,
                  ),
                  onPressed: onDismiss,
                ),
            ],
          ),
          const SizedBox(height: AppTokens.space8),
          if (job.isActive) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(3),
              child: LinearProgressIndicator(
                value: job.isIndeterminate ? null : job.progressFraction,
                minHeight: 5,
                backgroundColor: colors.containerHigh,
                valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
              ),
            ),
            const SizedBox(height: 6),
          ],
          Row(
            children: [
              Expanded(
                child: Text(
                  job.displayStatusMessage,
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    color: job.isFailed ? colors.errorText : colors.mutedInk,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (onReviewQueue != null) ...[
                const SizedBox(width: AppTokens.space8),
                TextButton.icon(
                  key: Key('background_task_review_queue_${job.id}'),
                  style: TextButton.styleFrom(
                    foregroundColor: colors.accentPrimary,
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space8,
                      vertical: 4,
                    ),
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  onPressed: onReviewQueue,
                  icon: const Icon(Icons.fact_check_outlined, size: 13),
                  label: Text(
                    'Review Queue',
                    style: AppTypography.bodySans(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: colors.accentPrimary,
                    ),
                  ),
                ),
              ],
              if (counterText != null) ...[
                const SizedBox(width: AppTokens.space8),
                Text(
                  counterText,
                  style: AppTypography.mono(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: colors.accentPrimary,
                  ),
                ),
              ] else if (timeStr.isNotEmpty) ...[
                const SizedBox(width: AppTokens.space8),
                Text(
                  timeStr,
                  style: AppTypography.mono(
                    fontSize: 10,
                    color: colors.mutedInk,
                  ),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}
