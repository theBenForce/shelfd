import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import '../../../data/models/duplicate_group.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
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
            return AlertDialog(
              backgroundColor: AppTokens.boneSurface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                side: const BorderSide(color: AppTokens.crispBorder),
              ),
              title: Row(
                children: [
                  const Icon(Icons.merge_type_rounded,
                      size: 22, color: AppTokens.charcoalInk),
                  const SizedBox(width: AppTokens.space8),
                  Text('Merge Options',
                      style: AppTypography.titleSerif(fontSize: 18)),
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
                          fontSize: 13, color: AppTokens.mutedCopy),
                    ),
                    const SizedBox(height: AppTokens.space16),
                    CheckboxListTile(
                      value: transferBookmarks,
                      title: const Text('Transfer Bookmarks',
                          style: TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w500)),
                      subtitle: const Text(
                          'Move all reading bookmarks from duplicate books to primary book',
                          style: TextStyle(
                              fontSize: 12, color: AppTokens.mutedCopy)),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: AppTokens.charcoalInk,
                      onChanged: (val) {
                        setDialogState(() => transferBookmarks = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: transferHighlights,
                      title: const Text('Transfer Highlights & Notes',
                          style: TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w500)),
                      subtitle: const Text(
                          'Retain all annotations and highlighted passages',
                          style: TextStyle(
                              fontSize: 12, color: AppTokens.mutedCopy)),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: AppTokens.charcoalInk,
                      onChanged: (val) {
                        setDialogState(() => transferHighlights = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: mergeMetadata,
                      title: const Text('Merge Missing Metadata',
                          style: TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w500)),
                      subtitle: const Text(
                          'Fill missing descriptions, publishers, dates, and taxonomy from duplicates',
                          style: TextStyle(
                              fontSize: 12, color: AppTokens.mutedCopy)),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: AppTokens.charcoalInk,
                      onChanged: (val) {
                        setDialogState(() => mergeMetadata = val ?? true);
                      },
                    ),
                    CheckboxListTile(
                      value: deleteFiles,
                      title: const Text('Delete Duplicate EPUB Files',
                          style: TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w500)),
                      subtitle: const Text(
                          'Permanently remove redundant duplicate files from disk to free storage',
                          style: TextStyle(
                              fontSize: 12, color: AppTokens.mutedCopy)),
                      contentPadding: EdgeInsets.zero,
                      controlAffinity: ListTileControlAffinity.leading,
                      activeColor: AppTokens.charcoalInk,
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
                  child: const Text('Cancel',
                      style: TextStyle(color: AppTokens.mutedCopy)),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppTokens.charcoalInk,
                    foregroundColor: Colors.white,
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

    return Scaffold(
      backgroundColor: Theme.of(context).scaffoldBackgroundColor,
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
                      message: state.successMessage!,
                      isError: false,
                      onDismiss: () => ref
                          .read(duplicateBooksProvider.notifier)
                          .clearMessages(),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  // Stats & Overview Banner
                  _buildOverviewCard(state),
                  const SizedBox(height: AppTokens.space24),

                  // Content Area
                  if (state.isScanning) ...[
                    _buildScanningState(),
                  ] else if (state.result != null &&
                      state.result!.groups.isNotEmpty) ...[
                    _buildDuplicateGroupsList(state),
                  ] else if (state.result != null &&
                      state.result!.groups.isEmpty) ...[
                    _buildCleanState(),
                  ] else ...[
                    _buildInitialScanPrompt(),
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
                ),
              ),
              const SizedBox(height: AppTokens.space4),
              Text(
                'Consolidate duplicates, clean library metadata, and optimize storage.',
                style: AppTypography.bodySans(
                  fontSize: 14,
                  color: AppTokens.mutedCopy,
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

  Widget _buildAlertBanner({
    required String message,
    required bool isError,
    required VoidCallback onDismiss,
  }) {
    final bg = isError ? const Color(0xFFFFF5F5) : const Color(0xFFEBFBEE);
    final border = isError ? const Color(0xFFFFC9C9) : const Color(0xFFB2F2BB);
    final iconColor =
        isError ? const Color(0xFFE03131) : const Color(0xFF2F9E44);

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
              color: iconColor),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w500,
                color:
                    isError ? const Color(0xFFC92A2A) : const Color(0xFF2B8A3E),
              ),
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close_rounded, size: 16),
            color: iconColor,
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
            onPressed: onDismiss,
          ),
        ],
      ),
    );
  }

  Widget _buildOverviewCard(DuplicateBooksState state) {
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
              color: AppTokens.boneContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            ),
            child: const Icon(
              Icons.copy_all_rounded,
              color: AppTokens.charcoalInk,
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
                          fontSize: 16, fontWeight: FontWeight.w700),
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
                          ? const Color(0xFFFFF3BF)
                          : const Color(0xFFEBFBEE),
                      textColor: totalGroups > 0
                          ? const Color(0xFFD9480F)
                          : const Color(0xFF2B8A3E),
                    ),
                  ],
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  'Identifies matching ISBN identifiers, exact title/author pairs, and subtitle variations to merge redundant catalog entries.',
                  style: AppTypography.bodySans(
                      fontSize: 13, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
          if (state.result != null) ...[
            const SizedBox(width: AppTokens.space24),
            _buildStatPill('Groups', '$totalGroups'),
            const SizedBox(width: AppTokens.space12),
            _buildStatPill('Redundant Copies',
                '${totalDuplicates > 0 ? totalDuplicates - totalGroups : 0}'),
          ],
        ],
      ),
    );
  }

  Widget _buildStatPill(String label, String value) {
    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space16, vertical: AppTokens.space8),
      decoration: BoxDecoration(
        color: AppTokens.boneContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      ),
      child: Column(
        children: [
          Text(
            value,
            style: const TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: AppTokens.charcoalInk,
            ),
          ),
          Text(
            label,
            style: const TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: AppTokens.mutedCopy,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildScanningState() {
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            const CircularProgressIndicator(
              strokeWidth: 3,
              color: AppTokens.charcoalInk,
            ),
            const SizedBox(height: AppTokens.space20),
            Text(
              'Scanning library for duplicate books...',
              style: AppTypography.titleSerif(
                  fontSize: 16, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Analyzing ISBN identifiers, author attributions, and title variations.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: AppTokens.mutedCopy),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCleanState() {
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: const BoxDecoration(
                color: Color(0xFFEBFBEE),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.check_circle_outline_rounded,
                color: Color(0xFF2B8A3E),
                size: 32,
              ),
            ),
            const SizedBox(height: AppTokens.space16),
            Text(
              'No Duplicate Books Found',
              style: AppTypography.titleSerif(
                  fontSize: 18, fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'All books in your library have distinct identifiers and unique titles.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: AppTokens.mutedCopy),
            ),
            const SizedBox(height: AppTokens.space20),
            OutlinedButton.icon(
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

  Widget _buildInitialScanPrompt() {
    return BentoCard(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      child: Center(
        child: Column(
          children: [
            const Icon(Icons.search_rounded,
                size: 40, color: AppTokens.mutedCopy),
            const SizedBox(height: AppTokens.space16),
            Text(
              'Duplicate Detection Ready',
              style: AppTypography.titleSerif(
                  fontSize: 18, fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Scan your collection to discover and merge duplicate books.',
              style: AppTypography.bodySans(
                  fontSize: 13, color: AppTokens.mutedCopy),
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

  Widget _buildDuplicateGroupsList(DuplicateBooksState state) {
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
                  fontSize: 18, fontWeight: FontWeight.w700),
            ),
            Text(
              'Select primary copy before merging',
              style: AppTypography.bodySans(
                  fontSize: 12, color: AppTokens.mutedCopy),
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
    final confidencePct = (group.confidence * 100).toInt();
    final confidenceColor = group.confidence >= 0.95
        ? const Color(0xFF2B8A3E)
        : group.confidence >= 0.85
            ? const Color(0xFFD9480F)
            : const Color(0xFF1971C2);
    final confidenceBg = group.confidence >= 0.95
        ? const Color(0xFFEBFBEE)
        : group.confidence >= 0.85
            ? const Color(0xFFFFF3BF)
            : const Color(0xFFE7F5FF);

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
                  foregroundColor: AppTokens.charcoalInk,
                  side: const BorderSide(color: AppTokens.crispBorder),
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
                fontSize: 12, color: AppTokens.mutedCopy),
          ),
          const SizedBox(height: AppTokens.space16),

          // Books in group comparison
          Wrap(
            spacing: AppTokens.space16,
            runSpacing: AppTokens.space16,
            children: group.books.map((book) {
              final isPrimary = book.id == selectedPrimaryId;
              return _buildBookCandidateCard(
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
    required Book book,
    required bool isPrimary,
    required VoidCallback onSelect,
  }) {
    return InkWell(
      onTap: onSelect,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Container(
        width: 340,
        padding: const EdgeInsets.all(AppTokens.space12),
        decoration: BoxDecoration(
          color: isPrimary ? AppTokens.boneSurface : Colors.transparent,
          border: Border.all(
            color: isPrimary ? AppTokens.charcoalInk : AppTokens.crispBorder,
            width: isPrimary ? 2 : 1,
          ),
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Cover Image Thumbnail
            Container(
              width: 54,
              height: 78,
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: AppTokens.crispBorder),
              ),
              clipBehavior: Clip.antiAlias,
              child: book.coverUrl != null && book.coverUrl!.isNotEmpty
                  ? Image.network(
                      book.coverUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => const Center(
                        child: Icon(Icons.book_rounded,
                            size: 24, color: AppTokens.mutedCopy),
                      ),
                    )
                  : const Center(
                      child: Icon(Icons.book_rounded,
                          size: 24, color: AppTokens.mutedCopy),
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
                            ? AppTokens.charcoalInk
                            : AppTokens.mutedCopy,
                      ),
                      const SizedBox(width: AppTokens.space4),
                      if (isPrimary)
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: AppTokens.charcoalInk,
                            borderRadius:
                                BorderRadius.circular(AppTokens.radiusPill),
                          ),
                          child: const Text(
                            'PRIMARY',
                            style: TextStyle(
                              fontSize: 9,
                              fontWeight: FontWeight.w700,
                              color: Colors.white,
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
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2.0),
                  Text(
                    book.authorDisplay,
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      color: AppTokens.mutedCopy,
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
                      _buildSmallBadge(_formatFileSize(book.fileSizeBytes)),
                      if (book.publishedDate != null &&
                          book.publishedDate!.isNotEmpty)
                        _buildSmallBadge(book.publishedDate!),
                      if (book.bookmarks.isNotEmpty)
                        _buildSmallBadge('${book.bookmarks.length} 🔖'),
                      if (book.highlights.isNotEmpty)
                        _buildSmallBadge('${book.highlights.length} ✏️'),
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

  Widget _buildSmallBadge(String text) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: AppTokens.boneContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
      ),
      child: Text(
        text,
        style: const TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w500,
          color: AppTokens.charcoalInk,
        ),
      ),
    );
  }
}
