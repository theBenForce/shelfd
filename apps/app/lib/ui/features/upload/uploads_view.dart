import 'package:desktop_drop/desktop_drop.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/upload_job.dart';
import '../../core/primitives.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../book_detail/metadata_search_dialog.dart';
import 'directory_scanner.dart';
import 'upload_review_dialog.dart';

class UploadsView extends ConsumerStatefulWidget {
  const UploadsView({super.key});

  @override
  ConsumerState<UploadsView> createState() => _UploadsViewState();
}

class _UploadsViewState extends ConsumerState<UploadsView> {
  bool _isDraggingOver = false;

  Future<void> _pickFiles() async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['epub', 'm4b', 'mp3', 'm4a', 'flac'],
        allowMultiple: true,
      );

      if (result.isEmpty) return;

      final pickedList = <PickedEpubFile>[];
      for (final f in result) {
        if (!isSupportedBook(f.name)) continue;
        pickedList.add(PickedEpubFile(
          name: f.name,
          path: f.path,
          readBytes: () => f.readAsBytes(),
        ));
      }

      if (pickedList.isNotEmpty && mounted) {
        await ref.read(uploadProvider.notifier).uploadEpubFiles(pickedList);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to pick files: $e')),
        );
      }
    }
  }

  Future<void> _pickFolder() async {
    try {
      final epubs = await pickFolderForEpubs();
      if (epubs.isEmpty) return;

      if (mounted) {
        await ref.read(uploadProvider.notifier).uploadEpubFiles(epubs);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to scan folder: $e')),
        );
      }
    }
  }

  Future<void> _handleDrop(DropDoneDetails details) async {
    setState(() => _isDraggingOver = false);
    if (details.files.isEmpty) return;

    final droppedBooks = <PickedEpubFile>[];
    final droppedCovers = <PickedEpubFile>[];

    for (final item in details.files) {
      final path = item.path;
      if (path.isNotEmpty && isDirectoryPath(path)) {
        final nestedBooks = await scanPathForEpubs(path);
        droppedBooks.addAll(nestedBooks);
      } else if (isSupportedBook(item.name)) {
        droppedBooks.add(PickedEpubFile(
          name: item.name,
          path: item.path.isNotEmpty ? item.path : null,
          readBytes: () => item.readAsBytes(),
        ));
      } else if (isSupportedCover(item.name)) {
        droppedCovers.add(PickedEpubFile(
          name: item.name,
          path: item.path.isNotEmpty ? item.path : null,
          readBytes: () => item.readAsBytes(),
        ));
      }
    }

    final pickedList = matchLooseFilesWithCovers(droppedBooks, droppedCovers);

    if (pickedList.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'No valid ebook or audiobook files detected in dropped items',
            ),
          ),
        );
      }
      return;
    }

    if (mounted) {
      await ref.read(uploadProvider.notifier).uploadEpubFiles(pickedList);
    }
  }

  Widget _buildAutoAddControl(ShelfdColors colors, UploadState state) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space12,
        vertical: AppTokens.space4,
      ),
      decoration: BoxDecoration(
        color: colors.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
        border: Border.all(color: colors.subtleBorder),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            'Auto-add',
            style: AppTypography.bodySans(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: colors.secondaryInk,
            ),
          ),
          const SizedBox(width: AppTokens.space8),
          SizedBox(
            height: 28,
            child: Switch(
              value: state.autoCommit,
              onChanged: (val) =>
                  ref.read(uploadProvider.notifier).toggleAutoCommit(val),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final uploadState = ref.watch(uploadProvider);
    final isSplitDesktop = MediaQuery.sizeOf(context).width >= 800;
    final isDesktop = Responsive.isDesktop(context);
    final horizontalPad = Responsive.horizontalPadding(context);
    final colors = ShelfdColors.of(context);
    final hasQueue = uploadState.stagedJobs.isNotEmpty;

    return Scaffold(
      backgroundColor: colors.background,
      appBar: isDesktop
          ? null
          : ShelfdTopBar(
              title: 'Uploads & Ingestion',
              subtitle: 'Stage EPUBs, audiobooks, or folders',
              actions: [
                Padding(
                  padding: const EdgeInsets.only(right: AppTokens.space12),
                  child: _buildAutoAddControl(colors, uploadState),
                ),
              ],
            ),
      body: DropTarget(
        onDragEntered: (_) => setState(() => _isDraggingOver = true),
        onDragExited: (_) => setState(() => _isDraggingOver = false),
        onDragDone: _handleDrop,
        child: SingleChildScrollView(
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPad,
            vertical: AppTokens.space24,
          ),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: AppTokens.maxLibraryWidth,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (isDesktop) ...[
                    ShelfdPageHeader(
                      title: 'Uploads & Ingestion',
                      subtitle:
                          'Drop EPUBs, audiobooks, or folders to review metadata before saving to /library.',
                      trailing: _buildAutoAddControl(colors, uploadState),
                    ),
                    const SizedBox(height: AppTokens.space20),
                  ],

                  // Adaptive Dropzone: compact strip when queue has items, full dropzone when empty
                  _buildDropzoneCard(context, compact: hasQueue),
                  const SizedBox(height: AppTokens.space20),

                  // Multi-file batch staging progress
                  if (uploadState.isUploading) ...[
                    _buildBatchProgressCard(context, uploadState),
                    const SizedBox(height: AppTokens.space20),
                  ],

                  // Queue toolbar (only when staged items exist)
                  if (hasQueue) ...[
                    _buildQueueToolbar(context, uploadState),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  // Main Workspace: Dual Column on Desktop, Single on Mobile
                  if (isSplitDesktop && hasQueue)
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(
                          flex: 6,
                          child: _buildQueueList(
                            context,
                            uploadState,
                            isDesktop: true,
                          ),
                        ),
                        const SizedBox(width: AppTokens.space24),
                        Expanded(
                          flex: 5,
                          child: _buildInspectorSection(context, uploadState),
                        ),
                      ],
                    )
                  else
                    _buildQueueList(
                      context,
                      uploadState,
                      isDesktop: isSplitDesktop,
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildDropzoneCard(BuildContext context, {required bool compact}) {
    final colors = ShelfdColors.of(context);
    final actionButtons = Wrap(
      spacing: AppTokens.space8,
      runSpacing: AppTokens.space8,
      alignment: compact ? WrapAlignment.end : WrapAlignment.center,
      children: [
        ShelfdSecondaryButton(
          label: 'Browse Files',
          icon: Icons.insert_drive_file_outlined,
          compact: compact,
          onPressed: _pickFiles,
        ),
        FilledButton.icon(
          style: FilledButton.styleFrom(
            backgroundColor: colors.accentPrimary,
            foregroundColor: colors.onAccent,
            padding: EdgeInsets.symmetric(
              horizontal: compact ? AppTokens.space12 : AppTokens.space16,
              vertical: compact ? AppTokens.space8 : AppTokens.space12,
            ),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            ),
          ),
          onPressed: _pickFolder,
          icon: Icon(Icons.folder_open_rounded, size: compact ? 15 : 16),
          label: Text(
            'Select Folder',
            style: AppTypography.bodySans(
              fontSize: compact ? 12 : 13,
              fontWeight: FontWeight.w600,
              color: colors.onAccent,
            ),
          ),
        ),
        ShelfdSecondaryButton(
          key: const Key('uploads_browse_storefronts_button'),
          label: 'Browse Storefronts',
          icon: Icons.storefront_outlined,
          compact: compact,
          onPressed: () => context.go('/plugins/catalog'),
        ),
      ],
    );

    if (compact) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space16,
          vertical: AppTokens.space12,
        ),
        decoration: BoxDecoration(
          color: _isDraggingOver
              ? colors.accentContainer.withValues(alpha: 0.45)
              : colors.containerLow,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(
            color: _isDraggingOver ? colors.accentPrimary : colors.border,
            width: _isDraggingOver ? 2 : 1,
          ),
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
                  Icons.cloud_upload_outlined,
                  size: 20,
                  color: colors.accentPrimary,
                ),
                const SizedBox(width: AppTokens.space8),
                Flexible(
                  child: Text(
                    'Drag & drop EPUB files or nested folders here',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: colors.ink,
                    ),
                  ),
                ),
              ],
            ),
            actionButtons,
          ],
        ),
      );
    }

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space24,
        vertical: AppTokens.space32,
      ),
      decoration: BoxDecoration(
        color: _isDraggingOver
            ? colors.accentContainer.withValues(alpha: 0.45)
            : colors.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        border: Border.all(
          color: _isDraggingOver ? colors.accentPrimary : colors.border,
          width: _isDraggingOver ? 2 : 1,
        ),
      ),
      child: Column(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: colors.surface,
              shape: BoxShape.circle,
              border: Border.all(color: colors.border),
            ),
            child: Icon(
              Icons.cloud_upload_outlined,
              size: 24,
              color: colors.accentPrimary,
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            'Drag & drop EPUB files or nested folders here',
            style: AppTypography.titleSerif(
              fontSize: 17,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space4),
          Text(
            'Supports EPUB, M4B, MP3, M4A, and FLAC • Preserves Audiobookshelf folder structure',
            style: AppTypography.captionSans(color: colors.mutedInk),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space20),
          actionButtons,
        ],
      ),
    );
  }

  Widget _buildBatchProgressCard(BuildContext context, UploadState state) {
    final colors = ShelfdColors.of(context);
    return ShelfdSectionCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
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
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Text(
                  state.currentBatchStatus ?? 'Processing files...',
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (state.uploadProgress != null) ...[
                const SizedBox(width: AppTokens.space8),
                Text(
                  '${(state.uploadProgress! * 100).toInt()}%',
                  style: AppTypography.mono(
                    fontSize: 12,
                    color: colors.mutedInk,
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: AppTokens.space8),
          ClipRRect(
            borderRadius: BorderRadius.circular(2),
            child: LinearProgressIndicator(
              value: state.uploadProgress,
              backgroundColor: colors.container,
              valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
              minHeight: 4,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildQueueToolbar(BuildContext context, UploadState state) {
    final colors = ShelfdColors.of(context);
    final readyCount = state.readyCount;
    final jobsCount = state.stagedJobs.length;

    return Wrap(
      alignment: WrapAlignment.spaceBetween,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: AppTokens.space12,
      runSpacing: AppTokens.space12,
      children: [
        Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: AppTokens.space8,
          runSpacing: AppTokens.space8,
          children: [
            Text(
              'Queue ($jobsCount)',
              style: AppTypography.titleSerif(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            StatusBadge(
              label: '$readyCount Ready',
              backgroundColor: colors.isDark
                  ? colors.accentSecondaryContainer
                  : AppTokens.completedBadgeBg,
              textColor: colors.accentSecondary,
            ),
            if (state.warningsCount > 0)
              StatusBadge(
                label: '${state.warningsCount} Incomplete',
                backgroundColor: colors.warningBg,
                textColor: colors.warningText,
              ),
          ],
        ),
        Wrap(
          spacing: AppTokens.space8,
          runSpacing: AppTokens.space8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            ShelfdDangerButton(
              label: 'Clear Queue',
              compact: true,
              onPressed: () =>
                  ref.read(uploadProvider.notifier).clearAllStaged(),
            ),
            FilledButton.icon(
              style: FilledButton.styleFrom(
                backgroundColor: colors.accentPrimary,
                foregroundColor: colors.onAccent,
                padding: const EdgeInsets.symmetric(
                  horizontal: AppTokens.space16,
                  vertical: AppTokens.space12,
                ),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
              ),
              onPressed: readyCount > 0
                  ? () => ref.read(uploadProvider.notifier).commitAllReady()
                  : null,
              icon: const Icon(Icons.done_all_rounded, size: 16),
              label: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    'Approve All Ready ($readyCount)',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: colors.onAccent,
                    ),
                  ),
                  const SizedBox.shrink(child: Text('Approve All Ready')),
                ],
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildQueueList(
    BuildContext context,
    UploadState state, {
    required bool isDesktop,
  }) {
    final jobs = state.stagedJobs;

    if (jobs.isEmpty) {
      return const ShelfdEmptyState(
        icon: Icons.inbox_outlined,
        title: 'No Pending Uploads',
        message: 'EPUBs and audiobooks staged for review will appear here.',
      );
    }

    return ListView.separated(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: jobs.length,
      separatorBuilder: (_, _) => const SizedBox(height: AppTokens.space8),
      itemBuilder: (context, index) {
        final job = jobs[index];
        final isSelected = isDesktop && job.jobId == state.selectedJobId;
        return _StagedJobCard(
          job: job,
          isSelected: isSelected,
          onTap: () {
            if (isDesktop) {
              ref.read(uploadProvider.notifier).selectJob(job.jobId);
            } else {
              showUploadReviewModal(context, job);
            }
          },
          onDiscard: () =>
              ref.read(uploadProvider.notifier).discardJob(job.jobId),
        );
      },
    );
  }

  Widget _buildInspectorSection(BuildContext context, UploadState state) {
    final selectedJob = state.selectedJob;
    if (selectedJob == null) {
      return const ShelfdEmptyState(
        icon: Icons.edit_note_rounded,
        title: 'Select a Book to Review',
        message:
            'Choose a staged item from the queue to inspect or edit its metadata.',
      );
    }

    return _MetadataInspector(
      key: ValueKey(selectedJob.jobId),
      job: selectedJob,
    );
  }
}

class _StagedJobCard extends ConsumerWidget {
  final StagedUploadJob job;
  final bool isSelected;
  final VoidCallback onTap;
  final VoidCallback onDiscard;

  const _StagedJobCard({
    required this.job,
    required this.isSelected,
    required this.onTap,
    required this.onDiscard,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final bookRepo = ref.watch(bookRepositoryProvider);
    final coverVersion =
        ref.watch(uploadProvider.select((s) => s.coverVersions[job.jobId])) ??
        job.updatedAt?.millisecondsSinceEpoch;
    final coverUrl = job.hasCover
        ? bookRepo.getUploadJobCoverUrl(job.jobId, version: coverVersion)
        : null;
    final hasWarnings = job.warnings.isNotEmpty;
    final isAudio = job.metadata.bookType == 'audiobook';

    return Material(
      color: isSelected ? colors.containerLow : colors.surface,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: Border.all(
              color: isSelected ? colors.accentPrimary : colors.subtleBorder,
              width: isSelected ? 1.5 : 1,
            ),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Cover Thumbnail
              Container(
                width: 48,
                height: 70,
                decoration: BoxDecoration(
                  color: colors.container,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  border: Border.all(color: colors.subtleBorder),
                ),
                clipBehavior: Clip.antiAlias,
                child: coverUrl != null
                    ? Image.network(
                        coverUrl,
                        fit: BoxFit.cover,
                        errorBuilder: (_, _, _) => Center(
                          child: Icon(
                            isAudio
                                ? Icons.headphones_outlined
                                : Icons.book_outlined,
                            size: 20,
                            color: colors.mutedInk,
                          ),
                        ),
                      )
                    : Center(
                        child: Icon(
                          isAudio
                              ? Icons.headphones_outlined
                              : Icons.book_outlined,
                          size: 20,
                          color: colors.mutedInk,
                        ),
                      ),
              ),
              const SizedBox(width: AppTokens.space12),
              // Details
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            job.metadata.title,
                            style: AppTypography.titleSerif(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: colors.ink,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        const SizedBox(width: AppTokens.space8),
                        StatusBadge(
                          label: isAudio ? 'AUDIO' : 'EPUB',
                          backgroundColor: isAudio
                              ? colors.accentContainer
                              : colors.container,
                          textColor: isAudio
                              ? colors.accentPrimary
                              : colors.mutedInk,
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      job.metadata.primaryAuthor,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        color: colors.secondaryInk,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (isAudio &&
                        job.metadata.narrator != null &&
                        job.metadata.narrator!.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        'Narrated by ${job.metadata.narrator}',
                        style: AppTypography.bodySans(
                          fontSize: 11,
                          color: colors.mutedInk,
                          fontStyle: FontStyle.italic,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                    const SizedBox(height: 4),
                    Text(
                      job.filename,
                      style: AppTypography.mono(
                        fontSize: 11,
                        color: colors.mutedInk,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (hasWarnings) ...[
                      const SizedBox(height: 6),
                      Wrap(
                        spacing: 4,
                        runSpacing: 4,
                        children: job.warnings.map((w) {
                          return Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: colors.warningBg,
                              borderRadius: BorderRadius.circular(
                                AppTokens.radiusSm,
                              ),
                              border: Border.all(color: colors.warningBorder),
                            ),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(
                                  Icons.warning_amber_rounded,
                                  size: 12,
                                  color: colors.warningText,
                                ),
                                const SizedBox(width: 4),
                                Flexible(
                                  child: Text(
                                    w,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.bodySans(
                                      fontSize: 10,
                                      fontWeight: FontWeight.w600,
                                      color: colors.warningText,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                          );
                        }).toList(),
                      ),
                    ],
                  ],
                ),
              ),
              IconButton(
                icon: Icon(
                  Icons.delete_outline_rounded,
                  size: 18,
                  color: colors.mutedInk,
                ),
                tooltip: 'Discard',
                onPressed: onDiscard,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MetadataInspector extends ConsumerStatefulWidget {
  final StagedUploadJob job;

  const _MetadataInspector({super.key, required this.job});

  @override
  ConsumerState<_MetadataInspector> createState() => _MetadataInspectorState();
}

class _MetadataInspectorState extends ConsumerState<_MetadataInspector> {
  final _formKey = GlobalKey<FormState>();

  late final TextEditingController _titleController;
  late final TextEditingController _authorController;
  late final TextEditingController _narratorController;
  late final TextEditingController _seriesController;
  late final TextEditingController _sequenceController;
  late final TextEditingController _genresController;
  late final TextEditingController _publisherController;
  late final TextEditingController _languageController;
  late final TextEditingController _descriptionController;

  bool _isSaving = false;
  bool _isDiscarding = false;
  bool _isUploadingCover = false;
  bool _hasCover = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _hasCover = widget.job.hasCover;
    final m = widget.job.metadata;
    _titleController = TextEditingController(text: m.title);
    _authorController = TextEditingController(
      text: m.authors.isNotEmpty ? m.authors.join(', ') : '',
    );
    _narratorController = TextEditingController(text: m.narrator ?? '');
    _seriesController = TextEditingController(text: m.series ?? '');
    _sequenceController = TextEditingController(
      text: m.sequenceNumber != null ? m.sequenceNumber.toString() : '',
    );
    _genresController = TextEditingController(text: m.genres.join(', '));
    _publisherController = TextEditingController(text: m.publisher ?? '');
    _languageController = TextEditingController(text: m.language ?? '');
    _descriptionController = TextEditingController(text: m.description ?? '');

    for (final ctrl in [
      _titleController,
      _authorController,
      _narratorController,
      _seriesController,
      _sequenceController,
      _genresController,
      _publisherController,
      _languageController,
      _descriptionController,
    ]) {
      ctrl.addListener(_onTextChanged);
    }
  }

  void _onTextChanged() {
    final authorsList = _authorController.text
        .split(',')
        .map((a) => a.trim())
        .where((a) => a.isNotEmpty)
        .toList();

    final genresList = _genresController.text
        .split(',')
        .map((g) => g.trim())
        .where((g) => g.isNotEmpty)
        .toList();

    final seq = double.tryParse(_sequenceController.text.trim());

    final update = widget.job.metadata.copyWith(
      title: _titleController.text.trim().isNotEmpty
          ? _titleController.text.trim()
          : 'Untitled',
      authors: authorsList.isNotEmpty ? authorsList : ['Unknown'],
      series: _seriesController.text.trim().isNotEmpty
          ? _seriesController.text.trim()
          : null,
      sequenceNumber: seq,
      genres: genresList,
      publisher: _publisherController.text.trim().isNotEmpty
          ? _publisherController.text.trim()
          : null,
      language: _languageController.text.trim().isNotEmpty
          ? _languageController.text.trim()
          : null,
      description: _descriptionController.text.trim().isNotEmpty
          ? _descriptionController.text.trim()
          : null,
      narrator: _narratorController.text.trim().isNotEmpty
          ? _narratorController.text.trim()
          : null,
    );

    ref
        .read(uploadProvider.notifier)
        .updateStagedMetadata(widget.job.jobId, update);
    setState(() {});
  }

  @override
  void dispose() {
    for (final ctrl in [
      _titleController,
      _authorController,
      _narratorController,
      _seriesController,
      _sequenceController,
      _genresController,
      _publisherController,
      _languageController,
      _descriptionController,
    ]) {
      ctrl.removeListener(_onTextChanged);
      ctrl.dispose();
    }
    super.dispose();
  }

  String get _currentDestinationPreview {
    final rawAuthor = _authorController.text.trim();
    final cleanAuthor = rawAuthor.isNotEmpty
        ? rawAuthor
              .split(',')
              .first
              .replaceAll(RegExp(r'[/\\:*?"<>|]'), '')
              .trim()
        : 'Unknown';
    final authorSegment = StagedMetadata.sanitizePathSegment(
      cleanAuthor.isEmpty ? 'Unknown' : cleanAuthor,
    );
    final rawTitle = _titleController.text.trim();
    final cleanTitle =
        rawTitle.replaceAll(RegExp(r'[/\\:*?"<>|]'), '').trim();
    final titleSegment = StagedMetadata.sanitizePathSegment(
      cleanTitle.isEmpty ? 'Untitled' : cleanTitle,
    );
    final ext = widget.job.filename.contains('.')
        ? '.${widget.job.filename.split('.').last.toLowerCase()}'
        : (widget.job.metadata.bookType == 'audiobook' ? '.m4b' : '.epub');
    return '/library/$authorSegment/$titleSegment/$titleSegment$ext';
  }

  bool get _hasMissingAuthorWarning {
    final authorText = _authorController.text.trim();
    return authorText.isEmpty || authorText.toLowerCase() == 'unknown';
  }

  bool get _isDuplicateWarning {
    final libraryBooks = ref.read(libraryProvider).books;
    final currentTitle = _titleController.text.trim().toLowerCase();
    final rawAuthor = _authorController.text.trim().toLowerCase();
    final primaryAuthor =
        rawAuthor.isNotEmpty ? rawAuthor.split(',').first.trim() : 'unknown';

    final matchesLibrary = libraryBooks.any(
      (b) =>
          b.title.trim().toLowerCase() == currentTitle &&
          (b.authors.isEmpty
              ? primaryAuthor == 'unknown'
              : b.authors.any(
                  (a) => a.name.trim().toLowerCase() == primaryAuthor,
                )),
    );

    if (matchesLibrary) return true;

    final initialTitle = widget.job.metadata.title.trim().toLowerCase();
    final initialAuthor =
        widget.job.metadata.primaryAuthor.trim().toLowerCase();
    return widget.job.isDuplicate &&
        currentTitle == initialTitle &&
        primaryAuthor == initialAuthor;
  }

  Future<void> _searchAndAutoPopulate() async {
    final result = await showMetadataSearchDialog(
      context,
      initialTitle: _titleController.text.trim().isNotEmpty
          ? _titleController.text.trim()
          : widget.job.metadata.title,
      initialAuthor: _authorController.text.trim().isNotEmpty
          ? _authorController.text.trim()
          : widget.job.metadata.authors.join(', '),
    );

    if (result == null) return;

    if (result.title.isNotEmpty) _titleController.text = result.title;
    if (result.author.isNotEmpty) _authorController.text = result.author;
    if (result.description != null && result.description!.isNotEmpty) {
      _descriptionController.text = result.description!;
    }
    if (result.publisher != null && result.publisher!.isNotEmpty) {
      _publisherController.text = result.publisher!;
    }
    if (result.language != null && result.language!.isNotEmpty) {
      _languageController.text = result.language!;
    }
    if (result.series != null && result.series!.isNotEmpty) {
      _seriesController.text = result.series!;
    }
    if (result.seriesSequence != null && result.seriesSequence!.isNotEmpty) {
      _sequenceController.text = result.seriesSequence!;
    }
    if (result.genres.isNotEmpty) {
      _genresController.text = result.genres.join(', ');
    }

    if (result.coverUrl != null && result.coverUrl!.isNotEmpty) {
      try {
        setState(() {
          _isUploadingCover = true;
          _errorMessage = null;
        });
        final repo = ref.read(bookRepositoryProvider);
        await repo.fetchJobCoverFromUrl(widget.job.jobId, result.coverUrl!);
        if (mounted) {
          setState(() {
            _hasCover = true;
            _isUploadingCover = false;
          });
        }
      } catch (e) {
        if (mounted) {
          setState(() => _isUploadingCover = false);
          final colors = ShelfdColors.of(context);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Failed to fetch cover from URL: $e'),
              backgroundColor: colors.errorText,
            ),
          );
        }
      }
    }

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Auto-populated metadata from ${result.provider}!'),
        ),
      );
    }
  }

  Future<void> _commit() async {
    if (!_formKey.currentState!.validate()) return;

    if (_isDuplicateWarning) {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Book Already Exists'),
          content: Text(
            'A book titled "${_titleController.text.trim()}" already exists in your library. Do you want to proceed with uploading and replacing it?',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Upload Anyway'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
    }

    setState(() {
      _isSaving = true;
      _errorMessage = null;
    });

    final authorsList = _authorController.text
        .split(',')
        .map((a) => a.trim())
        .where((a) => a.isNotEmpty)
        .toList();

    final genresList = _genresController.text
        .split(',')
        .map((g) => g.trim())
        .where((g) => g.isNotEmpty)
        .toList();

    final seq = double.tryParse(_sequenceController.text.trim());

    final update = widget.job.metadata.copyWith(
      title: _titleController.text.trim(),
      authors: authorsList.isNotEmpty ? authorsList : ['Unknown'],
      series: _seriesController.text.trim().isNotEmpty
          ? _seriesController.text.trim()
          : null,
      sequenceNumber: seq,
      genres: genresList,
      publisher: _publisherController.text.trim().isNotEmpty
          ? _publisherController.text.trim()
          : null,
      language: _languageController.text.trim().isNotEmpty
          ? _languageController.text.trim()
          : null,
      description: _descriptionController.text.trim().isNotEmpty
          ? _descriptionController.text.trim()
          : null,
      narrator: _narratorController.text.trim().isNotEmpty
          ? _narratorController.text.trim()
          : null,
    );

    final book = await ref
        .read(uploadProvider.notifier)
        .commitJob(widget.job.jobId, update);
    if (mounted) {
      setState(() => _isSaving = false);
      if (book != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Added "${book.title}" to library')),
        );
      }
    }
  }

  Future<void> _pickAndUploadCover() async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['jpg', 'jpeg', 'png', 'webp'],
      );
      if (result.isEmpty) return;

      final file = result.first;
      final bytes = await file.readAsBytes();
      if (bytes.isEmpty) return;

      setState(() => _isUploadingCover = true);
      await ref.read(uploadProvider.notifier).replaceJobCover(
            jobId: widget.job.jobId,
            filename: file.name,
            bytes: bytes,
          );

      if (mounted) {
        setState(() {
          _isUploadingCover = false;
          _hasCover = true;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Cover image updated successfully')),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isUploadingCover = false);
        final colors = ShelfdColors.of(context);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to upload cover: $e'),
            backgroundColor: colors.errorText,
          ),
        );
      }
    }
  }

  Future<void> _discard() async {
    setState(() => _isDiscarding = true);
    await ref.read(uploadProvider.notifier).discardJob(widget.job.jobId);
    if (mounted) {
      setState(() => _isDiscarding = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final bookRepo = ref.watch(bookRepositoryProvider);
    final coverVersion =
        ref.watch(
          uploadProvider.select((s) => s.coverVersions[widget.job.jobId]),
        ) ??
        widget.job.updatedAt?.millisecondsSinceEpoch;
    final hasCover = _hasCover || widget.job.hasCover;
    final coverUrl = hasCover
        ? bookRepo.getUploadJobCoverUrl(
            widget.job.jobId,
            version: coverVersion,
          )
        : null;

    return ShelfdSectionCard(
      icon: Icons.edit_note_rounded,
      title: 'Metadata Inspector',
      trailing: ShelfdSecondaryButton(
        label: 'Auto-Fill',
        icon: Icons.auto_fix_high,
        compact: true,
        onPressed: _isSaving || _isDiscarding ? null : _searchAndAutoPopulate,
      ),
      child: Form(
        key: _formKey,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Cover & Destination Path Preview
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Tooltip(
                  message: 'Click to upload replacement cover',
                  child: InkWell(
                    onTap: _isUploadingCover ? null : _pickAndUploadCover,
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    child: Stack(
                      alignment: Alignment.center,
                      children: [
                        Container(
                          width: 60,
                          height: 88,
                          decoration: BoxDecoration(
                            color: colors.containerLow,
                            borderRadius: BorderRadius.circular(
                              AppTokens.radiusSm,
                            ),
                            border: Border.all(color: colors.subtleBorder),
                          ),
                          clipBehavior: Clip.antiAlias,
                          child: coverUrl != null
                              ? Image.network(
                                  coverUrl,
                                  fit: BoxFit.cover,
                                  errorBuilder: (_, _, _) => Center(
                                    child: Icon(
                                      Icons.book_outlined,
                                      size: 22,
                                      color: colors.mutedInk,
                                    ),
                                  ),
                                )
                              : Center(
                                  child: Icon(
                                    Icons.book_outlined,
                                    size: 22,
                                    color: colors.mutedInk,
                                  ),
                                ),
                        ),
                        if (_isUploadingCover)
                          Container(
                            width: 60,
                            height: 88,
                            decoration: BoxDecoration(
                              color: Colors.black45,
                              borderRadius: BorderRadius.circular(
                                AppTokens.radiusSm,
                              ),
                            ),
                            child: const Center(
                              child: SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                  valueColor: AlwaysStoppedAnimation<Color>(
                                    Colors.white,
                                  ),
                                ),
                              ),
                            ),
                          )
                        else
                          Positioned(
                            bottom: 4,
                            right: 4,
                            child: Container(
                              padding: const EdgeInsets.all(3),
                              decoration: BoxDecoration(
                                color: colors.ink.withValues(alpha: 0.78),
                                shape: BoxShape.circle,
                              ),
                              child: Icon(
                                Icons.camera_alt_outlined,
                                size: 11,
                                color: colors.surface,
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(width: AppTokens.space16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Target Library Destination',
                        style: AppTypography.labelCaps(
                          fontSize: 10,
                          color: colors.mutedInk,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space4),
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppTokens.space8,
                          vertical: 6,
                        ),
                        decoration: BoxDecoration(
                          color: colors.containerLow,
                          borderRadius: BorderRadius.circular(
                            AppTokens.radiusSm,
                          ),
                          border: Border.all(color: colors.subtleBorder),
                        ),
                        child: Text(
                          _currentDestinationPreview,
                          style: AppTypography.mono(
                            fontSize: 11,
                            color: colors.ink,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space4),
                      Text(
                        'Matches Audiobookshelf /library folder layout',
                        style: AppTypography.captionSans(
                          fontSize: 11,
                          color: colors.mutedInk,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space20),

            // Primary Metadata Fields
            TextFormField(
              controller: _titleController,
              decoration: const InputDecoration(
                labelText: 'Book Title',
                isDense: true,
              ),
              validator: (v) =>
                  (v == null || v.trim().isEmpty) ? 'Title cannot be empty' : null,
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _authorController,
              decoration: const InputDecoration(
                labelText: 'Author(s) (comma-separated)',
                isDense: true,
              ),
              validator: (v) => (v == null || v.trim().isEmpty)
                  ? 'Author cannot be empty'
                  : null,
            ),
            const SizedBox(height: AppTokens.space12),

            if (widget.job.metadata.bookType == 'audiobook') ...[
              TextFormField(
                controller: _narratorController,
                decoration: const InputDecoration(
                  labelText: 'Narrator (optional)',
                  isDense: true,
                ),
              ),
              const SizedBox(height: AppTokens.space12),
            ],

            Row(
              children: [
                Expanded(
                  flex: 3,
                  child: TextFormField(
                    controller: _seriesController,
                    decoration: const InputDecoration(
                      labelText: 'Series (optional)',
                      isDense: true,
                    ),
                  ),
                ),
                const SizedBox(width: AppTokens.space12),
                Expanded(
                  flex: 1,
                  child: TextFormField(
                    controller: _sequenceController,
                    decoration: const InputDecoration(
                      labelText: 'Vol #',
                      isDense: true,
                    ),
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _genresController,
              decoration: const InputDecoration(
                labelText: 'Genres (comma-separated)',
                isDense: true,
              ),
            ),
            const SizedBox(height: AppTokens.space12),

            Row(
              children: [
                Expanded(
                  child: TextFormField(
                    controller: _publisherController,
                    decoration: const InputDecoration(
                      labelText: 'Publisher',
                      hintText: 'e.g. Parnassus Press',
                      isDense: true,
                    ),
                  ),
                ),
                const SizedBox(width: AppTokens.space12),
                Expanded(
                  child: TextFormField(
                    controller: _languageController,
                    decoration: const InputDecoration(
                      labelText: 'Language',
                      hintText: 'e.g. en',
                      isDense: true,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _descriptionController,
              maxLines: 3,
              decoration: const InputDecoration(
                labelText: 'Description / Synopsis',
                alignLabelWithHint: true,
                isDense: true,
              ),
            ),
            const SizedBox(height: AppTokens.space16),

            if (_hasMissingAuthorWarning) ...[
              const ShelfdInlineBanner(
                title: 'Missing Author',
                message:
                    'Metadata did not specify an author. Provide one to ensure proper /library folder structure.',
                tone: ShelfdBannerTone.warning,
              ),
              const SizedBox(height: AppTokens.space12),
            ],

            if (_isDuplicateWarning) ...[
              const ShelfdInlineBanner(
                title: 'Duplicate Book Detected',
                message:
                    'A book with this title and author already exists in your library. Saving will ask before overwriting.',
                tone: ShelfdBannerTone.warning,
              ),
              const SizedBox(height: AppTokens.space12),
            ],

            if (_errorMessage != null) ...[
              ShelfdInlineBanner(
                message: _errorMessage!,
                tone: ShelfdBannerTone.error,
              ),
              const SizedBox(height: AppTokens.space12),
            ],

            const SizedBox(height: AppTokens.space4),
            Wrap(
              alignment: WrapAlignment.end,
              spacing: AppTokens.space12,
              runSpacing: AppTokens.space8,
              children: [
                ShelfdDangerButton(
                  label: 'Discard',
                  isLoading: _isDiscarding,
                  onPressed: _isDiscarding || _isSaving ? null : _discard,
                ),
                PrimaryButton(
                  label: 'Save & Add to Library',
                  isLoading: _isSaving,
                  onPressed: _isSaving || _isDiscarding ? null : _commit,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
