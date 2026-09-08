import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/book_chat.dart';
import '../../../data/models/bookmark.dart';
import '../../../data/models/highlight.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class BookDetailView extends ConsumerStatefulWidget {
  final String bookId;

  const BookDetailView({super.key, required this.bookId});

  @override
  ConsumerState<BookDetailView> createState() => _BookDetailViewState();
}

class _BookDetailViewState extends ConsumerState<BookDetailView> {
  int _selectedTabIndex = 0;
  String _annotationFilter = 'all'; // 'all', 'highlights', 'bookmarks'
  final TextEditingController _chatController = TextEditingController();
  final ScrollController _chatScrollController = ScrollController();

  @override
  void dispose() {
    _chatController.dispose();
    _chatScrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_chatScrollController.hasClients) {
        _chatScrollController.animateTo(
          _chatScrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  void _sendMessage([String? presetText]) {
    final text = presetText ?? _chatController.text;
    if (text.trim().isEmpty) return;

    if (presetText == null) {
      _chatController.clear();
    }
    ref.read(bookChatProvider(widget.bookId).notifier).sendMessage(text);
    _scrollToBottom();
  }

  void _showAddBookmarkDialog(BuildContext context, Book book) {
    final titleController = TextEditingController(
      text: 'Bookmark #${book.bookmarks.length + 1}',
    );

    showDialog(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        backgroundColor: AppTokens.boneBackground,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          side: const BorderSide(color: AppTokens.crispBorder),
        ),
        title: Text(
          'Add Bookmark',
          style: AppTypography.titleSerif(fontSize: 18),
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Save your current place in ${book.title}.',
              style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
            ),
            const SizedBox(height: AppTokens.space16),
            TextField(
              controller: titleController,
              autofocus: true,
              decoration: const InputDecoration(
                labelText: 'Bookmark Title',
                border: OutlineInputBorder(),
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: AppTokens.charcoalInk,
            ),
            onPressed: () {
              final title = titleController.text.trim();
              if (title.isNotEmpty) {
                ref.read(bookDetailProvider(widget.bookId).notifier).addBookmark(
                      title: title,
                      progress: book.readingProgress,
                    );
              }
              Navigator.of(dialogCtx).pop();
            },
            child: const Text('Save Bookmark'),
          ),
        ],
      ),
    );
  }

  void _showAddHighlightDialog(BuildContext context, Book book) {
    final textController = TextEditingController();
    final noteController = TextEditingController();
    String selectedColor = 'yellow';

    showDialog(
      context: context,
      builder: (dialogCtx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          backgroundColor: AppTokens.boneBackground,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            side: const BorderSide(color: AppTokens.crispBorder),
          ),
          title: Text(
            'Add Note / Highlight',
            style: AppTypography.titleSerif(fontSize: 18),
          ),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(
                  controller: textController,
                  autofocus: true,
                  maxLines: 3,
                  decoration: const InputDecoration(
                    labelText: 'Selected Text / Quote',
                    hintText: 'Enter quoted passage...',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: AppTokens.space12),
                TextField(
                  controller: noteController,
                  maxLines: 2,
                  decoration: const InputDecoration(
                    labelText: 'Personal Note (optional)',
                    hintText: 'Your thoughts or reflection...',
                    border: OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: AppTokens.space16),
                Text(
                  'Highlight Color',
                  style: AppTypography.captionSans(fontSize: 12),
                ),
                const SizedBox(height: AppTokens.space8),
                Row(
                  children: [
                    for (final color in ['yellow', 'green', 'blue', 'pink'])
                      GestureDetector(
                        onTap: () {
                          setDialogState(() => selectedColor = color);
                        },
                        child: Container(
                          margin: const EdgeInsets.only(right: AppTokens.space12),
                          width: 32,
                          height: 32,
                          decoration: BoxDecoration(
                            color: _highlightColor(color),
                            shape: BoxShape.circle,
                            border: Border.all(
                              color: selectedColor == color
                                  ? AppTokens.charcoalInk
                                  : AppTokens.crispBorder,
                              width: selectedColor == color ? 2.5 : 1.0,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogCtx).pop(),
              child: const Text('Cancel'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: AppTokens.charcoalInk,
              ),
              onPressed: () {
                final quote = textController.text.trim();
                if (quote.isNotEmpty) {
                  ref.read(bookDetailProvider(widget.bookId).notifier).addHighlight(
                        selectedText: quote,
                        color: selectedColor,
                        note: noteController.text.trim().isNotEmpty
                            ? noteController.text.trim()
                            : null,
                      );
                }
                Navigator.of(dialogCtx).pop();
              },
              child: const Text('Save Highlight'),
            ),
          ],
        ),
      ),
    );
  }

  Color _highlightColor(String colorName) {
    switch (colorName.toLowerCase()) {
      case 'green':
        return const Color(0xFFD1E7DD);
      case 'blue':
        return const Color(0xFFCFE2FF);
      case 'pink':
        return const Color(0xFFF8D7DA);
      case 'yellow':
      default:
        return const Color(0xFFFFF3CD);
    }
  }

  String _formatFileSize(int? bytes) {
    if (bytes == null || bytes <= 0) return 'EPUB';
    if (bytes < 1024 * 1024) {
      return '${(bytes / 1024).toStringAsFixed(1)} KB';
    }
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  @override
  Widget build(BuildContext context) {
    final detailState = ref.watch(bookDetailProvider(widget.bookId));
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);

    if (detailState.isLoading && detailState.book == null) {
      return const Scaffold(
        backgroundColor: AppTokens.boneBackground,
        body: Center(
          child: CircularProgressIndicator(color: AppTokens.charcoalInk),
        ),
      );
    }

    if (detailState.error != null && detailState.book == null) {
      return Scaffold(
        backgroundColor: AppTokens.boneBackground,
        appBar: AppBar(
          backgroundColor: AppTokens.boneBackground,
          elevation: 0,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back, color: AppTokens.charcoalInk),
            onPressed: () {
              if (context.canPop()) {
                context.pop();
              } else {
                context.go('/library');
              }
            },
          ),
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppTokens.space32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(Icons.error_outline_rounded, size: 48, color: Colors.red),
                const SizedBox(height: AppTokens.space16),
                Text(
                  'Failed to load book details',
                  style: AppTypography.titleSerif(fontSize: 20),
                ),
                const SizedBox(height: AppTokens.space8),
                Text(
                  detailState.error!,
                  textAlign: TextAlign.center,
                  style: AppTypography.bodySans(fontSize: 14, color: AppTokens.mutedCopy),
                ),
                const SizedBox(height: AppTokens.space24),
                FilledButton(
                  style: FilledButton.styleFrom(backgroundColor: AppTokens.charcoalInk),
                  onPressed: () {
                    ref.read(bookDetailProvider(widget.bookId).notifier).loadBook();
                  },
                  child: const Text('Try Again'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    final book = detailState.book!;

    return Scaffold(
      backgroundColor: AppTokens.boneBackground,
      appBar: AppBar(
        backgroundColor: AppTokens.boneBackground,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: AppTokens.charcoalInk),
          tooltip: 'Back to Library',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/library');
            }
          },
        ),
        title: Text(
          book.title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: AppTypography.titleSerif(fontSize: 18, fontWeight: FontWeight.w600),
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.bookmark_add_outlined, color: AppTokens.charcoalInk),
            tooltip: 'Add Bookmark',
            onPressed: () => _showAddBookmarkDialog(context, book),
          ),
          IconButton(
            icon: const Icon(Icons.note_add_outlined, color: AppTokens.charcoalInk),
            tooltip: 'Add Note / Highlight',
            onPressed: () => _showAddHighlightDialog(context, book),
          ),
          Padding(
            padding: const EdgeInsets.only(right: AppTokens.space16, left: AppTokens.space8),
            child: FilledButton.icon(
              style: FilledButton.styleFrom(
                backgroundColor: AppTokens.charcoalInk,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16),
              ),
              icon: const Icon(Icons.menu_book_rounded, size: 18),
              label: Text(book.readingProgress > 0 ? 'Resume' : 'Read'),
              onPressed: () => context.go('/reader/${book.id}/read'),
            ),
          ),
        ],
      ),
      body: SafeArea(
        child: isDesktop
            ? _buildDesktopLayout(context, book, horizontalPad)
            : _buildMobileLayout(context, book, horizontalPad),
      ),
    );
  }

  Widget _buildDesktopLayout(BuildContext context, Book book, double horizontalPad) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space24),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Left Sticky Column: Hero Book Metadata Card (360px)
          SizedBox(
            width: 360,
            child: SingleChildScrollView(
              child: _buildHeroCard(context, book),
            ),
          ),
          const SizedBox(width: AppTokens.space32),
          // Right Flexible Column: Segmented Tabs & Content
          Expanded(
            child: BentoCard(
              padding: EdgeInsets.zero,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _buildTabBar(),
                  const Divider(height: 1, color: AppTokens.crispBorder),
                  Expanded(
                    child: _buildTabContent(context, book),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMobileLayout(BuildContext context, Book book, double horizontalPad) {
    return SingleChildScrollView(
      padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _buildHeroCard(context, book),
          const SizedBox(height: AppTokens.space20),
          _buildTabBar(),
          const SizedBox(height: AppTokens.space16),
          _buildTabContent(context, book),
          const SizedBox(height: AppTokens.space48),
        ],
      ),
    );
  }

  Widget _buildHeroCard(BuildContext context, Book book) {
    final progressPercent = (book.readingProgress * 100).toInt();

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Cover & Title Row
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // 2:3 Cover Aspect Ratio
              ClipRRect(
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                child: Container(
                  width: 110,
                  height: 165,
                  decoration: BoxDecoration(
                    color: AppTokens.boneContainer,
                    border: Border.all(color: AppTokens.crispBorder),
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  child: book.coverUrl != null
                      ? Image.network(
                          book.coverUrl!,
                          fit: BoxFit.cover,
                          errorBuilder: (context, error, stackTrace) =>
                              _buildCoverFallback(book.title),
                        )
                      : _buildCoverFallback(book.title),
                ),
              ),
              const SizedBox(width: AppTokens.space16),
              // Main Title & Series Info
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      book.title,
                      style: AppTypography.titleSerif(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: AppTokens.space4),
                    Text(
                      book.authorDisplay,
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        color: AppTokens.mutedCopy,
                      ),
                    ),
                    if (book.series != null) ...[
                      const SizedBox(height: AppTokens.space8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppTokens.space8,
                          vertical: AppTokens.space4,
                        ),
                        decoration: BoxDecoration(
                          color: AppTokens.matchBadgeBg,
                          borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                          border: Border.all(color: AppTokens.crispBorder),
                        ),
                        child: Text(
                          book.seriesSequence != null
                              ? '${book.series!.name} #${book.seriesSequence!.toStringAsFixed(book.seriesSequence!.truncateToDouble() == book.seriesSequence! ? 0 : 1)}'
                              : book.series!.name,
                          style: AppTypography.captionSans(
                            fontSize: 11,
                            color: AppTokens.matchBadgeText,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space20),

          // Reading Progress Bar
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    'Reading Progress',
                    style: AppTypography.captionSans(fontSize: 12),
                  ),
                  Text(
                    '$progressPercent%',
                    style: AppTypography.captionSans(
                      fontSize: 12,
                      color: AppTokens.mutedCopy,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space8),
              ClipRRect(
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                child: LinearProgressIndicator(
                  value: book.readingProgress.clamp(0.0, 1.0),
                  backgroundColor: AppTokens.boneContainer,
                  valueColor: const AlwaysStoppedAnimation<Color>(AppTokens.charcoalInk),
                  minHeight: 6,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Primary CTA button
          SizedBox(
            width: double.infinity,
            height: AppTokens.minTouchTarget,
            child: FilledButton.icon(
              style: FilledButton.styleFrom(
                backgroundColor: AppTokens.charcoalInk,
                foregroundColor: Colors.white,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                ),
              ),
              icon: const Icon(Icons.menu_book_rounded, size: 20),
              label: Text(
                book.readingProgress > 0 ? 'Resume Reading' : 'Start Reading',
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
              onPressed: () => context.go('/reader/${book.id}/read'),
            ),
          ),
          const SizedBox(height: AppTokens.space20),
          const Divider(height: 1, color: AppTokens.crispBorder),
          const SizedBox(height: AppTokens.space16),

          // Metadata Specs Grid
          Text(
            'Book Details',
            style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space12),
          _buildSpecRow('Chapters', '${book.spine.length} chapters'),
          if (book.publisher != null && book.publisher!.isNotEmpty)
            _buildSpecRow('Publisher', book.publisher!),
          if (book.publishedDate != null && book.publishedDate!.isNotEmpty)
            _buildSpecRow('Published', book.publishedDate!),
          if (book.language != null && book.language!.isNotEmpty)
            _buildSpecRow('Language', book.language!.toUpperCase()),
          _buildSpecRow('Format', _formatFileSize(book.fileSizeBytes)),
          if (book.genres.isNotEmpty)
            _buildSpecRow(
              'Genres',
              book.genres.map((g) => g.name).join(', '),
            ),
        ],
      ),
    );
  }

  Widget _buildCoverFallback(String title) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space12),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.book_rounded, size: 32, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            title,
            maxLines: 2,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 11),
          ),
        ],
      ),
    );
  }

  Widget _buildSpecRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppTokens.space8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 90,
            child: Text(
              label,
              style: AppTypography.captionSans(fontSize: 12, color: AppTokens.mutedCopy),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: AppTypography.bodySans(fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTabBar() {
    return Container(
      color: AppTokens.boneSurface,
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space12,
        vertical: AppTokens.space8,
      ),
      child: Row(
        children: [
          _buildTabButton(0, 'Overview', Icons.info_outline_rounded),
          const SizedBox(width: AppTokens.space8),
          _buildTabButton(1, 'Highlights & Bookmarks', Icons.bookmark_outline_rounded),
          const SizedBox(width: AppTokens.space8),
          _buildTabButton(2, 'Chat with Book', Icons.auto_awesome_rounded),
        ],
      ),
    );
  }

  Widget _buildTabButton(int index, String label, IconData icon) {
    final isSelected = _selectedTabIndex == index;
    return Expanded(
      child: InkWell(
        onTap: () {
          setState(() {
            _selectedTabIndex = index;
          });
        },
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          height: AppTokens.minTouchTarget,
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space8),
          decoration: BoxDecoration(
            color: isSelected ? AppTokens.boneBackground : Colors.transparent,
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: Border.all(
              color: isSelected ? AppTokens.crispBorder : Colors.transparent,
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                icon,
                size: 16,
                color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
              ),
              const SizedBox(width: AppTokens.space8),
              Flexible(
                child: Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                    color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildTabContent(BuildContext context, Book book) {
    switch (_selectedTabIndex) {
      case 0:
        return _buildOverviewTab(context, book);
      case 1:
        return _buildHighlightsBookmarksTab(context, book);
      case 2:
      default:
        return _buildChatTab(context, book);
    }
  }

  Widget _buildOverviewTab(BuildContext context, Book book) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Synopsis Section
          Text(
            'Synopsis',
            style: AppTypography.titleSerif(fontSize: 18, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            book.synopsis.isNotEmpty ? book.synopsis : 'No synopsis provided for this book.',
            style: AppTypography.bodySans(fontSize: 15, lineHeight: 1.6),
          ),
          const SizedBox(height: AppTokens.space32),
          const Divider(height: 1, color: AppTokens.crispBorder),
          const SizedBox(height: AppTokens.space24),

          // Table of Contents Spine
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Table of Contents',
                style: AppTypography.titleSerif(fontSize: 18, fontWeight: FontWeight.w600),
              ),
              Text(
                '${book.spine.length} items',
                style: AppTypography.captionSans(fontSize: 12, color: AppTokens.mutedCopy),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),
          if (book.spine.isEmpty)
            Text(
              'No chapters cataloged.',
              style: AppTypography.bodySans(fontSize: 14, color: AppTokens.mutedCopy),
            )
          else
            ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: book.spine.length,
              separatorBuilder: (context, index) =>
                  const Divider(height: 1, color: AppTokens.crispBorder),
              itemBuilder: (context, index) {
                final item = book.spine[index];
                return ListTile(
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space8,
                    vertical: AppTokens.space4,
                  ),
                  title: Text(
                    item.title,
                    style: AppTypography.bodySans(fontSize: 14, fontWeight: FontWeight.w500),
                  ),
                  subtitle: item.summary.isNotEmpty
                      ? Padding(
                          padding: const EdgeInsets.only(top: AppTokens.space4),
                          child: Text(
                            item.summary,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.captionSans(
                              fontSize: 12,
                              color: AppTokens.mutedCopy,
                            ),
                          ),
                        )
                      : null,
                  trailing: const Icon(
                    Icons.chevron_right_rounded,
                    size: 20,
                    color: AppTokens.mutedCopy,
                  ),
                  onTap: () {
                    context.go('/reader/${book.id}/read/${item.chapterIndex}');
                  },
                );
              },
            ),
        ],
      ),
    );
  }

  Widget _buildHighlightsBookmarksTab(BuildContext context, Book book) {
    final bookmarks = book.bookmarks;
    final highlights = book.highlights;

    List<dynamic> items = [];
    if (_annotationFilter == 'all') {
      items = [...highlights, ...bookmarks];
    } else if (_annotationFilter == 'highlights') {
      items = highlights;
    } else {
      items = bookmarks;
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Filter pills and Add action row
          Row(
            children: [
              _buildAnnotationFilterChip('all', 'All (${highlights.length + bookmarks.length})'),
              const SizedBox(width: AppTokens.space8),
              _buildAnnotationFilterChip('highlights', 'Highlights (${highlights.length})'),
              const SizedBox(width: AppTokens.space8),
              _buildAnnotationFilterChip('bookmarks', 'Bookmarks (${bookmarks.length})'),
              const Spacer(),
              PopupMenuButton<String>(
                icon: const Icon(Icons.add_circle_outline_rounded, color: AppTokens.charcoalInk),
                tooltip: 'Add annotation',
                onSelected: (val) {
                  if (val == 'bookmark') {
                    _showAddBookmarkDialog(context, book);
                  } else {
                    _showAddHighlightDialog(context, book);
                  }
                },
                itemBuilder: (ctx) => [
                  const PopupMenuItem(
                    value: 'bookmark',
                    child: Row(
                      children: [
                        Icon(Icons.bookmark_add_outlined, size: 18),
                        SizedBox(width: 8),
                        Text('Add Bookmark'),
                      ],
                    ),
                  ),
                  const PopupMenuItem(
                    value: 'highlight',
                    child: Row(
                      children: [
                        Icon(Icons.note_add_outlined, size: 18),
                        SizedBox(width: 8),
                        Text('Add Note / Highlight'),
                      ],
                    ),
                  ),
                ],
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space20),

          if (items.isEmpty)
            Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: AppTokens.space48),
                child: Column(
                  children: [
                    const Icon(
                      Icons.bookmarks_outlined,
                      size: 44,
                      color: AppTokens.mutedCopy,
                    ),
                    const SizedBox(height: AppTokens.space12),
                    Text(
                      'No annotations found',
                      style: AppTypography.titleSerif(fontSize: 16),
                    ),
                    const SizedBox(height: AppTokens.space4),
                    Text(
                      'Save your favorite passages or bookmarks as you read.',
                      style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
                    ),
                  ],
                ),
              ),
            )
          else
            ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: items.length,
              separatorBuilder: (context, index) =>
                  const SizedBox(height: AppTokens.space12),
              itemBuilder: (context, index) {
                final item = items[index];
                if (item is Highlight) {
                  return _buildHighlightCard(item);
                } else if (item is Bookmark) {
                  return _buildBookmarkCard(item);
                }
                return const SizedBox.shrink();
              },
            ),
        ],
      ),
    );
  }

  Widget _buildAnnotationFilterChip(String key, String label) {
    final isSelected = _annotationFilter == key;
    return ChoiceChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (selected) {
        if (selected) {
          setState(() {
            _annotationFilter = key;
          });
        }
      },
      selectedColor: AppTokens.boneContainer,
      backgroundColor: Colors.transparent,
      side: BorderSide(
        color: isSelected ? AppTokens.charcoalInk : AppTokens.crispBorder,
      ),
      labelStyle: TextStyle(
        fontSize: 12,
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
        color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
      ),
    );
  }

  Widget _buildHighlightCard(Highlight hl) {
    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 4,
            height: 54,
            decoration: BoxDecoration(
              color: _highlightColor(hl.color),
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            ),
          ),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '"${hl.selectedText}"',
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                  ),
                ),
                if (hl.note != null && hl.note!.isNotEmpty) ...[
                  const SizedBox(height: AppTokens.space8),
                  Text(
                    hl.note!,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: AppTokens.mutedCopy,
                    ),
                  ),
                ],
                const SizedBox(height: AppTokens.space8),
                Text(
                  hl.createdAt != null
                      ? 'Highlighted on ${hl.createdAt!.month}/${hl.createdAt!.day}/${hl.createdAt!.year}'
                      : 'Highlighted quote',
                  style: AppTypography.captionSans(fontSize: 11, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.delete_outline_rounded, size: 20, color: AppTokens.mutedCopy),
            tooltip: 'Delete Highlight',
            onPressed: () {
              ref.read(bookDetailProvider(widget.bookId).notifier).removeHighlight(hl.id);
            },
          ),
        ],
      ),
    );
  }

  Widget _buildBookmarkCard(Bookmark bm) {
    final percent = (bm.progress * 100).toInt();

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          const Icon(Icons.bookmark_rounded, color: AppTokens.charcoalInk, size: 24),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  bm.title,
                  style: AppTypography.bodySans(fontSize: 14, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  '$percent% completed',
                  style: AppTypography.captionSans(fontSize: 11, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.delete_outline_rounded, size: 20, color: AppTokens.mutedCopy),
            tooltip: 'Delete Bookmark',
            onPressed: () {
              ref.read(bookDetailProvider(widget.bookId).notifier).removeBookmark(bm.id);
            },
          ),
        ],
      ),
    );
  }

  Widget _buildChatTab(BuildContext context, Book book) {
    final chatState = ref.watch(bookChatProvider(widget.bookId));

    return Column(
      children: [
        // RAG Header info banner
        Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppTokens.space20,
            vertical: AppTokens.space12,
          ),
          decoration: const BoxDecoration(
            color: AppTokens.boneSurface,
            border: Border(bottom: BorderSide(color: AppTokens.crispBorder)),
          ),
          child: Row(
            children: [
              const Icon(Icons.auto_awesome_rounded, size: 18, color: AppTokens.charcoalInk),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Grounding questions in chapter vector embeddings & summaries',
                  style: AppTypography.captionSans(fontSize: 12, color: AppTokens.mutedCopy),
                ),
              ),
              if (chatState.messages.isNotEmpty)
                TextButton(
                  onPressed: () {
                    ref.read(bookChatProvider(widget.bookId).notifier).clearChat();
                  },
                  child: const Text('Clear', style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
        ),

        // Chat conversation history
        Expanded(
          child: chatState.messages.isEmpty
              ? _buildEmptyChatView(book)
              : ListView.builder(
                  controller: _chatScrollController,
                  padding: const EdgeInsets.all(AppTokens.space16),
                  itemCount: chatState.messages.length + (chatState.isSending ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == chatState.messages.length) {
                      return _buildTypingIndicator();
                    }
                    final msg = chatState.messages[index];
                    return _buildChatMessageBubble(msg);
                  },
                ),
        ),

        // Error message if any
        if (chatState.error != null)
          Container(
            padding: const EdgeInsets.all(AppTokens.space8),
            color: Colors.red.shade50,
            child: Row(
              children: [
                const Icon(Icons.error_outline, size: 16, color: Colors.red),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    chatState.error!,
                    style: const TextStyle(fontSize: 12, color: Colors.red),
                  ),
                ),
              ],
            ),
          ),

        // Bottom text input bar
        Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: const BoxDecoration(
            color: AppTokens.boneBackground,
            border: Border(top: BorderSide(color: AppTokens.crispBorder)),
          ),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _chatController,
                  onSubmitted: (_) => _sendMessage(),
                  decoration: InputDecoration(
                    hintText: 'Ask about characters, themes, or plot points...',
                    hintStyle: AppTypography.bodySans(
                      fontSize: 13,
                      color: AppTokens.mutedCopy,
                    ),
                    filled: true,
                    fillColor: AppTokens.boneContainer,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space16,
                      vertical: AppTokens.space12,
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      borderSide: const BorderSide(color: AppTokens.crispBorder),
                    ),
                    enabledBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      borderSide: const BorderSide(color: AppTokens.crispBorder),
                    ),
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              IconButton.filled(
                style: IconButton.styleFrom(
                  backgroundColor: AppTokens.charcoalInk,
                  foregroundColor: Colors.white,
                ),
                icon: const Icon(Icons.arrow_upward_rounded),
                onPressed: chatState.isSending ? null : () => _sendMessage(),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildEmptyChatView(Book book) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppTokens.space24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              padding: const EdgeInsets.all(AppTokens.space16),
              decoration: const BoxDecoration(
                color: AppTokens.boneContainer,
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.auto_awesome_rounded,
                size: 36,
                color: AppTokens.charcoalInk,
              ),
            ),
            const SizedBox(height: AppTokens.space16),
            Text(
              'Ask anything about ${book.title}',
              textAlign: TextAlign.center,
              style: AppTypography.titleSerif(fontSize: 18),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Shelfd uses semantic search to cite relevant chapters and answer questions.',
              textAlign: TextAlign.center,
              style: AppTypography.bodySans(fontSize: 13, color: AppTokens.mutedCopy),
            ),
            const SizedBox(height: AppTokens.space24),
            Wrap(
              spacing: AppTokens.space8,
              runSpacing: AppTokens.space8,
              alignment: WrapAlignment.center,
              children: [
                _buildPromptChip('Summarize the major themes'),
                _buildPromptChip('Who are the main characters?'),
                _buildPromptChip('What happens in the opening chapter?'),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPromptChip(String prompt) {
    return ActionChip(
      label: Text(prompt),
      backgroundColor: AppTokens.boneBackground,
      side: const BorderSide(color: AppTokens.crispBorder),
      labelStyle: AppTypography.bodySans(fontSize: 12),
      onPressed: () => _sendMessage(prompt),
    );
  }

  Widget _buildTypingIndicator() {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppTokens.space8),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppTokens.space16,
              vertical: AppTokens.space12,
            ),
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: AppTokens.charcoalInk,
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Text(
                  'Consulting chapter context...',
                  style: AppTypography.captionSans(fontSize: 12, color: AppTokens.mutedCopy),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildChatMessageBubble(BookChatMessage msg) {
    final isUser = msg.role == 'user';

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppTokens.space8),
      child: Row(
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!isUser) ...[
            Container(
              margin: const EdgeInsets.only(right: AppTokens.space8, top: 4),
              width: 28,
              height: 28,
              decoration: const BoxDecoration(
                color: AppTokens.charcoalInk,
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.auto_awesome, size: 14, color: Colors.white),
            ),
          ],
          Flexible(
            child: Container(
              padding: const EdgeInsets.all(AppTokens.space16),
              decoration: BoxDecoration(
                color: isUser ? AppTokens.charcoalInk : AppTokens.boneContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    msg.content,
                    style: TextStyle(
                      fontSize: 14,
                      height: 1.5,
                      color: isUser ? Colors.white : AppTokens.charcoalInk,
                    ),
                  ),
                  if (!isUser && msg.citations.isNotEmpty) ...[
                    const SizedBox(height: AppTokens.space12),
                    Wrap(
                      spacing: AppTokens.space8,
                      runSpacing: AppTokens.space8,
                      children: [
                        for (final cit in msg.citations)
                          ActionChip(
                            avatar: const Icon(Icons.menu_book, size: 14),
                            label: Text(
                              cit.chapterTitle != null && cit.chapterTitle!.isNotEmpty
                                  ? 'Ch. ${cit.chapterIndex}: ${cit.chapterTitle}'
                                  : 'Chapter ${cit.chapterIndex}',
                              style: const TextStyle(fontSize: 11),
                            ),
                            backgroundColor: AppTokens.boneBackground,
                            side: const BorderSide(color: AppTokens.crispBorder),
                            onPressed: () {
                              context.go('/reader/${widget.bookId}/read/${cit.chapterIndex}');
                            },
                          ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
