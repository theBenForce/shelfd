import 'package:desktop_drop/desktop_drop.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/upload_job.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
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
        allowedExtensions: ['epub'],
      );

      if (result.isEmpty) return;

      final pickedList = <PickedEpubFile>[];
      for (final f in result) {
        if (!f.name.toLowerCase().endsWith('.epub')) continue;
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
          SnackBar(content: Text('Failed to pick EPUB files: $e')),
        );
      }
    }
  }

  Future<void> _pickFolder() async {
    try {
      final dirPath = await FilePicker.getDirectoryPath();
      if (dirPath == null || dirPath.trim().isEmpty) return;

      final epubs = await scanPathForEpubs(dirPath);
      if (epubs.isEmpty) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('No .epub files found in selected folder')),
          );
        }
        return;
      }

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

    final pickedList = <PickedEpubFile>[];

    for (final item in details.files) {
      final path = item.path;
      if (path.isNotEmpty && isDirectoryPath(path)) {
        final nestedEpubs = await scanPathForEpubs(path);
        pickedList.addAll(nestedEpubs);
      } else if (item.name.toLowerCase().endsWith('.epub')) {
        pickedList.add(PickedEpubFile(
          name: item.name,
          path: item.path.isNotEmpty ? item.path : null,
          readBytes: () => item.readAsBytes(),
        ));
      }
    }

    if (pickedList.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('No valid .epub files detected in dropped items')),
        );
      }
      return;
    }

    if (mounted) {
      await ref.read(uploadProvider.notifier).uploadEpubFiles(pickedList);
    }
  }

  @override
  Widget build(BuildContext context) {
    final uploadState = ref.watch(uploadProvider);
    final isDesktop = MediaQuery.sizeOf(context).width >= 800;

    return Scaffold(
      backgroundColor: AppTokens.boneBackground,
      appBar: ShelfdTopBar(
        title: 'Uploads & Ingestion',
        subtitle: 'Ingest EPUB files or folders with nested structures',
        actions: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'Auto-add',
                style: AppTypography.bodySans(fontSize: 12, fontWeight: FontWeight.w500),
              ),
              const SizedBox(width: AppTokens.space4),
              Switch(
                value: uploadState.autoCommit,
                activeTrackColor: AppTokens.charcoalInk,
                onChanged: (val) => ref.read(uploadProvider.notifier).toggleAutoCommit(val),
              ),
              const SizedBox(width: AppTokens.space12),
            ],
          ),
        ],
      ),
      body: DropTarget(
        onDragEntered: (_) => setState(() => _isDraggingOver = true),
        onDragExited: (_) => setState(() => _isDraggingOver = false),
        onDragDone: _handleDrop,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Header Bar & Controls
                  _buildHeaderActions(context, uploadState),
                  const SizedBox(height: AppTokens.space20),

                  // Drop & Ingestion Bento Box
                  _buildDropzoneCard(context),
                  const SizedBox(height: AppTokens.space20),

                  // Multi-file batch staging progress
                  if (uploadState.isUploading) ...[
                    _buildBatchProgressCard(uploadState),
                    const SizedBox(height: AppTokens.space20),
                  ],

                  // Main Workspace: Dual Column on Desktop, Single on Mobile
                  if (isDesktop)
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // Left Column: Staged Queue
                        Expanded(
                          flex: 6,
                          child: _buildQueueSection(context, uploadState, isDesktop: true),
                        ),
                        const SizedBox(width: AppTokens.space24),
                        // Right Column: Metadata Review Inspector
                        Expanded(
                          flex: 5,
                          child: _buildInspectorSection(uploadState),
                        ),
                      ],
                    )
                  else ...[
                    _buildQueueSection(context, uploadState, isDesktop: false),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHeaderActions(BuildContext context, UploadState state) {
    final readyCount = state.readyCount;
    final totalCount = state.stagedJobs.length;

    final headerText = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Staged Upload Queue',
          style: AppTypography.titleSerif(fontSize: 22, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 2),
        Text(
          'Review and verify EPUB metadata before saving to /library',
          style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
        ),
      ],
    );

    if (totalCount == 0) {
      return headerText;
    }

    return Wrap(
      alignment: WrapAlignment.spaceBetween,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: AppTokens.space12,
      runSpacing: AppTokens.space12,
      children: [
        headerText,
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            OutlinedButton(
              style: OutlinedButton.styleFrom(
                foregroundColor: const Color(0xFFC92A2A),
                side: const BorderSide(color: Color(0xFFFFC9C9)),
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12, vertical: AppTokens.space8),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
              ),
              onPressed: () => ref.read(uploadProvider.notifier).clearAllStaged(),
              child: const Text('Clear Queue', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
            ),
            const SizedBox(width: AppTokens.space12),
            ElevatedButton.icon(
              style: ElevatedButton.styleFrom(
                backgroundColor: AppTokens.charcoalInk,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space12),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
              ),
              onPressed: readyCount > 0
                  ? () => ref.read(uploadProvider.notifier).commitAllReady()
                  : null,
              icon: const Icon(Icons.done_all_rounded, size: 16),
              label: Text(
                'Approve All Ready ($readyCount)',
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildDropzoneCard(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppTokens.space24),
      decoration: BoxDecoration(
        color: _isDraggingOver ? const Color(0xFFF1F3F5) : AppTokens.boneContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(
          color: _isDraggingOver ? AppTokens.charcoalInk : AppTokens.crispBorder,
          width: _isDraggingOver ? 2 : 1,
        ),
      ),
      child: Column(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: AppTokens.boneSurface,
              shape: BoxShape.circle,
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: const Icon(Icons.cloud_upload_outlined, size: 24, color: AppTokens.charcoalInk),
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            'Drag & drop EPUB files or nested folders here',
            style: AppTypography.titleSerif(fontSize: 16, fontWeight: FontWeight.w600),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space4),
          Text(
            'Recursively discovers nested EPUBs • Preserves Audiobookshelf folder structure',
            style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space16),
          Wrap(
            spacing: AppTokens.space12,
            runSpacing: AppTokens.space8,
            alignment: WrapAlignment.center,
            children: [
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  backgroundColor: AppTokens.boneSurface,
                  foregroundColor: AppTokens.charcoalInk,
                  side: const BorderSide(color: AppTokens.crispBorder),
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space12),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
                ),
                onPressed: _pickFiles,
                icon: const Icon(Icons.insert_drive_file_outlined, size: 16),
                label: const Text('Browse EPUB Files', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppTokens.charcoalInk,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16, vertical: AppTokens.space12),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
                ),
                onPressed: _pickFolder,
                icon: const Icon(Icons.folder_open_rounded, size: 16),
                label: const Text('Select Folder', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildBatchProgressCard(UploadState state) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppTokens.space16),
      decoration: BoxDecoration(
        color: AppTokens.boneSurface,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: AppTokens.crispBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const SizedBox(
                width: 14,
                height: 14,
                child: CircularProgressIndicator(strokeWidth: 2, color: AppTokens.charcoalInk),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Text(
                  state.currentBatchStatus ?? 'Processing files...',
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppTokens.charcoalInk,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (state.uploadProgress != null)
                Text(
                  '${(state.uploadProgress! * 100).toInt()}%',
                  style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                ),
            ],
          ),
          const SizedBox(height: AppTokens.space8),
          LinearProgressIndicator(
            value: state.uploadProgress,
            backgroundColor: AppTokens.boneContainer,
            valueColor: const AlwaysStoppedAnimation<Color>(AppTokens.charcoalInk),
            borderRadius: BorderRadius.circular(2),
            minHeight: 4,
          ),
        ],
      ),
    );
  }

  Widget _buildQueueSection(BuildContext context, UploadState state, {required bool isDesktop}) {
    final jobs = state.stagedJobs;

    if (jobs.isEmpty) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.all(AppTokens.space48),
        decoration: BoxDecoration(
          color: AppTokens.boneSurface,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(color: AppTokens.crispBorder),
        ),
        child: Column(
          children: [
            const Icon(Icons.inbox_outlined, size: 40, color: AppTokens.mutedCopy),
            const SizedBox(height: AppTokens.space12),
            Text(
              'No Pending Uploads',
              style: AppTypography.titleSerif(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: AppTokens.space4),
            Text(
              'EPUBs staged for review will appear here.',
              style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Queue (${jobs.length})',
              style: AppTypography.titleSerif(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            const SizedBox(width: AppTokens.space12),
            _StatusPill(
              label: '${state.readyCount} Ready',
              bgColor: const Color(0xFFEBFBEE),
              textColor: const Color(0xFF2B8A3E),
              borderColor: const Color(0xFFB2F2BB),
            ),
            const SizedBox(width: AppTokens.space8),
            _StatusPill(
              label: '${state.warningsCount} Incomplete',
              bgColor: const Color(0xFFFFF9DB),
              textColor: const Color(0xFFF08C00),
              borderColor: const Color(0xFFFFE066),
            ),
          ],
        ),
        const SizedBox(height: AppTokens.space12),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: jobs.length,
          separatorBuilder: (context, index) => const SizedBox(height: AppTokens.space8),
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
              onDiscard: () => ref.read(uploadProvider.notifier).discardJob(job.jobId),
              onReview: () {
                if (isDesktop) {
                  ref.read(uploadProvider.notifier).selectJob(job.jobId);
                } else {
                  showUploadReviewModal(context, job);
                }
              },
            );
          },
        ),
      ],
    );
  }

  Widget _buildInspectorSection(UploadState state) {
    final selectedJob = state.selectedJob;
    if (selectedJob == null) {
      return Container(
        padding: const EdgeInsets.all(AppTokens.space32),
        decoration: BoxDecoration(
          color: AppTokens.boneSurface,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(color: AppTokens.crispBorder),
        ),
        child: Center(
          child: Text(
            'Select a staged book from the queue to review or edit its metadata.',
            textAlign: TextAlign.center,
            style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
          ),
        ),
      );
    }

    return _MetadataInspector(
      key: ValueKey(selectedJob.jobId),
      job: selectedJob,
    );
  }
}

