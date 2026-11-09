import 'package:flutter/material.dart';
import 'package:flutter_markdown_plus/flutter_markdown_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/book_chat.dart';
import '../../../data/models/bookmark.dart';
import '../../../data/models/highlight.dart';
import '../../core/file_saver/file_saver.dart';
import '../../core/html_text.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'edit_book_metadata_dialog.dart';

class BookDetailView extends ConsumerStatefulWidget {
  final String bookId;

  const BookDetailView({super.key, required this.bookId});

  @override
  ConsumerState<BookDetailView> createState() => _BookDetailViewState();
}

class _BookDetailViewState extends ConsumerState<BookDetailView> {
  int _selectedTabIndex = 0;
  String _annotationFilter = 'all'; // 'all', 'highlights', 'bookmarks'
  bool _isDownloading = false;
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
      builder: (dialogCtx) {
        final colors = ShelfdColors.of(dialogCtx);
        return AlertDialog(
          backgroundColor: colors.surface,
          surfaceTintColor: Colors.transparent,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            side: BorderSide(color: colors.border),
          ),
          title: Text(
            'Add Bookmark',
            style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
          ),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Save your current place in ${book.title}.',
                style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
              ),
              const SizedBox(height: AppTokens.space16),
              TextField(
                controller: titleController,
                autofocus: true,
                style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
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
              child: Text(
                'Cancel',
                style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
              ),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: colors.accentPrimary,
                foregroundColor: colors.onAccent,
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
        );
      },
    );
  }

  String _sanitizeBookFileName(String rawTitle, String extension) {
    final trimmed = rawTitle.trim();
    final sanitized = trimmed.replaceAll(RegExp(r'[\\/:*?"<>|\x00-\x1f]'), '_').trim();
    final cleanTitle = sanitized.isNotEmpty ? sanitized : 'book';
    final cleanExt = extension.startsWith('.') ? extension.substring(1) : extension;
    return '$cleanTitle.$cleanExt';
  }

  void _handleExportMenuSelection(String action, Book book) {
    switch (action) {
      case 'epub':
        _downloadEpub(book);
        break;
      case 'notebooklm':
        _sendToNotebookLM(book);
        break;
      case 'markdown':
        _downloadMarkdown(book);
        break;
    }
  }

  List<PopupMenuEntry<String>> _buildExportMenuItems(ShelfdColors colors) {
    return [
      PopupMenuItem<String>(
        key: const Key('menu_item_download_epub'),
        value: 'epub',
        child: Row(
          children: [
            Icon(Icons.download_rounded, size: 18, color: colors.ink),
            const SizedBox(width: AppTokens.space12),
            Text(
              'Download EPUB',
              style: AppTypography.bodySans(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: colors.ink,
              ),
            ),
          ],
        ),
      ),
      PopupMenuItem<String>(
        key: const Key('menu_item_send_notebooklm'),
        value: 'notebooklm',
        child: Row(
          children: [
            Icon(Icons.auto_awesome_rounded, size: 18, color: colors.accentPrimary),
            const SizedBox(width: AppTokens.space12),
            Text(
              'Send to NotebookLM',
              style: AppTypography.bodySans(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: colors.ink,
              ),
            ),
          ],
        ),
      ),
      PopupMenuItem<String>(
        key: const Key('menu_item_download_markdown'),
        value: 'markdown',
        child: Row(
          children: [
            Icon(Icons.description_outlined, size: 18, color: colors.secondaryInk),
            const SizedBox(width: AppTokens.space12),
            Text(
              'Download Markdown (.md)',
              style: AppTypography.bodySans(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: colors.ink,
              ),
            ),
          ],
        ),
      ),
    ];
  }

  Future<void> _downloadEpub(Book book) async {
    if (_isDownloading) return;
    setState(() {
      _isDownloading = true;
    });

    try {
      final repo = ref.read(bookRepositoryProvider);
      final bytes = await repo.downloadBookEpub(book.id);
      final fileName = _sanitizeBookFileName(book.title, 'epub');

      final saved = await saveDownloadedFile(
        bytes,
        fileName,
        mimeType: 'application/epub+zip',
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              saved ? 'Downloaded $fileName' : 'Download canceled',
            ),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to download EPUB: $e'),
            backgroundColor: Colors.red.shade800,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isDownloading = false;
        });
      }
    }
  }

  Future<void> _downloadMarkdown(Book book) async {
    if (_isDownloading) return;
    setState(() {
      _isDownloading = true;
    });

    try {
      final repo = ref.read(bookRepositoryProvider);
      final bytes = await repo.downloadBookNotebookLM(book.id);
      final fileName = _sanitizeBookFileName(book.title, 'md');

      final saved = await saveDownloadedFile(
        bytes,
        fileName,
        mimeType: 'text/markdown;charset=utf-8',
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              saved ? 'Downloaded $fileName' : 'Download canceled',
            ),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to download Markdown: $e'),
            backgroundColor: Colors.red.shade800,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isDownloading = false;
        });
      }
    }
  }

  Future<void> _sendToNotebookLM(Book book) async {
    if (_isDownloading) return;
    setState(() {
      _isDownloading = true;
    });

    try {
      final repo = ref.read(bookRepositoryProvider);
      final result = await repo.sendBookToNotebookLM(book.id);
      final targetUrl = result.notebookUrl.isNotEmpty
          ? result.notebookUrl
          : 'https://notebooklm.google.com/';

      if (result.status == 'sent') {
        await openExternalUrl(targetUrl);
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                (result.message != null && result.message!.isNotEmpty)
                    ? result.message!
                    : 'Sent "${book.title}" to NotebookLM',
              ),
              behavior: SnackBarBehavior.floating,
            ),
          );
        }
      } else {
        // Default client_handoff flow: download compiled NotebookLM Markdown bundle and open NotebookLM
        final bytes = await repo.downloadBookNotebookLM(book.id);
        final fileName = (result.fileName != null && result.fileName!.isNotEmpty)
            ? result.fileName!
            : _sanitizeBookFileName(book.title, 'md');

        await saveDownloadedFile(
          bytes,
          fileName,
          mimeType: 'text/markdown;charset=utf-8',
        );
        await openExternalUrl(targetUrl);

        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                (result.message != null && result.message!.isNotEmpty)
                    ? result.message!
                    : 'Exported $fileName for NotebookLM — upload it as a source!',
              ),
              behavior: SnackBarBehavior.floating,
            ),
          );
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to send to NotebookLM: $e'),
            backgroundColor: Colors.red.shade800,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isDownloading = false;
        });
      }
    }
  }

  Color _highlightColor(String colorName, {bool isDark = false}) {
    final hlColor = KindleHighlightColor.fromName(colorName);
    return isDark ? hlColor.darkColor : hlColor.cardColor;
  }

  bool _hasAudiobook(Book book) {
    return book.isAudiobook ||
        (book.durationSeconds != null && book.durationSeconds! > 0) ||
        book.files.any((f) => f.fileType == 'audiobook' || f.durationSeconds != null);
  }

  String _formatAudioDuration(double? seconds) {
    if (seconds == null || seconds <= 0) return '';
    final d = Duration(seconds: seconds.round());
    final hours = d.inHours;
    final minutes = d.inMinutes.remainder(60);
    if (hours > 0) {
      return '${hours}h ${minutes}m';
    }
    return '${minutes}m';
  }

  String _formatFileSize(int? bytes) {
    if (bytes == null || bytes <= 0) return 'Unknown size';
    if (bytes < 1024 * 1024) {
      return '${(bytes / 1024).toStringAsFixed(1)} KB';
    }
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  String _formatChapterCount(Book book) {
    if (book.spine.isNotEmpty) {
      if (book.isFixedLayout) {
        final namedCount = book.namedSpine.length;
        if (namedCount > 0) {
          return '$namedCount chapters (${book.spine.length} pages)';
        }
        return '${book.spine.length} pages';
      }
      return '${book.spine.length} chapters';
    }
    if (book.audioChapters.isNotEmpty) {
      return '${book.audioChapters.length} chapters';
    }
    final audioFilesCount = book.files.where((f) => f.fileType == 'audiobook' || f.filePath.endsWith('.m4a') || f.filePath.endsWith('.m4b') || f.filePath.endsWith('.mp3')).length;
    if (audioFilesCount > 0) {
      return '$audioFilesCount chapters';
    }
    return '0 chapters';
  }

  String _formatBookFormat(Book book) {
    final hasEpub = book.spine.isNotEmpty || !book.isAudiobook || book.files.any((f) => f.fileType == 'epub' || f.filePath.endsWith('.epub'));
    final hasAudio = _hasAudiobook(book);
    final audioFiles = book.files.where((f) => f.fileType == 'audiobook' || f.filePath.endsWith('.m4a') || f.filePath.endsWith('.m4b') || f.filePath.endsWith('.mp3')).toList();

    if (hasEpub && hasAudio) {
      if (audioFiles.length > 1) {
        return 'Dual (EPUB + ${audioFiles.length} Audio Files)';
      }
      return 'Dual (EPUB + M4A)';
    } else if (hasAudio) {
      if (audioFiles.length > 1) {
        return 'Multi-file Audiobook (${audioFiles.length} files)';
      }
      if (book.files.isNotEmpty && book.files.first.filePath.endsWith('.m4b')) {
        return 'M4B Audiobook';
      }
      return 'M4A Audiobook';
    } else {
      return _formatFileSize(book.fileSizeBytes);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final detailState = ref.watch(bookDetailProvider(widget.bookId));
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);

    if (detailState.isLoading && detailState.book == null) {
      return Scaffold(
        backgroundColor: colors.background,
        body: Center(
          child: CircularProgressIndicator(color: colors.accentPrimary),
        ),
      );
    }

    if (detailState.error != null && detailState.book == null) {
      return Scaffold(
        backgroundColor: colors.background,
        appBar: AppBar(
          backgroundColor: colors.background,
          elevation: 0,
          leading: IconButton(
            icon: Icon(Icons.arrow_back, color: colors.ink),
            onPressed: () {
              if (context.canPop()) {
                context.pop();
              } else {
                context.go('/books');
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
                Icon(Icons.error_outline_rounded, size: 48, color: colors.errorText),
                const SizedBox(height: AppTokens.space16),
                Text(
                  'Failed to load book details',
                  style: AppTypography.titleSerif(fontSize: 20, color: colors.ink),
                ),
                const SizedBox(height: AppTokens.space8),
                Text(
                  detailState.error!,
                  textAlign: TextAlign.center,
                  style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
                ),
                const SizedBox(height: AppTokens.space24),
                FilledButton(
                  style: FilledButton.styleFrom(
                    backgroundColor: colors.accentPrimary,
                    foregroundColor: colors.onAccent,
                  ),
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
    final hasAudio = _hasAudiobook(book);

    return Scaffold(
      backgroundColor: colors.background,
      appBar: AppBar(
        backgroundColor: colors.background,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.ink),
          tooltip: 'Back to Library',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/books');
            }
          },
        ),
        title: Text(
          book.title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: AppTypography.titleSerif(
            fontSize: 18,
            fontWeight: FontWeight.w600,
            color: colors.ink,
          ),
        ),
        actions: [
          IconButton(
            key: const Key('edit_metadata_appbar_button'),
            icon: Icon(Icons.edit_note_rounded, color: colors.ink),
            tooltip: 'Edit Metadata',
            onPressed: () => showEditBookMetadataModal(context, book),
          ),
          PopupMenuButton<String>(
            key: const Key('download_appbar_menu_button'),
            enabled: !_isDownloading,
            tooltip: 'Download EPUB',
            position: PopupMenuPosition.under,
            color: colors.surface,
            surfaceTintColor: Colors.transparent,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              side: BorderSide(color: colors.border),
            ),
            icon: _isDownloading
                ? SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: colors.ink,
                    ),
                  )
                : Icon(Icons.download_rounded, color: colors.ink),
            onSelected: (action) => _handleExportMenuSelection(action, book),
            itemBuilder: (context) => _buildExportMenuItems(colors),
          ),
          IconButton(
            icon: Icon(Icons.bookmark_add_outlined, color: colors.ink),
            tooltip: 'Add Bookmark',
            onPressed: () => _showAddBookmarkDialog(context, book),
          ),
          if (hasAudio)
            Padding(
              padding: const EdgeInsets.only(left: AppTokens.space4, right: AppTokens.space4),
              child: FilledButton.icon(
                key: const Key('listen_appbar_button'),
                style: FilledButton.styleFrom(
                  backgroundColor: AppTokens.amberAccent,
                  foregroundColor: AppTokens.espressoInk,
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
                ),
                icon: const Icon(Icons.headphones, size: 18),
                label: const Text('Listen', style: TextStyle(fontWeight: FontWeight.w700)),
                onPressed: () => context.go('/books/${book.id}/listen'),
              ),
            ),
          Padding(
            padding: const EdgeInsets.only(right: AppTokens.space16, left: AppTokens.space4),
            child: FilledButton.icon(
              key: const Key('read_appbar_button'),
              style: FilledButton.styleFrom(
                backgroundColor: colors.accentPrimary,
                foregroundColor: colors.onAccent,
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16),
              ),
              icon: const Icon(Icons.menu_book_rounded, size: 18),
              label: Text(book.readingProgress > 0 ? 'Resume' : 'Read'),
              onPressed: () => context.go('/books/${book.id}/read'),
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
    final colors = ShelfdColors.of(context);
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
                  _buildTabBar(context),
                  Divider(height: 1, color: colors.subtleBorder),
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
          _buildTabBar(context),
          const SizedBox(height: AppTokens.space16),
          _buildTabContent(context, book),
          const SizedBox(height: AppTokens.space48),
        ],
      ),
    );
  }

  Widget _buildHeroCard(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
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
              // 2:3 3D Spine-Lit Cover
              TactileBookCover(
                coverUrl: book.coverUrl,
                title: book.title,
                width: 110,
                height: 165,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                showSpineCrease: true,
                showShadow: true,
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
                        color: colors.ink,
                      ),
                    ),
                    const SizedBox(height: AppTokens.space4),
                    if (book.authors.isNotEmpty)
                      InkWell(
                        onTap: () {
                          final author = book.authors.first;
                          context.go('/authors/${author.id}?name=${Uri.encodeComponent(author.name)}');
                        },
                        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 2),
                          child: Text(
                            book.authorDisplay,
                            style: AppTypography.bodySans(
                              fontSize: 14,
                              color: colors.secondaryInk,
                            ),
                          ),
                        ),
                      )
                    else
                      Text(
                        book.authorDisplay,
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          color: colors.secondaryInk,
                        ),
                      ),
                    if (book.series != null) ...[
                      const SizedBox(height: AppTokens.space8),
                      InkWell(
                        onTap: () {
                          context.go('/series/${book.series!.id}?name=${Uri.encodeComponent(book.series!.name)}');
                        },
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppTokens.space8,
                            vertical: AppTokens.space4,
                          ),
                          decoration: BoxDecoration(
                            color: colors.accentSecondaryContainer,
                            borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                            border: Border.all(color: colors.subtleBorder),
                          ),
                          child: Text(
                            book.seriesSequence != null
                                ? '${book.series!.name} #${book.seriesSequence!.toStringAsFixed(book.seriesSequence!.truncateToDouble() == book.seriesSequence! ? 0 : 1)}'
                                : book.series!.name,
                            style: AppTypography.captionSans(
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                              color: colors.accentSecondary,
                            ),
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
                    style: AppTypography.captionSans(
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                      color: colors.secondaryInk,
                    ),
                  ),
                  Text(
                    '$progressPercent%',
                    style: AppTypography.mono(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: colors.accentPrimary,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space8),
              ClipRRect(
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                child: LinearProgressIndicator(
                  value: book.readingProgress.clamp(0.0, 1.0),
                  backgroundColor: colors.container,
                  valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
                  minHeight: 6,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Dual Primary CTA buttons: Read EPUB & Listen Audiobook
          if (_hasAudiobook(book)) ...[
            SizedBox(
              width: double.infinity,
              height: AppTokens.minTouchTarget,
              child: FilledButton.icon(
                key: const Key('listen_hero_button'),
                style: FilledButton.styleFrom(
                  backgroundColor: AppTokens.amberAccent,
                  foregroundColor: AppTokens.espressoInk,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                ),
                icon: const Icon(Icons.headphones, size: 20),
                label: const Text(
                  'Listen Audiobook',
                  style: TextStyle(fontWeight: FontWeight.w700, fontSize: 15),
                ),
                onPressed: () => context.go('/books/${book.id}/listen'),
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            SizedBox(
              width: double.infinity,
              height: AppTokens.minTouchTarget,
              child: OutlinedButton.icon(
                key: const Key('read_hero_button'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                ),
                icon: const Icon(Icons.menu_book_rounded, size: 20),
                label: Text(
                  book.readingProgress > 0 ? 'Resume Reading' : 'Read EPUB',
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                onPressed: () => context.go('/books/${book.id}/read'),
              ),
            ),
          ] else ...[
            SizedBox(
              width: double.infinity,
              height: AppTokens.minTouchTarget,
              child: FilledButton.icon(
                key: const Key('read_hero_button'),
                style: FilledButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                ),
                icon: const Icon(Icons.menu_book_rounded, size: 20),
                label: Text(
                  book.readingProgress > 0 ? 'Resume Reading' : 'Start Reading',
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                onPressed: () => context.go('/books/${book.id}/read'),
              ),
            ),
          ],
          const SizedBox(height: AppTokens.space8),
          SizedBox(
            width: double.infinity,
            height: AppTokens.minTouchTarget,
            child: PopupMenuButton<String>(
              key: const Key('download_hero_menu_button'),
              enabled: !_isDownloading,
              tooltip: '',
              position: PopupMenuPosition.under,
              color: colors.surface,
              surfaceTintColor: Colors.transparent,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                side: BorderSide(color: colors.border),
              ),
              onSelected: (action) => _handleExportMenuSelection(action, book),
              itemBuilder: (context) => _buildExportMenuItems(colors),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.space16),
                decoration: BoxDecoration(
                  color: colors.containerLow,
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  border: Border.all(color: colors.border),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    if (_isDownloading)
                      SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: colors.ink,
                        ),
                      )
                    else
                      Icon(
                        Icons.download_rounded,
                        size: 20,
                        color: colors.ink,
                      ),
                    const SizedBox(width: AppTokens.space8),
                    Text(
                      _isDownloading ? 'Downloading...' : 'Download EPUB',
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
                    ),
                    const SizedBox(width: AppTokens.space4),
                    Icon(
                      Icons.arrow_drop_down_rounded,
                      size: 20,
                      color: colors.ink,
                    ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(height: AppTokens.space8),
          SizedBox(
            width: double.infinity,
            height: AppTokens.minTouchTarget,
            child: OutlinedButton.icon(
              key: const Key('edit_metadata_sidebar_button'),
              style: OutlinedButton.styleFrom(
                foregroundColor: colors.secondaryInk,
                side: BorderSide(color: colors.subtleBorder),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                ),
              ),
              icon: const Icon(Icons.edit_note_rounded, size: 20),
              label: const Text(
                'Edit Metadata',
                style: TextStyle(fontWeight: FontWeight.w600),
              ),
              onPressed: () => showEditBookMetadataModal(context, book),
            ),
          ),
          const SizedBox(height: AppTokens.space20),
          Divider(height: 1, color: colors.subtleBorder),
          const SizedBox(height: AppTokens.space16),

          // Metadata Specs Grid
          Text(
            'Book Details',
            style: AppTypography.titleSerif(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          _buildSpecRow(colors, 'Chapters', _formatChapterCount(book), isMono: true),
          if (book.durationSeconds != null && book.durationSeconds! > 0)
            _buildSpecRow(colors, 'Audio Length', _formatAudioDuration(book.durationSeconds), isMono: true),
          if (book.bitrateKbps != null && book.bitrateKbps! > 0)
            _buildSpecRow(colors, 'Audio Bitrate', '${book.bitrateKbps} kbps AAC', isMono: true),
          if (book.publisher != null && book.publisher!.isNotEmpty)
            _buildSpecRow(colors, 'Publisher', book.publisher!),
          if (book.publishedDate != null && book.publishedDate!.isNotEmpty)
            _buildSpecRow(colors, 'Published', book.publishedDate!, isMono: true),
          if (book.language != null && book.language!.isNotEmpty)
            _buildSpecRow(colors, 'Language', book.language!.toUpperCase(), isMono: true),
          _buildSpecRow(colors, 'Format', _formatBookFormat(book), isMono: true),
          if (book.genres.isNotEmpty)
            _buildSpecRow(
              colors,
              'Genres',
              book.genres.map((g) => g.name).join(', '),
            ),
        ],
      ),
    );
  }

  Widget _buildSpecRow(
    ShelfdColors colors,
    String label,
    String value, {
    bool isMono = false,
  }) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppTokens.space8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 90,
            child: Text(
              label,
              style: AppTypography.captionSans(fontSize: 12, color: colors.mutedInk),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: isMono
                  ? AppTypography.mono(fontSize: 12, color: colors.ink)
                  : AppTypography.bodySans(fontSize: 12, color: colors.ink),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTabBar(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return Container(
      color: colors.containerLow,
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space12,
        vertical: AppTokens.space8,
      ),
      child: Row(
        children: [
          _buildTabButton(context, 0, 'Overview', Icons.info_outline_rounded),
          const SizedBox(width: AppTokens.space8),
          _buildTabButton(context, 1, 'Highlights & Bookmarks', Icons.bookmark_outline_rounded),
          const SizedBox(width: AppTokens.space8),
          _buildTabButton(context, 2, 'Chat with Book', Icons.auto_awesome_rounded),
        ],
      ),
    );
  }

  Widget _buildTabButton(BuildContext context, int index, String label, IconData icon) {
    final colors = ShelfdColors.of(context);
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
            color: isSelected ? colors.surface : Colors.transparent,
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: Border.all(
              color: isSelected ? colors.border : Colors.transparent,
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                icon,
                size: 16,
                color: isSelected ? colors.accentPrimary : colors.mutedInk,
              ),
              const SizedBox(width: AppTokens.space8),
              Flexible(
                child: Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
                    color: isSelected ? colors.ink : colors.mutedInk,
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

  List<BookFile> _resolveBookFiles(Book book) {
    if (book.files.isNotEmpty) {
      return book.files;
    }
    final List<BookFile> list = [];
    if (book.fileSizeBytes != null || !book.isAudiobook) {
      list.add(BookFile(
        id: '${book.id}_epub',
        bookId: book.id,
        fileType: 'epub',
        filePath: '${book.title}.epub',
        fileSizeBytes: book.fileSizeBytes ?? 1840000,
        mimeType: 'application/epub+zip',
      ));
    }
    if (_hasAudiobook(book)) {
      list.add(BookFile(
        id: '${book.id}_audio',
        bookId: book.id,
        fileType: 'audiobook',
        filePath: '${book.title}.m4a',
        fileSizeBytes: (book.bitrateKbps != null && book.durationSeconds != null)
            ? ((book.bitrateKbps! * 1000 / 8) * book.durationSeconds!).round()
            : 256000000,
        durationSeconds: book.durationSeconds ?? 41200,
        bitrateKbps: book.bitrateKbps ?? 64,
        mimeType: 'audio/mp4',
      ));
    }
    if (book.coverUrl != null) {
      list.add(BookFile(
        id: '${book.id}_cover',
        bookId: book.id,
        fileType: 'cover',
        filePath: 'cover.jpg',
        fileSizeBytes: 380 * 1024,
        mimeType: 'image/jpeg',
      ));
    }
    return list;
  }

  Widget _buildAssociatedFilesSection(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
    final files = _resolveBookFiles(book);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Expanded(
              child: Text(
                'Associated Formats & Files',
                style: AppTypography.titleSerif(
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
            ),
            const SizedBox(width: AppTokens.space8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              decoration: BoxDecoration(
                color: colors.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                border: Border.all(color: colors.subtleBorder),
              ),
              child: Text(
                '${files.length} files linked',
                style: AppTypography.mono(fontSize: 11, color: colors.mutedInk),
              ),
            ),
          ],
        ),
        const SizedBox(height: AppTokens.space12),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: files.length,
          separatorBuilder: (context, index) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final f = files[index];
            final isAudio = f.fileType == 'audiobook' || f.filePath.endsWith('.m4a') || f.filePath.endsWith('.m4b') || f.filePath.endsWith('.mp3');
            final isEpub = f.fileType == 'epub' || f.filePath.endsWith('.epub');
            final isPdf = f.fileType == 'pdf' || f.filePath.endsWith('.pdf');

            Color badgeColor;
            Color badgeTextColor;
            IconData iconData;
            String typeLabel;

            if (isAudio) {
              badgeColor = AppTokens.amberAccent.withValues(alpha: colors.isDark ? 0.2 : 0.22);
              badgeTextColor = colors.isDark ? AppTokens.amberAccent : const Color(0xFFB45309);
              iconData = Icons.headphones;
              typeLabel = 'M4A AUDIOBOOK';
            } else if (isEpub) {
              badgeColor = colors.accentPrimary.withValues(alpha: 0.14);
              badgeTextColor = colors.accentPrimary;
              iconData = Icons.menu_book;
              typeLabel = 'EPUB';
            } else if (isPdf) {
              badgeColor = colors.errorBg;
              badgeTextColor = colors.errorText;
              iconData = Icons.picture_as_pdf;
              typeLabel = 'PDF';
            } else {
              badgeColor = colors.accentSecondaryContainer;
              badgeTextColor = colors.accentSecondary;
              iconData = Icons.image;
              typeLabel = 'IMAGE';
            }

            return Container(
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: colors.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: Border.all(color: colors.subtleBorder),
              ),
              child: Row(
                children: [
                  Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      color: badgeColor,
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    ),
                    child: Icon(iconData, color: badgeTextColor, size: 20),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                              decoration: BoxDecoration(
                                color: badgeColor,
                                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              ),
                              child: Text(
                                typeLabel,
                                style: AppTypography.mono(
                                  fontSize: 9,
                                  fontWeight: FontWeight.w700,
                                  color: badgeTextColor,
                                ),
                              ),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                f.filePath,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.mono(
                                  fontSize: 12,
                                  fontWeight: FontWeight.w600,
                                  color: colors.ink,
                                ),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 3),
                        Text(
                          isAudio
                              ? '${_formatFileSize(f.fileSizeBytes)} • ${_formatAudioDuration(f.durationSeconds ?? book.durationSeconds)} • ${f.bitrateKbps ?? book.bitrateKbps ?? 64} kbps AAC'
                              : isEpub
                                  ? '${_formatFileSize(f.fileSizeBytes)} • ${book.isFixedLayout ? 'Fixed-Layout EPUB3' : 'Reflowable EPUB3'}'
                                  : isPdf
                                      ? '${_formatFileSize(f.fileSizeBytes)} • Fixed Layout PDF'
                                      : '${_formatFileSize(f.fileSizeBytes)} • 2:3 Cover Artwork',
                          style: AppTypography.mono(fontSize: 11, color: colors.mutedInk),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  if (isAudio)
                    FilledButton(
                      key: Key('listen_file_button_$index'),
                      style: FilledButton.styleFrom(
                        backgroundColor: AppTokens.amberAccent,
                        foregroundColor: AppTokens.espressoInk,
                        visualDensity: VisualDensity.compact,
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                      ),
                      onPressed: () => context.go('/books/${book.id}/listen'),
                      child: const Text('Listen', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700)),
                    )
                  else if (isEpub)
                    FilledButton(
                      key: Key('read_file_button_$index'),
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.accentPrimary,
                        foregroundColor: colors.onAccent,
                        visualDensity: VisualDensity.compact,
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                      ),
                      onPressed: () => context.go('/books/${book.id}/read'),
                      child: const Text('Read', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
                    )
                  else
                    OutlinedButton(
                      style: OutlinedButton.styleFrom(
                        foregroundColor: colors.ink,
                        side: BorderSide(color: colors.border),
                        visualDensity: VisualDensity.compact,
                        padding: const EdgeInsets.symmetric(horizontal: 10),
                      ),
                      onPressed: () => _downloadEpub(book),
                      child: const Text('Export', style: TextStyle(fontSize: 11)),
                    ),
                ],
              ),
            );
          },
        ),
      ],
    );
  }

  Widget _buildOverviewTab(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Synopsis Section
          Text(
            'Synopsis',
            style: AppTypography.titleSerif(
              fontSize: 18,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          HtmlText(
            html: book.synopsis,
            style: AppTypography.bodySans(fontSize: 15, lineHeight: 1.6, color: colors.ink),
            emptyPlaceholder: 'No synopsis provided for this book.',
          ),
          const SizedBox(height: AppTokens.space24),
          Divider(height: 1, color: colors.subtleBorder),
          const SizedBox(height: AppTokens.space24),

          // Associated Formats & Files Section
          _buildAssociatedFilesSection(context, book),
          const SizedBox(height: AppTokens.space32),
          Divider(height: 1, color: colors.subtleBorder),
          const SizedBox(height: AppTokens.space24),

          // Table of Contents Spine
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Table of Contents',
                style: AppTypography.titleSerif(
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
              Text(
                book.spine.isNotEmpty
                    ? (book.isFixedLayout
                        ? (book.namedSpine.isNotEmpty
                            ? '${book.namedSpine.length} chapters (${book.spine.length} pages)'
                            : '${book.spine.length} pages')
                        : '${book.spine.length} items')
                    : book.audioChapters.isNotEmpty
                        ? '${book.audioChapters.length} tracks'
                        : '0 items',
                style: AppTypography.mono(fontSize: 12, color: colors.mutedInk),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),
          if ((book.isFixedLayout ? book.namedSpine.isEmpty : book.spine.isEmpty) && book.audioChapters.isEmpty)
            Text(
              'No chapters cataloged.',
              style: AppTypography.bodySans(fontSize: 14, color: colors.mutedInk),
            )
          else if (book.isFixedLayout ? book.namedSpine.isNotEmpty : book.spine.isNotEmpty)
            () {
              final displaySpine = book.isFixedLayout ? book.namedSpine : book.spine;
              return ListView.separated(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: displaySpine.length,
                separatorBuilder: (context, index) =>
                    Divider(height: 1, color: colors.subtleBorder),
                itemBuilder: (context, index) {
                  final item = displaySpine[index];
                  return ListTile(
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space8,
                      vertical: AppTokens.space4,
                    ),
                    title: Text(
                      item.title.isNotEmpty ? item.title : 'Chapter ${item.chapterIndex}',
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                        color: colors.ink,
                      ),
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
                                color: colors.secondaryInk,
                              ),
                            ),
                          )
                        : null,
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (book.isFixedLayout)
                          Text(
                            'Page ${item.chapterIndex}',
                            style: AppTypography.mono(fontSize: 12, color: colors.mutedInk),
                          ),
                        const SizedBox(width: AppTokens.space8),
                        Icon(
                          Icons.chevron_right_rounded,
                          size: 20,
                          color: colors.mutedInk,
                        ),
                      ],
                    ),
                    onTap: () {
                      context.go('/books/${book.id}/read/${item.chapterIndex}');
                    },
                  );
                },
              );
            }()
          else
            ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: book.audioChapters.length,
              separatorBuilder: (context, index) =>
                  Divider(height: 1, color: colors.subtleBorder),
              itemBuilder: (context, index) {
                final item = book.audioChapters[index];
                return ListTile(
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space8,
                    vertical: AppTokens.space4,
                  ),
                  leading: Icon(Icons.headphones_rounded, size: 18, color: colors.accentPrimary),
                  title: Text(
                    item.title,
                    style: AppTypography.bodySans(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      color: colors.ink,
                    ),
                  ),
                  subtitle: Text(
                    _formatAudioDuration(item.durationSec),
                    style: AppTypography.mono(fontSize: 12, color: colors.mutedInk),
                  ),
                  trailing: Icon(
                    Icons.play_arrow_rounded,
                    size: 20,
                    color: colors.mutedInk,
                  ),
                  onTap: () {
                    context.go('/books/${book.id}/listen');
                  },
                );
              },
            ),
        ],
      ),
    );
  }

  Widget _buildHighlightsBookmarksTab(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
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
              _buildAnnotationFilterChip(colors, 'all', 'All (${highlights.length + bookmarks.length})'),
              const SizedBox(width: AppTokens.space8),
              _buildAnnotationFilterChip(colors, 'highlights', 'Highlights (${highlights.length})'),
              const SizedBox(width: AppTokens.space8),
              _buildAnnotationFilterChip(colors, 'bookmarks', 'Bookmarks (${bookmarks.length})'),
              const Spacer(),
              IconButton(
                icon: Icon(Icons.bookmark_add_outlined, color: colors.ink),
                tooltip: 'Add Bookmark',
                onPressed: () => _showAddBookmarkDialog(context, book),
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
                    Icon(
                      _annotationFilter == 'highlights'
                          ? Icons.format_quote_rounded
                          : Icons.bookmarks_outlined,
                      size: 44,
                      color: colors.mutedInk,
                    ),
                    const SizedBox(height: AppTokens.space12),
                    Text(
                      _annotationFilter == 'highlights'
                          ? 'No highlights yet'
                          : 'No annotations found',
                      style: AppTypography.titleSerif(fontSize: 16, color: colors.ink),
                    ),
                    const SizedBox(height: AppTokens.space4),
                    Text(
                      _annotationFilter == 'highlights'
                          ? 'Select text while reading in the reader view to create highlights and add personal notes.'
                          : 'Save your favorite passages or bookmarks as you read.',
                      textAlign: TextAlign.center,
                      style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
                    ),
                    if (_annotationFilter == 'highlights') ...[
                      const SizedBox(height: AppTokens.space16),
                      OutlinedButton.icon(
                        style: OutlinedButton.styleFrom(
                          foregroundColor: colors.ink,
                          side: BorderSide(color: colors.border),
                        ),
                        icon: const Icon(Icons.menu_book_rounded, size: 16),
                        label: const Text('Start Reading'),
                        onPressed: () => context.go('/books/${widget.bookId}/read'),
                      ),
                    ],
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
                  return _buildHighlightCard(context, item);
                } else if (item is Bookmark) {
                  return _buildBookmarkCard(context, item);
                }
                return const SizedBox.shrink();
              },
            ),
        ],
      ),
    );
  }

  Widget _buildAnnotationFilterChip(ShelfdColors colors, String key, String label) {
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
      selectedColor: colors.accentContainer,
      backgroundColor: colors.containerLow,
      showCheckmark: false,
      side: BorderSide(
        color: isSelected ? colors.accentPrimary : colors.subtleBorder,
      ),
      labelStyle: AppTypography.bodySans(
        fontSize: 12,
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
        color: isSelected ? colors.accentPrimary : colors.secondaryInk,
      ),
    );
  }

  Widget _buildHighlightCard(BuildContext context, Highlight hl) {
    final colors = ShelfdColors.of(context);
    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      onTap: () {
        if (hl.chapterId != null && hl.chapterId!.isNotEmpty) {
          context.go('/books/${widget.bookId}/read/${hl.chapterId}');
        } else {
          context.go('/books/${widget.bookId}/read');
        }
      },
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 4,
            height: 54,
            decoration: BoxDecoration(
              color: _highlightColor(hl.color, isDark: colors.isDark),
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
                    color: colors.ink,
                  ),
                ),
                if (hl.note != null && hl.note!.isNotEmpty) ...[
                  const SizedBox(height: AppTokens.space8),
                  Text(
                    hl.note!,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.secondaryInk,
                    ),
                  ),
                ],
                const SizedBox(height: AppTokens.space8),
                Text(
                  hl.createdAt != null
                      ? 'Highlighted on ${hl.createdAt!.month}/${hl.createdAt!.day}/${hl.createdAt!.year}'
                      : 'Highlighted quote',
                  style: AppTypography.mono(fontSize: 11, color: colors.mutedInk),
                ),
              ],
            ),
          ),
          IconButton(
            icon: Icon(Icons.delete_outline_rounded, size: 20, color: colors.mutedInk),
            tooltip: 'Delete Highlight',
            onPressed: () {
              ref.read(bookDetailProvider(widget.bookId).notifier).removeHighlight(hl.id);
            },
          ),
        ],
      ),
    );
  }

  Widget _buildBookmarkCard(BuildContext context, Bookmark bm) {
    final colors = ShelfdColors.of(context);
    final percent = (bm.progress * 100).toInt();

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      onTap: () {
        if (bm.chapterId != null && bm.chapterId!.isNotEmpty) {
          context.go('/books/${widget.bookId}/read/${bm.chapterId}');
        } else {
          context.go('/books/${widget.bookId}/read');
        }
      },
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Icon(Icons.bookmark_rounded, color: colors.accentPrimary, size: 24),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  bm.title,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  '$percent% completed',
                  style: AppTypography.mono(fontSize: 11, color: colors.mutedInk),
                ),
              ],
            ),
          ),
          IconButton(
            icon: Icon(Icons.delete_outline_rounded, size: 20, color: colors.mutedInk),
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
    final colors = ShelfdColors.of(context);
    final chatState = ref.watch(bookChatProvider(widget.bookId));

    return Column(
      children: [
        // RAG Header info banner
        Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppTokens.space20,
            vertical: AppTokens.space12,
          ),
          decoration: BoxDecoration(
            color: colors.containerLow,
            border: Border(bottom: BorderSide(color: colors.subtleBorder)),
          ),
          child: Row(
            children: [
              Icon(Icons.auto_awesome_rounded, size: 18, color: colors.accentPrimary),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Connected to Shelfd MCP tools: search passages and read chapters on demand',
                  style: AppTypography.captionSans(fontSize: 12, color: colors.secondaryInk),
                ),
              ),
              if (chatState.messages.isNotEmpty)
                TextButton(
                  style: TextButton.styleFrom(foregroundColor: colors.accentPrimary),
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
              ? _buildEmptyChatView(context, book)
              : ListView.builder(
                  controller: _chatScrollController,
                  padding: const EdgeInsets.all(AppTokens.space16),
                  itemCount: chatState.messages.length + (chatState.isSending ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == chatState.messages.length) {
                      return _buildTypingIndicator(context);
                    }
                    final msg = chatState.messages[index];
                    return _buildChatMessageBubble(context, msg);
                  },
                ),
        ),

        // Error message if any
        if (chatState.error != null)
          Container(
            padding: const EdgeInsets.all(AppTokens.space8),
            color: colors.errorBg,
            child: Row(
              children: [
                Icon(Icons.error_outline, size: 16, color: colors.errorText),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    chatState.error!,
                    style: AppTypography.bodySans(fontSize: 12, color: colors.errorText),
                  ),
                ),
                TextButton(
                  onPressed: () {
                    ref.read(bookChatProvider(widget.bookId).notifier).retry();
                  },
                  child: Text(
                    'Retry',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      color: colors.errorText,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ],
            ),
          ),

        // Bottom text input bar
        Container(
          padding: const EdgeInsets.all(AppTokens.space12),
          decoration: BoxDecoration(
            color: colors.surface,
            border: Border(top: BorderSide(color: colors.subtleBorder)),
          ),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _chatController,
                  onSubmitted: (_) => _sendMessage(),
                  style: AppTypography.bodySans(fontSize: 13, color: colors.ink),
                  decoration: InputDecoration(
                    hintText: 'Ask about characters, themes, or plot points...',
                    hintStyle: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.mutedInk,
                    ),
                    filled: true,
                    fillColor: colors.containerLow,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space16,
                      vertical: AppTokens.space12,
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      borderSide: BorderSide(color: colors.border),
                    ),
                    enabledBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      borderSide: BorderSide(color: colors.border),
                    ),
                    focusedBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                    ),
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              IconButton.filled(
                style: IconButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
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

  Widget _buildEmptyChatView(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppTokens.space24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              padding: const EdgeInsets.all(AppTokens.space16),
              decoration: BoxDecoration(
                color: colors.accentContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.auto_awesome_rounded,
                size: 36,
                color: colors.accentPrimary,
              ),
            ),
            const SizedBox(height: AppTokens.space16),
            Text(
              'Ask anything about ${book.title}',
              textAlign: TextAlign.center,
              style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              'Shelfd uses semantic search to cite relevant chapters and answer questions.',
              textAlign: TextAlign.center,
              style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
            ),
            const SizedBox(height: AppTokens.space24),
            Wrap(
              spacing: AppTokens.space8,
              runSpacing: AppTokens.space8,
              alignment: WrapAlignment.center,
              children: [
                _buildPromptChip(context, 'Summarize the major themes'),
                _buildPromptChip(context, 'Who are the main characters?'),
                _buildPromptChip(context, 'What happens in the opening chapter?'),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPromptChip(BuildContext context, String prompt) {
    final colors = ShelfdColors.of(context);
    return ActionChip(
      label: Text(prompt),
      backgroundColor: colors.containerLow,
      side: BorderSide(color: colors.border),
      labelStyle: AppTypography.bodySans(fontSize: 12, color: colors.ink),
      onPressed: () => _sendMessage(prompt),
    );
  }

  Widget _buildTypingIndicator(BuildContext context) {
    final colors = ShelfdColors.of(context);
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
              color: colors.containerLow,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: Border.all(color: colors.subtleBorder),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
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
                Text(
                  'Agent consulting library & retrieving passages...',
                  style: AppTypography.captionSans(fontSize: 12, color: colors.secondaryInk),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildChatMessageBubble(BuildContext context, BookChatMessage msg) {
    final colors = ShelfdColors.of(context);
    final isUser = msg.role == 'user';
    final textColor = isUser ? colors.onAccent : colors.ink;
    final secondaryTextColor =
        isUser ? colors.onAccent.withValues(alpha: 0.8) : colors.secondaryInk;
    final codeBgColor = isUser
        ? colors.onAccent.withValues(alpha: 0.15)
        : colors.ink.withValues(alpha: 0.08);

    final markdownStyleSheet = MarkdownStyleSheet(
      p: AppTypography.bodySans(
        fontSize: 14,
        lineHeight: 1.5,
        color: textColor,
      ),
      pPadding: EdgeInsets.zero,
      h1: AppTypography.titleSerif(
        fontSize: 18,
        fontWeight: FontWeight.bold,
        height: 1.3,
        color: textColor,
      ),
      h1Padding: const EdgeInsets.only(top: AppTokens.space8),
      h2: AppTypography.titleSerif(
        fontSize: 16,
        fontWeight: FontWeight.bold,
        height: 1.35,
        color: textColor,
      ),
      h2Padding: const EdgeInsets.only(top: AppTokens.space8),
      h3: AppTypography.titleSerif(
        fontSize: 15,
        fontWeight: FontWeight.w600,
        height: 1.4,
        color: textColor,
      ),
      h3Padding: const EdgeInsets.only(top: AppTokens.space4),
      h4: AppTypography.bodySans(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        lineHeight: 1.4,
        color: textColor,
      ),
      h4Padding: const EdgeInsets.only(top: AppTokens.space4),
      strong: TextStyle(
        fontWeight: FontWeight.bold,
        color: textColor,
      ),
      em: TextStyle(
        fontStyle: FontStyle.italic,
        color: textColor,
      ),
      code: AppTypography.mono(
        fontSize: 13,
        color: textColor,
      ).copyWith(backgroundColor: codeBgColor),
      codeblockPadding: const EdgeInsets.all(AppTokens.space8),
      codeblockDecoration: BoxDecoration(
        color: isUser ? colors.onAccent.withValues(alpha: 0.1) : colors.surface,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(
          color: isUser ? colors.onAccent.withValues(alpha: 0.24) : colors.border,
        ),
      ),
      blockquote: AppTypography.bodySans(
        fontSize: 14,
        fontStyle: FontStyle.italic,
        color: secondaryTextColor,
      ),
      blockquotePadding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space12,
        vertical: AppTokens.space4,
      ),
      blockquoteDecoration: BoxDecoration(
        border: Border(
          left: BorderSide(
            color: isUser ? colors.onAccent.withValues(alpha: 0.4) : colors.accentPrimary,
            width: 3,
          ),
        ),
      ),
      listBullet: AppTypography.bodySans(
        fontSize: 14,
        lineHeight: 1.5,
        color: textColor,
      ),
      listBulletPadding: const EdgeInsets.only(right: 6),
      listIndent: 20.0,
      blockSpacing: AppTokens.space8,
      a: TextStyle(
        color: isUser ? colors.onAccent : colors.accentPrimary,
        decoration: TextDecoration.underline,
      ),
    );

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
              decoration: BoxDecoration(
                color: colors.accentContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(Icons.auto_awesome, size: 14, color: colors.accentPrimary),
            ),
          ],
          Flexible(
            child: Container(
              padding: const EdgeInsets.all(AppTokens.space16),
              decoration: BoxDecoration(
                color: isUser ? colors.accentPrimary : colors.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: isUser ? null : Border.all(color: colors.subtleBorder),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  MarkdownBody(
                    data: msg.content.trim(),
                    styleSheet: markdownStyleSheet,
                    shrinkWrap: true,
                  ),
                  if (!isUser && msg.citations.isNotEmpty) ...[
                    const SizedBox(height: AppTokens.space12),
                    Wrap(
                      spacing: AppTokens.space8,
                      runSpacing: AppTokens.space8,
                      children: [
                        for (final cit in msg.citations)
                          ActionChip(
                            avatar: Icon(Icons.menu_book, size: 14, color: colors.accentPrimary),
                            label: Text(
                              _formatCitationLabel(cit),
                              style: AppTypography.mono(fontSize: 11, color: colors.ink),
                            ),
                            backgroundColor: colors.surface,
                            side: BorderSide(color: colors.border),
                            onPressed: () {
                              _showCitationDetails(context, cit);
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

  String _formatCitationLabel(BookCitation cit) {
    final title = cit.chapterTitle != null && cit.chapterTitle!.isNotEmpty
        ? 'Ch. ${cit.chapterIndex}: ${cit.chapterTitle}'
        : 'Chapter ${cit.chapterIndex}';
    if (cit.startParagraph != null) {
      final para = cit.startParagraph == cit.endParagraph
          ? '¶${cit.startParagraph}'
          : '¶¶${cit.startParagraph}–${cit.endParagraph}';
      return '$title ($para)';
    }
    return title;
  }

  void _showCitationDetails(BuildContext context, BookCitation cit) {
    final colors = ShelfdColors.of(context);
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: colors.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (bottomSheetContext) {
        final sheetColors = ShelfdColors.of(bottomSheetContext);
        final title = cit.chapterTitle != null && cit.chapterTitle!.isNotEmpty
            ? cit.chapterTitle!
            : 'Chapter ${cit.chapterIndex}';
        final paraLabel = cit.startParagraph != null
            ? (cit.startParagraph == cit.endParagraph
                ? 'Paragraph ${cit.startParagraph}'
                : 'Paragraphs ${cit.startParagraph}–${cit.endParagraph}')
            : null;

        final previewText = (cit.excerpt != null && cit.excerpt!.isNotEmpty)
            ? cit.excerpt!
            : cit.summary;

        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(AppTokens.space20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.menu_book, size: 20, color: sheetColors.accentPrimary),
                    const SizedBox(width: AppTokens.space8),
                    Expanded(
                      child: Text(
                        title,
                        style: AppTypography.titleSerif(fontSize: 16, color: sheetColors.ink),
                      ),
                    ),
                    IconButton(
                      icon: Icon(Icons.close, size: 18, color: sheetColors.mutedInk),
                      onPressed: () => Navigator.of(bottomSheetContext).pop(),
                    ),
                  ],
                ),
                if (paraLabel != null) ...[
                  const SizedBox(height: AppTokens.space4),
                  Text(
                    paraLabel,
                    style: AppTypography.mono(fontSize: 12, color: sheetColors.mutedInk),
                  ),
                ],
                const SizedBox(height: AppTokens.space12),
                if (previewText.isNotEmpty) ...[
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(AppTokens.space12),
                    decoration: BoxDecoration(
                      color: sheetColors.containerLow,
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      border: Border.all(color: sheetColors.subtleBorder),
                    ),
                    child: Text(
                      previewText,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        lineHeight: 1.5,
                        color: sheetColors.ink,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppTokens.space16),
                ],
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    style: FilledButton.styleFrom(
                      backgroundColor: sheetColors.accentPrimary,
                      foregroundColor: sheetColors.onAccent,
                      padding: const EdgeInsets.symmetric(vertical: AppTokens.space12),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      ),
                    ),
                    icon: const Icon(Icons.auto_stories, size: 16),
                    label: const Text('Read in Reader'),
                    onPressed: () {
                      Navigator.of(bottomSheetContext).pop();
                      context.go('/books/${widget.bookId}/read/${cit.chapterIndex}');
                    },
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
