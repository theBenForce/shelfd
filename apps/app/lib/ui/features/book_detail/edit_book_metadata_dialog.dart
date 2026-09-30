import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import '../../core/responsive.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

/// Shows the book metadata editor modal (dialog on desktop/tablet, bottom sheet on mobile).
Future<Book?> showEditBookMetadataModal(
  BuildContext context,
  Book book,
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
        child: EditBookMetadataContent(book: book, isBottomSheet: true),
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
        constraints: const BoxConstraints(maxWidth: 880, maxHeight: 820),
        child: EditBookMetadataContent(book: book, isBottomSheet: false),
      ),
    ),
  );
}

class EditBookMetadataContent extends ConsumerStatefulWidget {
  final Book book;
  final bool isBottomSheet;

  const EditBookMetadataContent({
    super.key,
    required this.book,
    required this.isBottomSheet,
  });

  @override
  ConsumerState<EditBookMetadataContent> createState() => _EditBookMetadataContentState();
}

class _EditBookMetadataContentState extends ConsumerState<EditBookMetadataContent> {
  final _formKey = GlobalKey<FormState>();

  late final TextEditingController _titleController;
  late final TextEditingController _authorController;
  late final TextEditingController _seriesController;
  late final TextEditingController _sequenceController;
  late final TextEditingController _genresController;
  late final TextEditingController _topicsController;
  late final TextEditingController _publisherController;
  late final TextEditingController _languageController;
  late final TextEditingController _descriptionController;

  bool _isSaving = false;
  bool _isUploadingCover = false;
  int _coverVersion = 0;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    final b = widget.book;
    _titleController = TextEditingController(text: b.title);
    _authorController = TextEditingController(
      text: b.authors.map((a) => a.name).join(', '),
    );
    _seriesController = TextEditingController(
      text: b.series != null ? b.series!.name : '',
    );
    _sequenceController = TextEditingController(
      text: b.seriesSequence != null
          ? (b.seriesSequence!.truncateToDouble() == b.seriesSequence!
              ? b.seriesSequence!.toInt().toString()
              : b.seriesSequence!.toString())
          : '',
    );
    _genresController = TextEditingController(
      text: b.genres.map((g) => g.name).join(', '),
    );
    _topicsController = TextEditingController(
      text: b.topics.map((t) => t.name).join(', '),
    );
    _publisherController = TextEditingController(text: b.publisher ?? '');
    _languageController = TextEditingController(text: b.language ?? '');
    _descriptionController = TextEditingController(text: b.synopsis);

    _titleController.addListener(_onFieldChanged);
    _authorController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() {
    setState(() {});
  }