class _StatusPill extends StatelessWidget {
  final String label;
  final Color bgColor;
  final Color textColor;
  final Color borderColor;

  const _StatusPill({
    required this.label,
    required this.bgColor,
    required this.textColor,
    required this.borderColor,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
        border: Border.all(color: borderColor),
      ),
      child: Text(
        label,
        style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: textColor),
      ),
    );
  }
}

class _StagedJobCard extends ConsumerWidget {
  final StagedUploadJob job;
  final bool isSelected;
  final VoidCallback onTap;
  final VoidCallback onDiscard;
  final VoidCallback onReview;

  const _StagedJobCard({
    required this.job,
    required this.isSelected,
    required this.onTap,
    required this.onDiscard,
    required this.onReview,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final bookRepo = ref.watch(bookRepositoryProvider);
    final coverUrl = job.hasCover ? bookRepo.getUploadJobCoverUrl(job.jobId) : null;
    final hasWarnings = job.warnings.isNotEmpty;

    return Material(
      color: isSelected ? const Color(0xFFF3F4F3) : AppTokens.boneSurface,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: Border.all(
              color: isSelected ? AppTokens.charcoalInk : AppTokens.crispBorder,
              width: isSelected ? 2 : 1,
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
                  color: AppTokens.boneContainer,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  border: Border.all(color: AppTokens.crispBorder),
                ),
                clipBehavior: Clip.antiAlias,
                child: coverUrl != null
                    ? Image.network(
                        coverUrl,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) => const Center(
                          child: Icon(Icons.book_outlined, size: 20, color: AppTokens.mutedCopy),
                        ),
                      )
                    : const Center(
                        child: Icon(Icons.book_outlined, size: 20, color: AppTokens.mutedCopy),
                      ),
              ),
              const SizedBox(width: AppTokens.space12),
              // Details
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      job.metadata.title,
                      style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      job.metadata.primaryAuthor,
                      style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      job.filename,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 11,
                        color: Color(0xFF888888),
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
                            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                            decoration: BoxDecoration(
                              color: const Color(0xFFFFF9DB),
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              border: Border.all(color: const Color(0xFFFFE066)),
                            ),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.warning_amber_rounded, size: 12, color: Color(0xFFF08C00)),
                                const SizedBox(width: 4),
                                Flexible(
                                  child: Text(
                                    w,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: const TextStyle(
                                      fontSize: 10,
                                      fontWeight: FontWeight.w600,
                                      color: Color(0xFFD9480F),
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
              // Actions
              Column(
                children: [
                  IconButton(
                    icon: const Icon(Icons.delete_outline_rounded, size: 18, color: AppTokens.mutedCopy),
                    tooltip: 'Discard',
                    onPressed: onDiscard,
                  ),
                ],
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
  late final TextEditingController _seriesController;
  late final TextEditingController _sequenceController;
  late final TextEditingController _genresController;
  late final TextEditingController _descriptionController;

  bool _isSaving = false;
  bool _isDiscarding = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    final m = widget.job.metadata;
    _titleController = TextEditingController(text: m.title);
    _authorController = TextEditingController(
      text: m.authors.isNotEmpty && m.authors.first != 'Unknown' ? m.authors.join(', ') : '',
    );
    _seriesController = TextEditingController(text: m.series ?? '');
    _sequenceController = TextEditingController(
      text: m.sequenceNumber != null ? m.sequenceNumber.toString() : '',
    );
    _genresController = TextEditingController(text: m.genres.join(', '));
    _descriptionController = TextEditingController(text: m.description ?? '');

    _titleController.addListener(_onTextChanged);
    _authorController.addListener(_onTextChanged);
  }

  void _onTextChanged() {
    setState(() {});
  }

  @override
  void dispose() {
    _titleController.removeListener(_onTextChanged);
    _authorController.removeListener(_onTextChanged);
    _titleController.dispose();
    _authorController.dispose();
    _seriesController.dispose();
    _sequenceController.dispose();
    _genresController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  String get _currentDestinationPreview {
    final rawAuthor = _authorController.text.trim();
    final authorSegment = StagedMetadata.sanitizePathSegment(
      rawAuthor.isNotEmpty ? rawAuthor.split(',').first.trim() : 'Unknown',
    );
    final rawTitle = _titleController.text.trim();
    final titleSegment = StagedMetadata.sanitizePathSegment(
      rawTitle.isNotEmpty ? rawTitle : 'Untitled',
    );
    return '/library/$authorSegment/$titleSegment/$titleSegment.epub';
  }

  Future<void> _commit() async {
    if (!_formKey.currentState!.validate()) return;

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

    final update = StagedMetadata(
      title: _titleController.text.trim(),
      authors: authorsList.isNotEmpty ? authorsList : ['Unknown'],
      series: _seriesController.text.trim().isNotEmpty ? _seriesController.text.trim() : null,
      sequenceNumber: seq,
      genres: genresList,
      description: _descriptionController.text.trim().isNotEmpty ? _descriptionController.text.trim() : null,
    );

    final book = await ref.read(uploadProvider.notifier).commitJob(widget.job.jobId, update);
    if (mounted) {
      setState(() => _isSaving = false);
      if (book != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Added "${book.title}" to library'),
            backgroundColor: AppTokens.charcoalInk,
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
    final bookRepo = ref.watch(bookRepositoryProvider);
    final coverUrl = widget.job.hasCover ? bookRepo.getUploadJobCoverUrl(widget.job.jobId) : null;

    return Container(
      padding: const EdgeInsets.all(AppTokens.space24),
      decoration: BoxDecoration(
        color: AppTokens.boneSurface,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: AppTokens.crispBorder),
      ),
      child: Form(
        key: _formKey,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.edit_note_rounded, size: 20, color: AppTokens.charcoalInk),
                const SizedBox(width: AppTokens.space8),
                Text(
                  'Metadata Inspector',
                  style: AppTypography.titleSerif(fontSize: 16, fontWeight: FontWeight.w600),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space16),

            // Cover & Path Preview Row
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 64,
                  height: 94,
                  decoration: BoxDecoration(
                    color: AppTokens.boneContainer,
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    border: Border.all(color: AppTokens.crispBorder),
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: coverUrl != null
                      ? Image.network(
                          coverUrl,
                          fit: BoxFit.cover,
                          errorBuilder: (context, error, stackTrace) => const Center(
                            child: Icon(Icons.book_outlined, size: 24, color: AppTokens.mutedCopy),
                          ),
                        )
                      : const Center(
                          child: Icon(Icons.book_outlined, size: 24, color: AppTokens.mutedCopy),
                        ),
                ),
                const SizedBox(width: AppTokens.space16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Target Library Destination',
                        style: AppTypography.labelCaps(fontSize: 11),
                      ),
                      const SizedBox(height: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                        decoration: BoxDecoration(
                          color: AppTokens.boneContainer,
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          border: Border.all(color: AppTokens.crispBorder),
                        ),
                        child: Text(
                          _currentDestinationPreview,
                          style: const TextStyle(
                            fontFamily: 'monospace',
                            fontSize: 11,
                            color: AppTokens.charcoalInk,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Conforms strictly to Audiobookshelf /library hierarchy',
                        style: AppTypography.bodySans(fontSize: 11, color: AppTokens.mutedCopy),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space20),

            // Form Fields
            TextFormField(
              controller: _titleController,
              decoration: const InputDecoration(
                labelText: 'Book Title',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              validator: (v) => (v == null || v.trim().isEmpty) ? 'Title cannot be empty' : null,
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _authorController,
              decoration: const InputDecoration(
                labelText: 'Author(s) (comma-separated)',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              validator: (v) => (v == null || v.trim().isEmpty) ? 'Author cannot be empty' : null,
            ),
            const SizedBox(height: AppTokens.space12),

            Row(
              children: [
                Expanded(
                  flex: 3,
                  child: TextFormField(
                    controller: _seriesController,
                    decoration: const InputDecoration(
                      labelText: 'Series (optional)',
                      border: OutlineInputBorder(),
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
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                    keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _genresController,
              decoration: const InputDecoration(
                labelText: 'Genres (comma-separated)',
                border: OutlineInputBorder(),
                isDense: true,
              ),
            ),
            const SizedBox(height: AppTokens.space12),

            TextFormField(
              controller: _descriptionController,
              maxLines: 3,
              decoration: const InputDecoration(
                labelText: 'Description / Synopsis',
                border: OutlineInputBorder(),
                alignLabelWithHint: true,
                isDense: true,
              ),
            ),
            const SizedBox(height: AppTokens.space20),

            if (_errorMessage != null) ...[
              Text(
                _errorMessage!,
                style: const TextStyle(color: Color(0xFFC92A2A), fontSize: 12),
              ),
              const SizedBox(height: AppTokens.space12),
            ],

            // Action Buttons
            Wrap(
              alignment: WrapAlignment.end,
              spacing: AppTokens.space12,
              runSpacing: AppTokens.space8,
              children: [
                OutlinedButton(
                  style: OutlinedButton.styleFrom(
                    foregroundColor: const Color(0xFFC92A2A),
                    side: const BorderSide(color: Color(0xFFFFC9C9)),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
                  ),
                  onPressed: _isDiscarding || _isSaving ? null : _discard,
                  child: _isDiscarding
                      ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Discard'),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppTokens.charcoalInk,
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusSm)),
                  ),
                  onPressed: _isSaving || _isDiscarding ? null : _commit,
                  child: _isSaving
                      ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                      : const Text('Save & Add to Library'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
