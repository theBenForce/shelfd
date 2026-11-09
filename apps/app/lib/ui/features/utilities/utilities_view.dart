import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import '../../../data/models/duplicate_group.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class UtilitiesView extends ConsumerStatefulWidget {
  const UtilitiesView({super.key});

  @override
  ConsumerState<UtilitiesView> createState() => _UtilitiesViewState();
}

class _UtilitiesViewState extends ConsumerState<UtilitiesView> {
  MergeOptions _mergeOptions = const MergeOptions();

  @override
  void initState() {
    super.initState();
    // Auto-scan on initial load if not yet scanned
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final state = ref.read(duplicateBooksProvider);
      if (state.result == null && !state.isScanning) {
        ref.read(duplicateBooksProvider.notifier).scanForDuplicates();
      }
    });
  }

  void _showMergeOptionsDialog(
      BuildContext context, DuplicateGroup group, String primaryBookId) {
    bool transferBookmarks = _mergeOptions.transferBookmarks;
    bool transferHighlights = _mergeOptions.transferHighlights;
    bool mergeMetadata = _mergeOptions.mergeMetadata;
    bool deleteFiles = _mergeOptions.deleteFiles;

    showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            final colors = context.shelfdColors;
            return AlertDialog(
              backgroundColor: colors.surface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusLg),
                side: BorderSide(color: colors.border),
              ),
              title: Row(
                children: [
                  Icon(Icons.merge_type_rounded,
                      size: 22, color: colors.accentPrimary),
                  const SizedBox(width: AppTokens.space8),
                  Text(
                    'Merge Options',
                    style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
                  ),
                ],
              ),
              content: SizedBox(
                width: 440,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Configure how duplicate copies are consolidated into the selected primary book:',
                      style: AppTypography.bodySans(
                          fontSize: 13, color: colors.secondaryInk),
                    ),
                    const SizedBox(height: AppTokens.space16),
                    CheckboxListTile(
                      value: transferBookmarks,
                      title: Text(
                        'Transfer Bookmarks',
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      subtitle: Text(
                        'Move all reading bookmarks from duplicate books to primary book',
                        style: AppTypography.captionSans(
                          fontSize: 12,
                          color: colors.mutedInk,
                        ),
                      ),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: colors.accentPrimary,
                      checkColor: colors.onAccent,
                      onChanged: (val) {
                        setDialogState(() => transferBookmarks = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: transferHighlights,
                      title: Text(
                        'Transfer Highlights & Notes',
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      subtitle: Text(
                        'Retain all annotations and highlighted passages',
                        style: AppTypography.captionSans(
                          fontSize: 12,
                          color: colors.mutedInk,
                        ),
                      ),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: colors.accentPrimary,
                      checkColor: colors.onAccent,
                      onChanged: (val) {
                        setDialogState(() => transferHighlights = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: mergeMetadata,
                      title: Text(
                        'Merge Missing Metadata',
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      subtitle: Text(
                        'Fill missing descriptions, publishers, dates, and taxonomy from duplicates',
                        style: AppTypography.captionSans(
                          fontSize: 12,
                          color: colors.mutedInk,
                        ),
                      ),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: colors.accentPrimary,
                      checkColor: colors.onAccent,
                      onChanged: (val) {
                        setDialogState(() => mergeMetadata = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: deleteFiles,
                      title: Text(
                        'Delete Duplicate EPUB Files',
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      subtitle: Text(
                        'Permanently remove redundant duplicate files from disk to free storage',
                        style: AppTypography.captionSans(
                          fontSize: 12,
                          color: colors.mutedInk,
                        ),
                      ),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: colors.accentPrimary,
                      checkColor: colors.onAccent,
                      onChanged: (val) {
                        setDialogState(() => deleteFiles = val ?? true);
                      },
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogCtx).pop(),
                  child: Text(
                    'Cancel',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                      color: colors.secondaryInk,
                    ),
                  ),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.accentPrimary,
                    foregroundColor: colors.onAccent,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    ),
                  ),
                  onPressed: () {
                    setState(() {
                      _mergeOptions = MergeOptions(
                        transferBookmarks: transferBookmarks,
                        transferHighlights: transferHighlights,
                        mergeMetadata: mergeMetadata,
                        deleteFiles: deleteFiles,
                      );
                    });
                    Navigator.of(dialogCtx).pop();
                    ref.read(duplicateBooksProvider.notifier).mergeGroup(
                          group: group,
                          primaryBookId: primaryBookId,
                          options: _mergeOptions,
                        );
                  },
                  child: const Text('Confirm & Merge'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  String _formatFileSize(int? bytes) {
    if (bytes == null || bytes <= 0) return 'Unknown size';
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(duplicateBooksProvider);
    final isMobile = Responsive.isMobile(context);
    final colors = context.shelfdColors;

    return Scaffold(
      backgroundColor: colors.background,
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.symmetric(
            horizontal: isMobile ? AppTokens.space16 : AppTokens.space32,
            vertical: AppTokens.space24,
          ),
          child: Center(
            child: ConstrainedBox(
              constraints:
                  const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Page Header
                  _buildHeader(context, state),
                  const SizedBox(height: AppTokens.space24),

                  // Notifications / Banners
                  if (state.errorMessage != null) ...[
                    _buildAlertBanner(
                      context,
                      message: state.errorMessage!,
                      isError: true,
                      onDismiss: () => ref
                          .read(duplicateBooksProvider.notifier)
                          .clearMessages(),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],
                  if (state.successMessage != null) ...[
                    _buildAlertBanner(
                      context,
                      message: state.successMessage!,
                      isError: false,
                      onDismiss: () => ref
                          .read(duplicateBooksProvider.notifier)
                          .clearMessages(),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  // Stats & Overview Banner
                  _buildOverviewCard(context, state),
                  const SizedBox(height: AppTokens.space24),

                  // Content Area
                  if (state.isScanning) ...[
                    _buildScanningState(context),
                  ] else if (state.result != null &&
                      state.result!.groups.isNotEmpty) ...[
                    _buildDuplicateGroupsList(context, state),
                  ] else if (state.result != null &&
                      state.result!.groups.isEmpty) ...[
                    _buildCleanState(context),
                  ] else ...[
                    _buildInitialScanPrompt(context),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, DuplicateBooksState state) {
    final isMobile = Responsive.isMobile(context);
    final colors = context.shelfdColors;

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Utilities & Maintenance',
                style: AppTypography.titleSerif(
                  fontSize: isMobile ? 24 : 30,
                  fontWeight: FontWeight.w700,
                  color: colors.ink,
                ),
              ),
              const SizedBox(height: AppTokens.space4),
              Text(
                'Consolidate duplicates, clean library metadata, and optimize storage.',
                style: AppTypography.bodySans(
                  fontSize: 14,
                  color: colors.secondaryInk,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(width: AppTokens.space16),
        PrimaryButton(
          label: state.isScanning ? 'Scanning...' : 'Scan for Duplicates',
          icon: Icons.sync_rounded,
          isLoading: state.isScanning,
          onPressed: state.isScanning
              ? null
              : () =>
                  ref.read(duplicateBooksProvider.notifier).scanForDuplicates(),
        ),
      ],
    );
  }

  Widget _buildAlertBanner(
    BuildContext context, {
    required String message,
    required bool isError,
    required VoidCallback onDismiss,
  }) {
    final colors = context.shelfdColors;
    final bg = isError
        ? colors.errorBg
        : (colors.isDark
            ? colors.accentSecondaryContainer
            : AppTokens.completedBadgeBg);
    final border = isError
        ? colors.errorBorder
        : colors.accentSecondary.withValues(alpha: 0.35);
    final fgColor = isError ? colors.errorText : colors.accentSecondary;

    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space16, vertical: AppTokens.space12),
      decoration: BoxDecoration(
        color: bg,
        border: Border.all(color: border),
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      ),
      child: Row(
        children: [
          Icon(
              isError
                  ? Icons.error_outline_rounded
                  : Icons.check_circle_outline_rounded,
              size: 20,
              color: fgColor),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Text(
              message,
              style: AppTypography.bodySans(
                fontSize: 13,
                fontWeight: FontWeight.w500,
                color: fgColor,
              ),
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close_rounded, size: 16),
            color: fgColor,
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
            onPressed: onDismiss,
          ),
        ],
      ),
    );
  }

  Widget _buildOverviewCard(BuildContext context, DuplicateBooksState state) {
    final colors = context.shelfdColors;
    final totalGroups = state.result?.totalGroups ?? 0;
    final totalDuplicates = state.result?.totalDuplicateBooks ?? 0;

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space20),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: colors.containerLow,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: Border.all(color: colors.subtleBorder),
            ),
            child: Icon(
              Icons.copy_all_rounded,
              color: colors.accentPrimary,
              size: 24,
            ),
          ),
          const SizedBox(width: AppTokens.space16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      'Duplicate Books Finder',
                      style: AppTypography.titleSerif(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: colors.ink,
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    StatusBadge(
                      label: state.isScanning
                          ? 'Scanning...'
                          : state.result == null
                              ? 'Idle'
                              : totalGroups > 0
                                  ? '$totalGroups Groups Found'
                                  : 'Library Clean',
                      backgroundColor: totalGroups > 0
                          ? colors.accentContainer
                          : (colors.isDark
                              ? colors.accentSecondaryContainer
                              : AppTokens.completedBadgeBg),
                      textColor: totalGroups > 0
                          ? colors.accentPrimary
                          : colors.accentSecondary,
                    ),
                  ],
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  'Identifies matching ISBN identifiers, exact title/author pairs, and subtitle variations to merge redundant catalog entries.',
                  style: AppTypography.bodySans(
                      fontSize: 13, color: colors.secondaryInk),
                ),
              ],
            ),
          ),
          if (state.result != null) ...[
            const SizedBox(width: AppTokens.space24),
            _buildStatPill(context, 'Groups', '$totalGroups'),
            const SizedBox(width: AppTokens.space12),
            _buildStatPill(context, 'Redundant Copies',
                '${totalDuplicates > 0 ? totalDuplicates - totalGroups : 0}'),
          ],
        ],
      ),
    );
  }

  Widget _buildStatPill(BuildContext context, String label, String value) {
    final colors = context.shelfdColors;
    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space16, vertical: AppTokens.space8),
      decoration: BoxDecoration(
        color: colors.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: colors.subtleBorder),
      ),
      child: Column(
        children: [
          Text(
            value,
            style: AppTypography.mono(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: colors.ink,
            ),
          ),
          Text(
            label,
            style: AppTypography.captionSans(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: colors.mutedInk,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildScanningState(BuildContext context) {
    final colors = context.shelfdColors;
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            CircularProgressIndicator(
              strokeWidth: 3,
              color: colors.accentPrimary,
            ),
            const SizedBox(height: AppTokens.space20),
            Text(
              'Scanning library for duplicate books...',
              style: AppTypography.titleSerif(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Analyzing ISBN identifiers, author attributions, and title variations.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: colors.secondaryInk),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCleanState(BuildContext context) {
    final colors = context.shelfdColors;
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: colors.isDark
                    ? colors.accentSecondaryContainer
                    : AppTokens.completedBadgeBg,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.check_circle_outline_rounded,
                color: colors.accentSecondary,
                size: 32,
              ),
            ),
            const SizedBox(height: AppTokens.space16),
            Text(
              'No Duplicate Books Found',
              style: AppTypography.titleSerif(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'All books in your library have distinct identifiers and unique titles.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: colors.secondaryInk),
            ),
            const SizedBox(height: AppTokens.space20),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: colors.ink,
                side: BorderSide(color: colors.border),
              ),
              onPressed: () =>
                  ref.read(duplicateBooksProvider.notifier).scanForDuplicates(),
              icon: const Icon(Icons.sync_rounded, size: 16),
              label: const Text('Re-scan Library'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInitialScanPrompt(BuildContext context) {
    final colors = context.shelfdColors;
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            Icon(Icons.search_rounded, size: 40, color: colors.mutedInk),
            const SizedBox(height: AppTokens.space16),
            Text(
              'Duplicate Detection Ready',
              style: AppTypography.titleSerif(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Scan your collection to discover and merge duplicate books.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: colors.secondaryInk),
            ),
            const SizedBox(height: AppTokens.space20),
            PrimaryButton(
              label: 'Scan for Duplicates',
              icon: Icons.play_arrow_rounded,
              onPressed: () =>
                  ref.read(duplicateBooksProvider.notifier).scanForDuplicates(),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDuplicateGroupsList(
      BuildContext context, DuplicateBooksState state) {
    final colors = context.shelfdColors;
    final groups = state.result!.groups;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              'Detected Duplicate Sets (${groups.length})',
              style: AppTypography.titleSerif(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: colors.ink,
              ),
            ),
            Text(
              'Select primary copy before merging',
              style: AppTypography.bodySans(
                  fontSize: 12, color: colors.mutedInk),
            ),
          ],
        ),
        const SizedBox(height: AppTokens.space16),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: groups.length,
          separatorBuilder: (context, index) =>
              const SizedBox(height: AppTokens.space20),
          itemBuilder: (context, index) {
            final group = groups[index];
            final selectedPrimaryId = state.selectedPrimaryBooks[group.id] ??
                (group.books.isNotEmpty ? group.books.first.id : '');
            final isMergingThisGroup =
                state.isMerging && state.mergingGroupId == group.id;

            return _buildDuplicateGroupCard(
              context: context,
              group: group,
              selectedPrimaryId: selectedPrimaryId,
              isMerging: isMergingThisGroup,
            );
          },
        ),
      ],
    );
  }

  Widget _buildDuplicateGroupCard({
    required BuildContext context,
    required DuplicateGroup group,
    required String selectedPrimaryId,
    required bool isMerging,
  }) {
    final colors = context.shelfdColors;
    final confidencePct = (group.confidence * 100).toInt();
    final isHighConfidence = group.confidence >= 0.95;
    final confidenceColor =
        isHighConfidence ? colors.accentSecondary : colors.accentPrimary;
    final confidenceBg = isHighConfidence
        ? (colors.isDark
            ? colors.accentSecondaryContainer
            : AppTokens.completedBadgeBg)
        : colors.accentContainer;

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Group Header
          Row(
            children: [
              Expanded(
                child: Row(
                  children: [
                    Icon(
                      Icons.auto_awesome_rounded,
                      size: 18,
                      color: confidenceColor,
                    ),
                    const SizedBox(width: AppTokens.space8),
                    Text(
                      group.matchReason,
                      style: AppTypography.titleSerif(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: colors.ink,
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    StatusBadge(
                      label: '$confidencePct% Match',
                      backgroundColor: confidenceBg,
                      textColor: confidenceColor,
                    ),
                  ],
                ),
              ),
              // Action Buttons
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  visualDensity: VisualDensity.compact,
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                ),
                onPressed: isMerging
                    ? null
                    : () => _showMergeOptionsDialog(
                        context, group, selectedPrimaryId),
                icon: const Icon(Icons.tune_rounded, size: 16),
                label: const Text('Options', style: TextStyle(fontSize: 12)),
              ),
              const SizedBox(width: AppTokens.space8),
              PrimaryButton(
                label: isMerging ? 'Merging...' : 'Merge Group',
                icon: Icons.call_merge_rounded,
                isLoading: isMerging,
                onPressed: isMerging
                    ? null
                    : () {
                        ref.read(duplicateBooksProvider.notifier).mergeGroup(
                              group: group,
                              primaryBookId: selectedPrimaryId,
                              options: _mergeOptions,
                            );
                      },
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space8),
          Text(
            'The selected primary book will be preserved. Bookmarks, highlights, and missing metadata will be transferred from other versions.',
            style: AppTypography.bodySans(
                fontSize: 12, color: colors.secondaryInk),
          ),
          const SizedBox(height: AppTokens.space16),

          // Books in group comparison
          Wrap(
            spacing: AppTokens.space16,
            runSpacing: AppTokens.space16,
            children: group.books.map((book) {
              final isPrimary = book.id == selectedPrimaryId;
              return _buildBookCandidateCard(
                context: context,
                book: book,
                isPrimary: isPrimary,
                onSelect: () {
                  ref
                      .read(duplicateBooksProvider.notifier)
                      .selectPrimaryBook(group.id, book.id);
                },
              );
            }).toList(),
          ),
        ],
      ),
    );
  }

  Widget _buildBookCandidateCard({
    required BuildContext context,
    required Book book,
    required bool isPrimary,
    required VoidCallback onSelect,
  }) {
    final colors = context.shelfdColors;

    return InkWell(
      onTap: onSelect,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Container(
        width: 340,
        padding: const EdgeInsets.all(AppTokens.space12),
        decoration: BoxDecoration(
          color: isPrimary ? colors.containerLow : colors.surface,
          border: Border.all(
            color: isPrimary ? colors.accentPrimary : colors.border,
            width: isPrimary ? 2 : 1,
          ),
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Cover Image Thumbnail
            SizedBox(
              width: 54,
              height: 78,
              child: TactileBookCover(
                coverUrl: book.coverUrl,
                title: book.title,
                showFallbackTitle: false,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                showShadow: false,
              ),
            ),
            const SizedBox(width: AppTokens.space12),

            // Book Details
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        isPrimary
                            ? Icons.radio_button_checked
                            : Icons.radio_button_off,
                        size: 16,
                        color: isPrimary
                            ? colors.accentPrimary
                            : colors.mutedInk,
                      ),
                      const SizedBox(width: AppTokens.space4),
                      if (isPrimary)
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: colors.accentPrimary,
                            borderRadius:
                                BorderRadius.circular(AppTokens.radiusPill),
                          ),
                          child: Text(
                            'PRIMARY',
                            style: AppTypography.mono(
                              fontSize: 9,
                              fontWeight: FontWeight.w700,
                              color: colors.onAccent,
                              letterSpacing: 0.5,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: AppTokens.space4),
                  Text(
                    book.title,
                    style: AppTypography.titleSerif(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: colors.ink,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2.0),
                  Text(
                    book.authorDisplay,
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      color: colors.secondaryInk,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 6.0),

                  // Metadata Badges
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: [
                      _buildSmallBadge(
                          context, _formatFileSize(book.fileSizeBytes)),
                      if (book.publishedDate != null &&
                          book.publishedDate!.isNotEmpty)
                        _buildSmallBadge(context, book.publishedDate!),
                      if (book.bookmarks.isNotEmpty)
                        _buildSmallBadge(
                            context, '${book.bookmarks.length} 🔖'),
                      if (book.highlights.isNotEmpty)
                        _buildSmallBadge(
                            context, '${book.highlights.length} ✏️'),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSmallBadge(BuildContext context, String text) {
    final colors = context.shelfdColors;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: colors.container,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
      ),
      child: Text(
        text,
        style: AppTypography.mono(
          fontSize: 10,
          fontWeight: FontWeight.w500,
          color: colors.secondaryInk,
        ),
      ),
    );
  }
}
