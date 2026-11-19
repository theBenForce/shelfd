import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import '../../../data/models/duplicate_group.dart';
import '../../core/primitives.dart';
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
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final state = ref.read(duplicateBooksProvider);
      if (state.result == null && !state.isScanning) {
        ref.read(duplicateBooksProvider.notifier).scanForDuplicates();
      }
    });
  }

  void _showMergeOptionsDialog(
    BuildContext context,
    DuplicateGroup group,
    String primaryBookId,
  ) {
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
              title: Row(
                children: [
                  Icon(
                    Icons.merge_type_rounded,
                    size: 20,
                    color: colors.accentPrimary,
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Text(
                      'Merge Options',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.titleSerif(
                        fontSize: 18,
                        color: colors.ink,
                      ),
                    ),
                  ),
                ],
              ),
              content: SizedBox(
                width: 420,
                child: Material(
                  type: MaterialType.transparency,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Configure how duplicate copies consolidate into the selected primary book:',
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          color: colors.secondaryInk,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space12),
                      _buildMergeOptionTile(
                        colors: colors,
                        value: transferBookmarks,
                        title: 'Transfer Bookmarks',
                        subtitle: 'Move reading bookmarks to the primary book',
                        onChanged: (val) => setDialogState(
                          () => transferBookmarks = val ?? true,
                        ),
                      ),
                      _buildMergeOptionTile(
                        colors: colors,
                        value: transferHighlights,
                        title: 'Transfer Highlights & Notes',
                        subtitle: 'Retain all annotations and passages',
                        onChanged: (val) => setDialogState(
                          () => transferHighlights = val ?? true,
                        ),
                      ),
                      _buildMergeOptionTile(
                        colors: colors,
                        value: mergeMetadata,
                        title: 'Merge Missing Metadata',
                        subtitle:
                            'Fill missing descriptions, publishers, dates, and tags',
                        onChanged: (val) =>
                            setDialogState(() => mergeMetadata = val ?? true),
                      ),
                      _buildMergeOptionTile(
                        colors: colors,
                        value: deleteFiles,
                        title: 'Delete Duplicate EPUB Files',
                        subtitle: 'Remove redundant files from disk',
                        onChanged: (val) =>
                            setDialogState(() => deleteFiles = val ?? true),
                      ),
                    ],
                  ),
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
                FilledButton(
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

  Widget _buildMergeOptionTile({
    required ShelfdColors colors,
    required bool value,
    required String title,
    required String subtitle,
    required ValueChanged<bool?> onChanged,
  }) {
    return CheckboxListTile(
      value: value,
      title: Text(
        title,
        style: AppTypography.bodySans(
          fontSize: 14,
          fontWeight: FontWeight.w600,
          color: colors.ink,
        ),
      ),
      subtitle: Text(
        subtitle,
        style: AppTypography.captionSans(fontSize: 12, color: colors.mutedInk),
      ),
      contentPadding: EdgeInsets.zero,
      controlAffinity: ListTileControlAffinity.leading,
      activeColor: colors.accentPrimary,
      checkColor: colors.onAccent,
      onChanged: onChanged,
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
    final isDesktop = Responsive.isDesktop(context);
    final horizontalPad = Responsive.horizontalPadding(context);
    final colors = context.shelfdColors;

    return Scaffold(
      backgroundColor: colors.background,
      appBar: isDesktop
          ? null
          : ShelfdTopBar(
              title: 'Utilities & Maintenance',
              subtitle: 'Find and merge duplicate books',
              actions: [
                IconButton(
                  tooltip: 'Scan for Duplicates',
                  icon: state.isScanning
                      ? SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: colors.accentPrimary,
                          ),
                        )
                      : Icon(Icons.sync_rounded, color: colors.ink),
                  onPressed: state.isScanning
                      ? null
                      : () => ref
                          .read(duplicateBooksProvider.notifier)
                          .scanForDuplicates(),
                ),
              ],
            ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPad,
            vertical: AppTokens.space24,
          ),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: AppTokens.maxWorkspaceWidth,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (isDesktop) ...[
                    ShelfdPageHeader(
                      title: 'Utilities & Maintenance',
                      subtitle:
                          'Consolidate duplicate editions and keep your catalog clean.',
                      trailing: PrimaryButton(
                        label: state.isScanning
                            ? 'Scanning...'
                            : 'Scan for Duplicates',
                        icon: Icons.sync_rounded,
                        isLoading: state.isScanning,
                        onPressed: state.isScanning
                            ? null
                            : () => ref
                                .read(duplicateBooksProvider.notifier)
                                .scanForDuplicates(),
                      ),
                    ),
                    const SizedBox(height: AppTokens.space20),
                  ],

                  if (state.errorMessage != null) ...[
                    ShelfdInlineBanner(
                      message: state.errorMessage!,
                      tone: ShelfdBannerTone.error,
                      onDismiss: () => ref
                          .read(duplicateBooksProvider.notifier)
                          .clearMessages(),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],
                  if (state.successMessage != null) ...[
                    ShelfdInlineBanner(
                      message: state.successMessage!,
                      tone: ShelfdBannerTone.success,
                      onDismiss: () => ref
                          .read(duplicateBooksProvider.notifier)
                          .clearMessages(),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  // Compact status strip
                  _buildStatusStrip(context, state),
                  const SizedBox(height: AppTokens.space20),

                  // Main content area
                  if (state.isScanning)
                    _buildScanningState(context)
                  else if (state.result != null &&
                      state.result!.groups.isNotEmpty)
                    _buildDuplicateGroupsList(context, state)
                  else if (state.result != null && state.result!.groups.isEmpty)
                    _buildCleanState(context)
                  else
                    _buildInitialScanPrompt(context),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildStatusStrip(BuildContext context, DuplicateBooksState state) {
    final colors = context.shelfdColors;
    final totalGroups = state.result?.totalGroups ?? 0;
    final totalDuplicates = state.result?.totalDuplicateBooks ?? 0;
    final redundantCount =
        totalDuplicates > totalGroups ? totalDuplicates - totalGroups : 0;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space16,
        vertical: AppTokens.space12,
      ),
      decoration: BoxDecoration(
        color: colors.surface,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: colors.subtleBorder),
      ),
      child: Wrap(
        alignment: WrapAlignment.spaceBetween,
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: AppTokens.space16,
        runSpacing: AppTokens.space12,
        children: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.copy_all_rounded,
                size: 18,
                color: colors.accentPrimary,
              ),
              const SizedBox(width: AppTokens.space8),
              Flexible(
                child: Text(
                  'Duplicate Books Finder',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
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
          if (state.result != null)
            Wrap(
              spacing: AppTokens.space16,
              runSpacing: AppTokens.space8,
              children: [
                _buildInlineMetric(context, 'Groups', '$totalGroups'),
                _buildInlineMetric(context, 'Redundant', '$redundantCount'),
              ],
            ),
        ],
      ),
    );
  }

  Widget _buildInlineMetric(BuildContext context, String label, String value) {
    final colors = context.shelfdColors;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          value,
          style: AppTypography.mono(
            fontSize: 14,
            fontWeight: FontWeight.w700,
            color: colors.ink,
          ),
        ),
        const SizedBox(width: AppTokens.space4),
        Text(
          label,
          style: AppTypography.captionSans(
            fontSize: 12,
            color: colors.mutedInk,
          ),
        ),
      ],
    );
  }

  Widget _buildScanningState(BuildContext context) {
    final colors = context.shelfdColors;
    return ShelfdSectionCard(
      padding: const EdgeInsets.symmetric(
        vertical: AppTokens.space48,
        horizontal: AppTokens.space24,
      ),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CircularProgressIndicator(
              strokeWidth: 2.5,
              color: colors.accentPrimary,
            ),
            const SizedBox(height: AppTokens.space16),
            Text(
              'Scanning library for duplicate books...',
              style: AppTypography.titleSerif(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space4),
            Text(
              'Checking ISBNs, author attributions, and title variations.',
              style: AppTypography.bodySans(
                fontSize: 13,
                color: colors.secondaryInk,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCleanState(BuildContext context) {
    return ShelfdEmptyState(
      icon: Icons.check_circle_outline_rounded,
      title: 'No Duplicate Books Found',
      message:
          'All books in your library have distinct identifiers and unique titles.',
      positive: true,
      action: ShelfdSecondaryButton(
        label: 'Re-scan Library',
        icon: Icons.sync_rounded,
        onPressed: () =>
            ref.read(duplicateBooksProvider.notifier).scanForDuplicates(),
      ),
    );
  }

  Widget _buildInitialScanPrompt(BuildContext context) {
    return ShelfdEmptyState(
      icon: Icons.search_rounded,
      title: 'Duplicate Detection Ready',
      message: 'Scan your collection to discover and merge duplicate books.',
      action: PrimaryButton(
        label: 'Scan for Duplicates',
        icon: Icons.play_arrow_rounded,
        onPressed: () =>
            ref.read(duplicateBooksProvider.notifier).scanForDuplicates(),
      ),
    );
  }

  Widget _buildDuplicateGroupsList(
    BuildContext context,
    DuplicateBooksState state,
  ) {
    final colors = context.shelfdColors;
    final groups = state.result!.groups;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          alignment: WrapAlignment.spaceBetween,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: AppTokens.space12,
          runSpacing: AppTokens.space4,
          children: [
            Text(
              'Detected Duplicate Sets (${groups.length})',
              style: AppTypography.titleSerif(
                fontSize: 17,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            Text(
              'Select the primary copy to keep before merging',
              style: AppTypography.captionSans(color: colors.mutedInk),
            ),
          ],
        ),
        const SizedBox(height: AppTokens.space12),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: groups.length,
          separatorBuilder: (_, _) => const SizedBox(height: AppTokens.space16),
          itemBuilder: (context, index) {
            final group = groups[index];
            final selectedPrimaryId =
                state.selectedPrimaryBooks[group.id] ??
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

    return ShelfdSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            alignment: WrapAlignment.spaceBetween,
            crossAxisAlignment: WrapCrossAlignment.center,
            spacing: AppTokens.space12,
            runSpacing: AppTokens.space12,
            children: [
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.auto_awesome_rounded,
                    size: 16,
                    color: confidenceColor,
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Flexible(
                    child: Text(
                      group.matchReason,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.titleSerif(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
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
              Wrap(
                spacing: AppTokens.space8,
                runSpacing: AppTokens.space8,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  ShelfdSecondaryButton(
                    label: 'Options',
                    icon: Icons.tune_rounded,
                    compact: true,
                    onPressed: isMerging
                        ? null
                        : () => _showMergeOptionsDialog(
                              context,
                              group,
                              selectedPrimaryId,
                            ),
                  ),
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
            ],
          ),
          const SizedBox(height: AppTokens.space16),
          Wrap(
            spacing: AppTokens.space12,
            runSpacing: AppTokens.space12,
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

    return Material(
      color: isPrimary ? colors.containerLow : colors.surface,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onSelect,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          width: 310,
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            border: Border.all(
              color: isPrimary ? colors.accentPrimary : colors.border,
              width: isPrimary ? 1.5 : 1,
            ),
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SizedBox(
                width: 50,
                height: 74,
                child: TactileBookCover(
                  coverUrl: book.coverUrl,
                  title: book.title,
                  showFallbackTitle: false,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  showShadow: false,
                ),
              ),
              const SizedBox(width: AppTokens.space12),
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
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: colors.accentPrimary,
                              borderRadius: BorderRadius.circular(
                                AppTokens.radiusPill,
                              ),
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
                    const SizedBox(height: 2),
                    Text(
                      book.authorDisplay,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        color: colors.secondaryInk,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 6),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: [
                        _buildSmallBadge(
                          context,
                          _formatFileSize(book.fileSizeBytes),
                        ),
                        if (book.publishedDate != null &&
                            book.publishedDate!.isNotEmpty)
                          _buildSmallBadge(context, book.publishedDate!),
                        if (book.bookmarks.isNotEmpty)
                          _buildSmallBadge(
                            context,
                            '${book.bookmarks.length}',
                            icon: Icons.bookmark_outline_rounded,
                          ),
                        if (book.highlights.isNotEmpty)
                          _buildSmallBadge(
                            context,
                            '${book.highlights.length}',
                            icon: Icons.edit_note_rounded,
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSmallBadge(
    BuildContext context,
    String text, {
    IconData? icon,
  }) {
    final colors = context.shelfdColors;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: colors.container,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 11, color: colors.secondaryInk),
            const SizedBox(width: 3),
          ],
          Text(
            text,
            style: AppTypography.mono(
              fontSize: 10,
              fontWeight: FontWeight.w500,
              color: colors.secondaryInk,
            ),
          ),
        ],
      ),
    );
  }
}
