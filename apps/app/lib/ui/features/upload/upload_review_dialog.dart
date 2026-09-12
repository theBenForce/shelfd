import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import '../../../data/models/upload_job.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

/// Shows the metadata review dialog or bottom sheet depending on screen size.
Future<Book?> showUploadReviewModal(
  BuildContext context,
  StagedUploadJob job,
) async {
  final isMobile = Responsive.isMobile(context);

  if (isMobile) {
    return showModalBottomSheet<Book>(
      context: context,
      isScrollControlled: true,
      backgroundColor: AppTokens.boneBackground,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(ctx).viewInsets.bottom),
        child: UploadReviewContent(job: job, isBottomSheet: true),
      ),
    );
  }

  return showDialog<Book>(
    context: context,
    barrierDismissible: false,
    builder: (ctx) => Dialog(
      backgroundColor: AppTokens.boneBackground,
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: const BorderSide(color: AppTokens.crispBorder),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 880, maxHeight: 780),
        child: UploadReviewContent(job: job, isBottomSheet: false),
      ),
    ),
  );
}

class UploadReviewContent extends ConsumerStatefulWidget {
  final StagedUploadJob job;
  final bool isBottomSheet;

  const UploadReviewContent({
    super.key,
    required this.job,
    required this.isBottomSheet,
  });

  @override
  ConsumerState<UploadReviewContent> createState() => _UploadReviewContentState();
}

class _UploadReviewContentState extends ConsumerState<UploadReviewContent> {
  final _formKey = GlobalKey<FormState>();

