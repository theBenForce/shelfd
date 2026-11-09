import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/book.dart';
import 'metadata_search_dialog.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

/// Shows the book metadata editor modal (dialog on desktop/tablet, bottom sheet on mobile).
Future<Book?> showEditBookMetadataModal(
  BuildContext context,
  Book book,
) async {
  final isMobile = Responsive.isMobile(context);
  final colors = ShelfdColors.of(context);

  if (isMobile) {
    return showModalBottomSheet<Book>(
      context: context,
      isScrollControlled: true,
      backgroundColor: colors.surface,
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
    builder: (ctx) {
      final dialogColors = ShelfdColors.of(ctx);
      return Dialog(
        backgroundColor: dialogColors.surface,
        surfaceTintColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
          side: BorderSide(color: dialogColors.border),
        ),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 880, maxHeight: 820),
          child: EditBookMetadataContent(book: book, isBottomSheet: false),
        ),
      );
    },
  );
}

class EditBookMetadataContent extends ConsumerStatefulWidget {
  final Book book;
  final bool isBottomSheet;

  const EditBookMetadataContent({
    super.key,
    required this.book,
    this.isBottomSheet = false,
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
        final colors = ShelfdColors.of(context);
        setState(() {
          _isUploadingCover = false;
          _coverVersion++;
        });

        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Cover image updated successfully!'),
            backgroundColor: colors.accentPrimary,
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

  Future<void> _searchAndAutoPopulate() async {
    final result = await showMetadataSearchDialog(
      context,
      initialTitle: _titleController.text.trim().isNotEmpty
          ? _titleController.text.trim()
          : widget.book.title,
      initialAuthor: _authorController.text.trim().isNotEmpty
          ? _authorController.text.trim()
          : widget.book.authors.map((a) => a.name).join(', '),
    );

    if (result == null) return;

    setState(() {
      if (result.title.isNotEmpty) {
        _titleController.text = result.title;
      }
      if (result.author.isNotEmpty) {
        _authorController.text = result.author;
      }
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
    });

    if (result.coverUrl != null && result.coverUrl!.isNotEmpty) {
      try {
        setState(() {
          _isUploadingCover = true;
          _errorMessage = null;
        });
        final repo = ref.read(bookRepositoryProvider);
        await repo.fetchBookCoverFromUrl(widget.book.id, result.coverUrl!);
        ref.invalidate(bookDetailProvider(widget.book.id));
        if (mounted) {
          setState(() {
            _isUploadingCover = false;
            _coverVersion++;
          });
        }
      } catch (e) {
        if (mounted) {
          setState(() {
            _isUploadingCover = false;
          });
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Failed to fetch cover from URL: $e'),
              backgroundColor: Colors.red.shade700,
            ),
          );
        }
      }
    }

    if (mounted) {
      final colors = ShelfdColors.of(context);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Auto-populated metadata from ${result.provider}!'),
          backgroundColor: colors.accentPrimary,
        ),
      );
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
        final colors = ShelfdColors.of(context);
        Navigator.of(context).pop(updatedBook);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Book metadata and EPUB file saved successfully!'),
            backgroundColor: colors.accentPrimary,
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
    final colors = ShelfdColors.of(context);
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
              color: colors.container,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: colors.border),
            ),
            child: Icon(Icons.edit_note_rounded, color: colors.accentPrimary, size: 20),
          ),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  'Edit Book Metadata',
                  style: AppTypography.titleSerif(
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
                Text(
                  'Changes update database records and are written in-place to the EPUB file',
                  style: AppTypography.bodySans(fontSize: 12, color: colors.secondaryInk),
                ),
              ],
            ),
          ),
          OutlinedButton.icon(
            icon: Icon(Icons.auto_fix_high, size: 16, color: colors.accentPrimary),
            label: const Text('Auto-Fill'),
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.ink,
              side: BorderSide(color: colors.border),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            ),
            onPressed: _isSaving ? null : _searchAndAutoPopulate,
          ),
          const SizedBox(width: AppTokens.space8),
          IconButton(
            icon: Icon(Icons.close_rounded, size: 20, color: colors.mutedInk),
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
                TactileBookCover(
                  coverUrl: coverUrl,
                  title: _titleController.text.isNotEmpty
                      ? _titleController.text
                      : 'No Cover Available',
                  height: isMobile ? 180 : 260,
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  showFallbackTitle: false,
                  showSpineCrease: true,
                  showShadow: true,
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
                        color: AppTokens.espressoInk.withValues(alpha: 0.85),
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
        const SizedBox(height: AppTokens.space8),
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                icon: Icon(Icons.upload_file, size: 14, color: colors.accentPrimary),
                label: const Text('Upload', style: TextStyle(fontSize: 12)),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(vertical: 8),
                ),
                onPressed: _isUploadingCover ? null : _pickAndUploadCover,
              ),
            ),
            const SizedBox(width: AppTokens.space8),
            Expanded(
              child: OutlinedButton.icon(
                icon: Icon(Icons.search, size: 14, color: colors.accentPrimary),
                label: const Text('Search', style: TextStyle(fontSize: 12)),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(vertical: 8),
                ),
                onPressed: _isSaving || _isUploadingCover ? null : _searchAndAutoPopulate,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppTokens.space12),

        // File info card
        Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            color: colors.containerLow,
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: colors.subtleBorder),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.insert_drive_file_outlined, size: 16, color: colors.mutedInk),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Text(
                      book.title,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                '${book.spine.length} Chapters • ${book.genres.map((g) => g.name).join(', ')}',
                style: AppTypography.mono(
                  fontSize: 11,
                  color: colors.mutedInk,
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
            style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
            decoration: _inputDecoration(colors, 'e.g. Clean Architecture'),
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
            style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
            decoration: _inputDecoration(colors, 'e.g. Robert C. Martin, Martin Fowler'),
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
                      style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                      decoration: _inputDecoration(colors, 'e.g. Robert C. Martin Series'),
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
                      style: AppTypography.mono(fontSize: 14, color: colors.ink),
                      keyboardType: const TextInputType.numberWithOptions(decimal: true),
                      decoration: _inputDecoration(colors, '1.0'),
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
                      style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                      decoration: _inputDecoration(colors, 'e.g. Software, Engineering'),
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
                      style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                      decoration: _inputDecoration(colors, 'e.g. Architecture, Design Patterns'),
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
                      style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                      decoration: _inputDecoration(colors, 'e.g. Prentice Hall'),
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
                      style: AppTypography.mono(fontSize: 14, color: colors.ink),
                      decoration: _inputDecoration(colors, 'en'),
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
            style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
            maxLines: 4,
            decoration: _inputDecoration(colors, 'Book overview or synopsis...'),
          ),
        ],
      ),
    );

    final footer = Container(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space24, vertical: AppTokens.space16),
      decoration: BoxDecoration(
        color: colors.containerLow,
        border: Border(top: BorderSide(color: colors.subtleBorder)),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          OutlinedButton(
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.ink,
              side: BorderSide(color: colors.border),
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
              backgroundColor: colors.accentPrimary,
              foregroundColor: colors.onAccent,
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppTokens.radiusMd)),
            ),
            icon: _isSaving
                ? SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation(colors.onAccent),
                    ),
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
        Divider(height: 1, color: colors.subtleBorder),
        if (_errorMessage != null)
          Container(
            padding: const EdgeInsets.all(AppTokens.space12),
            margin: const EdgeInsets.symmetric(horizontal: AppTokens.space24, vertical: AppTokens.space8),
            decoration: BoxDecoration(
              color: colors.errorBg,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: colors.errorBorder),
            ),
            child: Text(
              _errorMessage!,
              style: AppTypography.bodySans(color: colors.errorText, fontSize: 13),
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
                      Divider(height: 1, color: colors.subtleBorder),
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

  InputDecoration _inputDecoration(ShelfdColors colors, String hint) {
    return InputDecoration(
      hintText: hint,
      hintStyle: AppTypography.bodySans(color: colors.mutedInk, fontSize: 13),
      filled: true,
      fillColor: colors.containerLow,
      contentPadding: const EdgeInsets.symmetric(horizontal: AppTokens.space12, vertical: AppTokens.space12),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: BorderSide(color: colors.border),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: BorderSide(color: colors.border),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        borderSide: BorderSide(color: colors.errorText),
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
    final colors = ShelfdColors.of(context);
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
              color: colors.ink,
            ),
          ),
          if (isRequired)
            Text(
              '*',
              style: TextStyle(color: colors.errorText, fontWeight: FontWeight.bold),
            ),
          if (hint != null)
            Text(
              '($hint)',
              style: AppTypography.bodySans(fontSize: 11, color: colors.mutedInk),
            ),
        ],
      ),
    );
  }
}