  @override
  void dispose() {
    _titleController.removeListener(_onFieldChanged);
    _authorController.removeListener(_onFieldChanged);
    _titleController.dispose();
    _authorController.dispose();
    _seriesController.dispose();
    _sequenceController.dispose();
    _genresController.dispose();
    _topicsController.dispose();
    _publisherController.dispose();
    _languageController.dispose();
    _descriptionController.dispose();
    super.dispose();
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

      setState(() {
        _isUploadingCover = true;
        _errorMessage = null;
      });

      final notifier = ref.read(bookDetailProvider(widget.book.id).notifier);
      await notifier.uploadCover(
        filename: file.name,
        bytes: bytes,
      );

      if (mounted) {
        setState(() {
          _isUploadingCover = false;
          _coverVersion++;
        });

        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Cover image updated successfully!'),
            backgroundColor: AppTokens.charcoalInk,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isUploadingCover = false;
          _errorMessage = 'Failed to upload cover: $e';
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to upload cover: $e'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  Future<void> _saveMetadata() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }

    setState(() {
      _isSaving = true;
      _errorMessage = null;
    });

    try {
      final rawAuthors = _authorController.text
          .split(',')
          .map((a) => a.trim())
          .where((a) => a.isNotEmpty)
          .toList();

      final rawGenres = _genresController.text
          .split(',')
          .map((g) => g.trim())
          .where((g) => g.isNotEmpty)
          .toList();

      final rawTopics = _topicsController.text
          .split(',')
          .map((t) => t.trim())
          .where((t) => t.isNotEmpty)
          .toList();

      double? seqNum;
      if (_sequenceController.text.trim().isNotEmpty) {
        seqNum = double.tryParse(_sequenceController.text.trim());
      }

      final notifier = ref.read(bookDetailProvider(widget.book.id).notifier);
      final updatedBook = await notifier.updateMetadata(
        title: _titleController.text.trim(),
        authors: rawAuthors.isNotEmpty ? rawAuthors : ['Unknown'],
        series: _seriesController.text.trim().isNotEmpty ? _seriesController.text.trim() : null,
        sequenceNumber: seqNum,
        description: _descriptionController.text.trim().isNotEmpty ? _descriptionController.text.trim() : null,
        publisher: _publisherController.text.trim().isNotEmpty ? _publisherController.text.trim() : null,
        language: _languageController.text.trim().isNotEmpty ? _languageController.text.trim() : null,
        genres: rawGenres,
        topics: rawTopics,
      );

      if (mounted) {
        Navigator.of(context).pop(updatedBook);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Book metadata and EPUB file saved successfully!'),
            backgroundColor: AppTokens.charcoalInk,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSaving = false;
          _errorMessage = e.toString();
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isMobile = widget.isBottomSheet || Responsive.isMobile(context);
    final book = widget.book;
    final coverUrl = book.coverUrl != null
        ? '${book.coverUrl}${book.coverUrl!.contains('?') ? '&' : '?'}v=$_coverVersion'
        : null;

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
                  'Edit Book Metadata',
                  style: AppTypography.titleSerif(fontSize: 18, fontWeight: FontWeight.w600),
                ),
                Text(
                  'Changes update database records and are written in-place to the EPUB file',
                  style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close_rounded, size: 20),
            onPressed: _isSaving ? null : () => Navigator.of(context).pop(null),
            tooltip: 'Cancel',
          ),
        ],
      ),
    );

    final leftSummaryColumn = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Cover card with change button
        Tooltip(
          message: 'Click to upload replacement cover',
          child: InkWell(
            onTap: _isUploadingCover ? null : _pickAndUploadCover,
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            child: Stack(
              alignment: Alignment.center,
              children: [
                Container(
                  height: isMobile ? 180 : 260,
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
                          errorBuilder: (ctx, err, stack) => _CoverPlaceholder(title: _titleController.text),
                        )
                      : _CoverPlaceholder(title: _titleController.text),
                ),
                if (_isUploadingCover)
                  Container(
                    height: isMobile ? 180 : 260,
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
                            'Change Cover',
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
                      book.title,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppTokens.charcoalInk,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                '${book.spine.length} Chapters • ${book.genres.map((g) => g.name).join(', ')}',
                style: const TextStyle(
                  fontFamily: 'sans-serif',
                  fontSize: 11,
                  color: AppTokens.mutedCopy,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
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
          const _FieldLabel(label: 'Title', isRequired: true),
          TextFormField(
            key: const Key('edit_book_title_field'),
            controller: _titleController,
            decoration: _inputDecoration('e.g. Clean Architecture'),
            validator: (val) {
              if (val == null || val.trim().isEmpty) {
                return 'Title is required';
              }
              return null;
            },
          ),
          const SizedBox(height: AppTokens.space16),

          // Authors (comma-separated)
          const _FieldLabel(
            label: 'Author(s)',
            isRequired: true,
            hint: 'Separate multiple authors with commas',
          ),
          TextFormField(
            key: const Key('edit_book_authors_field'),
            controller: _authorController,
            decoration: _inputDecoration('e.g. Robert C. Martin, Martin Fowler'),
            validator: (val) {
              if (val == null || val.trim().isEmpty) {
                return 'At least one author is required';
              }
              return null;
            },
          ),
          const SizedBox(height: AppTokens.space16),

          // Series + Index row
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 3,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Series'),
                    TextFormField(
                      key: const Key('edit_book_series_field'),
                      controller: _seriesController,
                      decoration: _inputDecoration('e.g. Robert C. Martin Series'),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                flex: 1,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Book #'),
                    TextFormField(
                      key: const Key('edit_book_sequence_field'),
                      controller: _sequenceController,
                      keyboardType: const TextInputType.numberWithOptions(decimal: true),
                      decoration: _inputDecoration('1.0'),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Genres + Topics row
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Genres', hint: 'Comma separated'),
                    TextFormField(
                      key: const Key('edit_book_genres_field'),
                      controller: _genresController,
                      decoration: _inputDecoration('e.g. Software, Engineering'),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Topics', hint: 'Comma separated'),
                    TextFormField(
                      key: const Key('edit_book_topics_field'),
                      controller: _topicsController,
                      decoration: _inputDecoration('e.g. Architecture, Design Patterns'),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Publisher + Language row
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 2,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Publisher'),
                    TextFormField(
                      key: const Key('edit_book_publisher_field'),
                      controller: _publisherController,
                      decoration: _inputDecoration('e.g. Prentice Hall'),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                flex: 1,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _FieldLabel(label: 'Language'),
                    TextFormField(
                      key: const Key('edit_book_language_field'),
                      controller: _languageController,
                      decoration: _inputDecoration('en'),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Description
          const _FieldLabel(label: 'Description / Synopsis'),
          TextFormField(
            key: const Key('edit_book_description_field'),
            controller: _descriptionController,
            maxLines: 4,
            decoration: _inputDecoration('Book overview or synopsis...'),
          ),
        ],
      ),
    );

    final footer = Container(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space24, vertical: AppTokens.space16),
      decoration: const BoxDecoration(
        color: AppTokens.boneSurface,
        border: Border(top: BorderSide(color: AppTokens.crispBorder)),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          OutlinedButton(
            style: OutlinedButton.styleFrom(
              foregroundColor: AppTokens.charcoalInk,
              side: const BorderSide(color: AppTokens.crispBorder),
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusMd)),
            ),
            onPressed: _isSaving ? null : () => Navigator.of(context).pop(null),
            child: const Text('Cancel'),
          ),
          const SizedBox(width: AppTokens.space12),
          FilledButton.icon(
            key: const Key('save_book_metadata_button'),
            style: FilledButton.styleFrom(
              backgroundColor: AppTokens.charcoalInk,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusMd)),
            ),
            icon: _isSaving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(Colors.white)),
                  )
                : const Icon(Icons.check_rounded, size: 18),
            label: Text(_isSaving ? 'Saving & Syncing...' : 'Save & Sync EPUB'),
            onPressed: _isSaving ? null : _saveMetadata,
          ),
        ],
      ),
    );

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        header,
        const Divider(height: 1, color: AppTokens.crispBorder),
        if (_errorMessage != null)
          Container(
            padding: const EdgeInsets.all(AppTokens.space12),
            margin: const EdgeInsets.symmetric(horizontal: AppTokens.space24, vertical: AppTokens.space8),
            decoration: BoxDecoration(
              color: Colors.red.shade50,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: Colors.red.shade200),
            ),
            child: Text(
              _errorMessage!,
              style: TextStyle(color: Colors.red.shade900, fontSize: 13),
            ),
          ),
        Flexible(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppTokens.space24),
            child: isMobile
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      leftSummaryColumn,
                      const SizedBox(height: AppTokens.space20),
                      const Divider(height: 1, color: AppTokens.crispBorder),
                      const SizedBox(height: AppTokens.space20),
                      formFields,
                    ],
                  )
                : Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(width: 280, child: leftSummaryColumn),
                      const SizedBox(width: AppTokens.space32),
                      Expanded(child: formFields),
                    ],
                  ),
          ),
        ),
        footer,
      ],
    );
  }

  InputDecoration _inputDecoration(String hint) {
    return InputDecoration(
      hintText: hint,
      hintStyle: const TextStyle(color: AppTokens.mutedCopy, fontSize: 13),
      filled: true,
      fillColor: AppTokens.boneSurface,
      contentPadding: const EdgeInsets.symmetric(horizontal: AppTokens.space12, vertical: AppTokens.space12),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: const BorderSide(color: AppTokens.crispBorder),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: const BorderSide(color: AppTokens.crispBorder),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: const BorderSide(color: AppTokens.charcoalInk, width: 1.5),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: const BorderSide(color: Colors.red),
      ),
    );
  }
}

class _FieldLabel extends StatelessWidget {
  final String label;
  final bool isRequired;
  final String? hint;

  const _FieldLabel({
    required this.label,
    this.isRequired = false,
    this.hint,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: 4,
        children: [
          Text(
            label,
            style: AppTypography.bodySans(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: AppTokens.charcoalInk,
            ),
          ),
          if (isRequired)
            const Text(
              '*',
              style: TextStyle(color: Colors.red, fontWeight: FontWeight.bold),
            ),
          if (hint != null)
            Text(
              '($hint)',
              style: AppTypography.bodySans(fontSize: 11, color: AppTokens.mutedCopy),
            ),
        ],
      ),
    );
  }
}

class _CoverPlaceholder extends StatelessWidget {
  final String title;

  const _CoverPlaceholder({required this.title});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppTokens.space16),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.menu_book_rounded, size: 48, color: AppTokens.mutedCopy),
            const SizedBox(height: AppTokens.space8),
            Text(
              title.isNotEmpty ? title : 'No Cover Available',
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
            ),
          ],
        ),
      ),
    );
  }
}