  late final TextEditingController _titleController;
  late final TextEditingController _authorController;
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
      text: m.authors.isNotEmpty && m.authors.first != 'Unknown' ? m.authors.join(', ') : '',
    );
    _seriesController = TextEditingController(text: m.series ?? '');
    _sequenceController = TextEditingController(
      text: m.sequenceNumber != null ? m.sequenceNumber.toString() : '',
    );
    _genresController = TextEditingController(text: m.genres.join(', '));
    _publisherController = TextEditingController(text: m.publisher ?? '');
    _languageController = TextEditingController(text: m.language ?? '');
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
    _publisherController.dispose();
    _languageController.dispose();
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

  bool get _hasMissingAuthorWarning {
    final authorText = _authorController.text.trim();
    return authorText.isEmpty || authorText.toLowerCase() == 'unknown';
  }

  Future<void> _discard() async {
    setState(() {
      _isDiscarding = true;
      _errorMessage = null;
    });

    try {
      await ref.read(bookRepositoryProvider).deleteUploadJob(widget.job.jobId);
      if (mounted) {
        Navigator.of(context).pop(null);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isDiscarding = false;
          _errorMessage = 'Failed to discard: $e';
        });
      }
    }
  }

  Future<void> _commit() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }

    setState(() {
      _isSaving = true;
      _errorMessage = null;
    });

    try {
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
        publisher: _publisherController.text.trim().isNotEmpty ? _publisherController.text.trim() : null,
        language: _languageController.text.trim().isNotEmpty ? _languageController.text.trim() : null,
        description: _descriptionController.text.trim().isNotEmpty ? _descriptionController.text.trim() : null,
      );

      final book = await ref.read(bookRepositoryProvider).commitUpload(widget.job.jobId, update);
      ref.read(libraryProvider.notifier).loadLibrary();

      if (mounted) {
        Navigator.of(context).pop(book);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSaving = false;
          _errorMessage = 'Failed to save: $e';
        });
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
          const SnackBar(
            content: Text('Cover image updated successfully'),
            backgroundColor: AppTokens.charcoalInk,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isUploadingCover = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to upload cover: $e'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isMobile = widget.isBottomSheet || Responsive.isMobile(context);
    final bookRepo = ref.watch(bookRepositoryProvider);
    final coverVersion = ref.watch(uploadProvider.select((s) => s.coverVersions[widget.job.jobId]));
    final hasCover = _hasCover || widget.job.hasCover;
    final coverUrl = hasCover ? bookRepo.getUploadJobCoverUrl(widget.job.jobId, version: coverVersion) : null;

    final header = Padding(
      padding: const EdgeInsets.fromLTRB(
        AppTokens.space24,
        AppTokens.space20,
        AppTokens.space24,
        AppTokens.space12,
      ),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: const Icon(Icons.edit_note_rounded, color: AppTokens.charcoalInk, size: 20),
          ),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  'Review Staged EPUB',
                  style: AppTypography.titleSerif(fontSize: 18, fontWeight: FontWeight.w600),
                ),
                Text(
                  'Verify and adjust metadata before adding to /library',
                  style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close_rounded, size: 20),
            onPressed: _isSaving || _isDiscarding ? null : () => Navigator.of(context).pop(null),
            tooltip: 'Cancel',
          ),
        ],
      ),
    );

    final leftSummaryColumn = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Cover card
        Tooltip(
          message: 'Click to upload replacement cover',
          child: InkWell(
            onTap: _isUploadingCover ? null : _pickAndUploadCover,
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            child: Stack(
              alignment: Alignment.center,
              children: [
                Container(
                  height: isMobile ? 160 : 260,
                  width: double.infinity,
                  decoration: BoxDecoration(
                    color: AppTokens.boneContainer,
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    border: Border.all(color: AppTokens.crispBorder),
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: coverUrl != null
                      ? Image.network(
                          coverUrl,
                          fit: BoxFit.contain,
                          errorBuilder: (ctx, err, stack) => _CoverPlaceholder(title: widget.job.metadata.title),
                        )
                      : _CoverPlaceholder(title: widget.job.metadata.title),
                ),
                if (_isUploadingCover)
                  Container(
                    height: isMobile ? 160 : 260,
                    width: double.infinity,
                    decoration: BoxDecoration(
                      color: Colors.black45,
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    ),
                    child: const Center(
                      child: SizedBox(
                        width: 24,
                        height: 24,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                        ),
                      ),
                    ),
                  )
                else
                  Positioned(
                    bottom: 8,
                    right: 8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: AppTokens.charcoalInk.withValues(alpha: 0.8),
                        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                      ),
                      child: const Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.camera_alt_outlined, size: 14, color: Colors.white),
                          SizedBox(width: 4),
                          Text(
                            'Change',
                            style: TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w500),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
        const SizedBox(height: AppTokens.space12),

        // File info card
        Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            color: AppTokens.boneSurface,
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: AppTokens.crispBorder),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.insert_drive_file_outlined, size: 16, color: AppTokens.mutedCopy),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Text(
                      widget.job.filename,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppTokens.charcoalInk,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  const StatusBadge(
                    label: 'Staged',
                    backgroundColor: AppTokens.boneContainer,
                    textColor: AppTokens.mutedCopy,
                  ),
                ],
              ),
            ],
          ),
        ),
        const SizedBox(height: AppTokens.space12),

        // Amber warning if author missing
        if (_hasMissingAuthorWarning)
          Container(
            padding: const EdgeInsets.all(AppTokens.space12),
            decoration: BoxDecoration(
              color: const Color(0xFFFFF4E6),
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: const Color(0xFFFFD8A8)),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(Icons.warning_amber_rounded, size: 18, color: Color(0xFFD9480F)),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Missing Author',
                        style: AppTypography.bodySans(
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                          color: const Color(0xFFB13600),
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'EPUB metadata did not specify an author. Please provide one to ensure proper folder structure.',
                        style: AppTypography.bodySans(
                          fontSize: 11,
                          color: const Color(0xFF8F2A00),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

        const SizedBox(height: AppTokens.space12),

        // Live destination preview box
        Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            color: AppTokens.charcoalInk,
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.folder_outlined, size: 14, color: Color(0xFFADB5BD)),
                  const SizedBox(width: 6),
                  Text(
                    'DESTINATION PATH',
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 0.8,
                      color: Colors.grey.shade400,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              SelectableText(
                _currentDestinationPreview,
                style: const TextStyle(
                  fontFamily: 'monospace',
                  fontSize: 11,
                  color: Color(0xFF69DB7C),
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
      ],
    );

    final formFields = Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Title
          TextFormField(
            controller: _titleController,
            decoration: const InputDecoration(
              labelText: 'Title *',
              hintText: 'Enter book title',
              border: OutlineInputBorder(),
              isDense: true,
            ),
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Title is required' : null,
          ),
          const SizedBox(height: AppTokens.space16),

          // Author
          TextFormField(
            controller: _authorController,
            decoration: const InputDecoration(
              labelText: 'Author(s) *',
              hintText: 'e.g. Ursula K. Le Guin',
              helperText: 'Separate multiple authors with commas',
              border: OutlineInputBorder(),
              isDense: true,
            ),
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Author is required' : null,
          ),
          const SizedBox(height: AppTokens.space16),

          // Series & Sequence row
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 3,
                child: TextFormField(
                  controller: _seriesController,
                  decoration: const InputDecoration(
                    labelText: 'Series',
                    hintText: 'e.g. Earthsea Cycle',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                flex: 2,
                child: TextFormField(
                  controller: _sequenceController,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(
                    labelText: 'Volume #',
                    hintText: 'e.g. 1.0',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Genres & Tags
          TextFormField(
            controller: _genresController,
            decoration: const InputDecoration(
              labelText: 'Genres / Tags',
              hintText: 'e.g. Fantasy, Classic, Magic',
              helperText: 'Comma separated',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: AppTokens.space16),

          // Publisher & Language row
          Row(
            children: [
              Expanded(
                child: TextFormField(
                  controller: _publisherController,
                  decoration: const InputDecoration(
                    labelText: 'Publisher',
                    hintText: 'e.g. Parnassus Press',
                    border: OutlineInputBorder(),
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
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Description
          TextFormField(
            controller: _descriptionController,
            maxLines: 4,
            minLines: 2,
            decoration: const InputDecoration(
              labelText: 'Description / Synopsis',
              hintText: 'Brief summary of the work',
              border: OutlineInputBorder(),
              alignLabelWithHint: true,
            ),
          ),
        ],
      ),
    );

    final actions = Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space24,
        vertical: AppTokens.space16,
      ),
      decoration: const BoxDecoration(
        color: AppTokens.boneSurface,
        border: Border(top: BorderSide(color: AppTokens.crispBorder)),
      ),
      child: Row(
        children: [
          // Discard Button
          OutlinedButton.icon(
            style: OutlinedButton.styleFrom(
              foregroundColor: const Color(0xFFC92A2A),
              side: const BorderSide(color: Color(0xFFFFC9C9)),
              minimumSize: const Size(100, AppTokens.minTouchTarget),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              ),
            ),
            onPressed: _isSaving || _isDiscarding ? null : _discard,
            icon: _isDiscarding
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Color(0xFFC92A2A)),
                  )
                : const Icon(Icons.delete_outline_rounded, size: 18),
            label: const Text('Discard'),
          ),
          const Spacer(),

          // Save & Add Button
          PrimaryButton(
            label: 'Save & Add to Library',
            icon: Icons.check_circle_outline_rounded,
            isLoading: _isSaving,
            onPressed: _isDiscarding ? null : _commit,
          ),
        ],
      ),
    );

    return Column(
      mainAxisSize: isMobile ? MainAxisSize.min : MainAxisSize.max,
      children: [
        if (widget.isBottomSheet)
          Center(
            child: Container(
              margin: const EdgeInsets.only(top: 8, bottom: 4),
              width: 36,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.grey.shade400,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
        header,
        const Divider(color: AppTokens.crispBorder, height: 1),
        if (_errorMessage != null)
          Container(
            margin: const EdgeInsets.all(AppTokens.space16),
            padding: const EdgeInsets.all(AppTokens.space12),
            decoration: BoxDecoration(
              color: const Color(0xFFFFF5F5),
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: const Color(0xFFFFC9C9)),
            ),
            child: Row(
              children: [
                const Icon(Icons.error_outline_rounded, size: 18, color: Color(0xFFC92A2A)),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    _errorMessage!,
                    style: const TextStyle(fontSize: 12, color: Color(0xFFC92A2A)),
                  ),
                ),
              ],
            ),
          ),
        Expanded(
          flex: isMobile ? 0 : 1,
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppTokens.space24),
            child: isMobile
                ? Column(
                    children: [
                      leftSummaryColumn,
                      const SizedBox(height: AppTokens.space20),
                      const Divider(color: AppTokens.crispBorder),
                      const SizedBox(height: AppTokens.space20),
                      formFields,
                    ],
                  )
                : Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(
                        width: 300,
                        child: leftSummaryColumn,
                      ),
                      const SizedBox(width: AppTokens.space24),
                      Expanded(
                        child: formFields,
                      ),
                    ],
                  ),
          ),
        ),
        actions,
      ],
    );
  }
}

class _CoverPlaceholder extends StatelessWidget {
  final String title;

  const _CoverPlaceholder({required this.title});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.auto_stories_rounded, size: 48, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            title,
            textAlign: TextAlign.center,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 13, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 4),
          const Text(
            'No cover extracted',
            style: TextStyle(fontSize: 11, color: AppTokens.mutedCopy),
          ),
        ],
      ),
    );
  }
}
