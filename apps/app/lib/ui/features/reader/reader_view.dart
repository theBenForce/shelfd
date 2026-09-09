import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/chapter.dart';
import '../../../data/models/highlight.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'reader_markdown.dart';

class ReaderView extends ConsumerStatefulWidget {
  final String bookId;
  final int? chapterIndex;
  final dynamic chapterIdentifier;
  final Chapter? initialChapter;

  const ReaderView({
    super.key,
    required this.bookId,
    this.chapterIndex,
    this.chapterIdentifier,
    this.initialChapter,
  });

  @override
  ConsumerState<ReaderView> createState() => _ReaderViewState();
}

class _ReaderViewState extends ConsumerState<ReaderView> {
  Chapter? _currentChapter;
  List<SpineItem> _spine = const [];
  bool _isLoading = false;
  String? _error;
  dynamic _activeIdentifier;
  String? _selectedText;

  int _getSpineIndex(List<SpineItem> spine) {
    if (spine.isEmpty) return -1;
    return spine.indexWhere((s) =>
        s.id == _currentChapter?.id ||
        (s.id.isNotEmpty && s.id == _activeIdentifier?.toString()) ||
        (s.chapterIndex > 0 && s.chapterIndex == _currentChapter?.chapterIndex));
  }

  @override
  void initState() {
    super.initState();
    if (widget.initialChapter != null) {
      _currentChapter = widget.initialChapter;
      _activeIdentifier = widget.initialChapter!.id;
    } else {
      _activeIdentifier = widget.chapterIdentifier ?? widget.chapterIndex ?? 1;
    }
    _initBookAndChapter();
  }

  Future<void> _initBookAndChapter() async {
    // Refresh book detail for latest highlights and annotations (cross-device sync)
    ref.read(bookDetailProvider(widget.bookId).notifier).loadBook();

    final bookRepo = ref.read(bookRepositoryProvider);
    try {
      final book = await bookRepo.getBookDetail(widget.bookId);
      if (mounted && book.spine.isNotEmpty) {
        setState(() {
          _spine = book.spine;
        });
      }
    } catch (_) {}

    if (_currentChapter == null) {
      final bookDetail = ref.read(bookDetailProvider(widget.bookId)).book;
      final availableSpine = _spine.isNotEmpty ? _spine : (bookDetail?.spine ?? const []);
      final target = _activeIdentifier ?? (availableSpine.isNotEmpty ? availableSpine.first.id : 1);
      _loadChapter(target);
    }
  }

  void _createHighlight({
    required String selectedText,
    required KindleHighlightColor color,
    required List<ReaderBlock> blocks,
    String? note,
  }) {
    final range = findHighlightRange(
      blocks: blocks,
      fullContent: _currentChapter?.content ?? '',
      selectedText: selectedText,
    );

    ref.read(bookDetailProvider(widget.bookId).notifier).addHighlight(
          selectedText: selectedText.trim(),
          color: color.name,
          note: note,
          chapterId: _currentChapter?.id,
          startOffset: range?.startOffset,
          endOffset: range?.endOffset,
          startParagraph: range?.startParagraph,
          endParagraph: range?.endParagraph,
          location: range != null ? 'p.${range.startParagraph}:${range.startOffset}' : null,
        );

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(note != null && note.isNotEmpty ? 'Note & highlight saved' : 'Highlight saved'),
        duration: const Duration(seconds: 2),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  void _showAddNoteSheet({
    required String selectedText,
    required List<ReaderBlock> blocks,
  }) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).cardColor,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => _AddNoteSheet(
        selectedText: selectedText,
        onSave: (color, note) {
          Navigator.of(ctx).pop();
          _createHighlight(
            selectedText: selectedText,
            color: color,
            blocks: blocks,
            note: note,
          );
        },
      ),
    );
  }

  void _showExistingHighlightSheet(Highlight highlight) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).cardColor,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => _ExistingHighlightSheet(
        highlight: highlight,
        onDelete: () {
          Navigator.of(ctx).pop();
          ref.read(bookDetailProvider(widget.bookId).notifier).removeHighlight(highlight.id);
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Highlight deleted'),
              duration: Duration(seconds: 2),
              behavior: SnackBarBehavior.floating,
            ),
          );
        },
      ),
    );
  }

  Future<void> _loadChapter(dynamic identifier) async {
    setState(() {
      _isLoading = true;
      _error = null;
      _activeIdentifier = identifier;
    });

    final readerRepo = ref.read(readerRepositoryProvider);
    try {
      final ch = await readerRepo.loadChapter(widget.bookId, identifier);
      if (mounted) {
        setState(() {
          _currentChapter = ch;
          _activeIdentifier = ch.id.isNotEmpty ? ch.id : identifier;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  void _showTypographySheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).cardColor,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => const _TypographySheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final settings = ref.watch(readerSettingsProvider);
    final theme = AppTheme.buildTheme(settings.themeMode);
    final bookDetailState = ref.watch(bookDetailProvider(widget.bookId));
    final effectiveSpine = _spine.isNotEmpty ? _spine : (bookDetailState.book?.spine ?? const []);
    final spineIdx = _getSpineIndex(effectiveSpine);

    final chapterHighlights = (bookDetailState.book?.highlights ?? const []).where((h) {
      if (_currentChapter == null) return false;
      if (h.chapterId != null && h.chapterId!.isNotEmpty) {
        return h.chapterId == _currentChapter!.id;
      }
      return false;
    }).toList();

    final blocks = parseReaderBlocks(_currentChapter?.content ?? '');

    final canPrev = effectiveSpine.isNotEmpty
        ? spineIdx > 0
        : (_currentChapter != null ? _currentChapter!.chapterIndex > 1 : false);
    final canNext = effectiveSpine.isNotEmpty
        ? (spineIdx >= 0 && spineIdx < effectiveSpine.length - 1)
        : true;

    VoidCallback? onPrev;
    if (canPrev) {
      onPrev = () {
        if (effectiveSpine.isNotEmpty && spineIdx > 0) {
          _loadChapter(effectiveSpine[spineIdx - 1].id);
        } else if (_currentChapter != null && _currentChapter!.chapterIndex > 1) {
          _loadChapter(_currentChapter!.chapterIndex - 1);
        }
      };
    }

    VoidCallback? onNext;
    if (canNext) {
      onNext = () {
        if (effectiveSpine.isNotEmpty && spineIdx >= 0 && spineIdx < effectiveSpine.length - 1) {
          _loadChapter(effectiveSpine[spineIdx + 1].id);
        } else if (_currentChapter != null) {
          _loadChapter(_currentChapter!.chapterIndex + 1);
        }
      };
    }

    String? resolvedTitle;
    if (_currentChapter?.title != null) {
      final t = _currentChapter!.title.trim();
      final isSyntheticId = t.isEmpty ||
          RegExp(r'^Chapter\s+([0-9A-HJKMNP-TV-Z]{26}|[0-9a-fA-F-]{36})$', caseSensitive: false).hasMatch(t) ||
          RegExp(r'^([0-9A-HJKMNP-TV-Z]{26}|[0-9a-fA-F-]{36})$', caseSensitive: false).hasMatch(t);
      if (!isSyntheticId) {
        resolvedTitle = t;
      }
    }

    final spineTitle = effectiveSpine.isNotEmpty && spineIdx >= 0 ? effectiveSpine[spineIdx].title : null;
    final displayTitle = resolvedTitle ??
        spineTitle ??
        (_currentChapter != null && _currentChapter!.chapterIndex > 0
            ? 'Chapter ${_currentChapter!.chapterIndex}'
            : 'Reader');

    return Theme(
      data: theme,
      child: Scaffold(
        backgroundColor: theme.scaffoldBackgroundColor,
        appBar: AppBar(
          backgroundColor: theme.scaffoldBackgroundColor,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back_rounded),
            onPressed: () {
              if (context.canPop()) {
                context.pop();
              } else {
                context.go('/reader/${widget.bookId}');
              }
            },
          ),
          title: Text(
            displayTitle,
            style: AppTypography.titleSerif(
              fontSize: 16,
              color: theme.colorScheme.primary,
            ),
          ),
          actions: [
            IconButton(
              icon: const Icon(Icons.bookmark_add_outlined),
              tooltip: 'Bookmark Location',
              onPressed: () {
                final title = displayTitle;
                final spineProgress = effectiveSpine.isNotEmpty && spineIdx >= 0
                    ? (spineIdx + 1) / effectiveSpine.length
                    : 0.0;
                ref.read(bookDetailProvider(widget.bookId).notifier).addBookmark(
                      title: title,
                      progress: spineProgress,
                      chapterId: _currentChapter?.id,
                    );
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text('Bookmarked "$title"'),
                    duration: const Duration(seconds: 2),
                  ),
                );
              },
            ),
            IconButton(
              icon: const Icon(Icons.text_fields_rounded),
              tooltip: 'Typography Settings',
              onPressed: _showTypographySheet,
            ),
            Builder(
              builder: (innerContext) => IconButton(
                icon: const Icon(Icons.list_rounded),
                tooltip: 'Table of Contents',
                onPressed: () => Scaffold.of(innerContext).openEndDrawer(),
              ),
            ),
          ],
          bottom: PreferredSize(
            preferredSize: const Size.fromHeight(1),
            child: Container(color: theme.dividerColor, height: 1),
          ),
        ),
        endDrawer: Drawer(
          backgroundColor: theme.scaffoldBackgroundColor,
          child: SafeArea(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: const EdgeInsets.all(AppTokens.space20),
                  child: Text(
                    'Table of Contents',
                    style: AppTypography.titleSerif(
                      fontSize: 20,
                      color: theme.colorScheme.primary,
                    ),
                  ),
                ),
                const Divider(color: AppTokens.crispBorder, height: 1),
                Expanded(
                  child: effectiveSpine.isNotEmpty
                      ? ListView.builder(
                          itemCount: effectiveSpine.length,
                          itemBuilder: (context, idx) {
                            final item = effectiveSpine[idx];
                            final isSelected = idx == spineIdx;
                            return ListTile(
                              selected: isSelected,
                              selectedTileColor: AppTokens.boneContainer,
                              title: Text(
                                item.title,
                                style: isSelected
                                    ? AppTypography.titleSerif(
                                        fontSize: 15,
                                        color: theme.colorScheme.primary,
                                      )
                                    : AppTypography.bodySans(
                                        fontSize: 14,
                                        color: theme.colorScheme.onSurface,
                                      ),
                              ),
                              onTap: () {
                                Navigator.of(context).pop();
                                _loadChapter(item.id);
                              },
                            );
                          },
                        )
                      : ListView.builder(
                          itemCount: 20,
                          itemBuilder: (context, idx) {
                            final chapterNum = idx + 1;
                            final isSelected = chapterNum == (_currentChapter?.chapterIndex ?? 1);
                            return ListTile(
                              selected: isSelected,
                              selectedTileColor: AppTokens.boneContainer,
                              title: Text(
                                'Chapter $chapterNum',
                                style: isSelected
                                    ? AppTypography.titleSerif(
                                        fontSize: 15,
                                        color: theme.colorScheme.primary,
                                      )
                                    : AppTypography.bodySans(
                                        fontSize: 14,
                                        color: theme.colorScheme.onSurface,
                                      ),
                              ),
                              onTap: () {
                                Navigator.of(context).pop();
                                _loadChapter(chapterNum);
                              },
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
        bottomNavigationBar: Container(
          decoration: BoxDecoration(
            color: theme.scaffoldBackgroundColor,
            border: Border(top: BorderSide(color: theme.dividerColor)),
          ),
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space20, vertical: 8),
          child: SafeArea(
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                IconButton(
                  icon: const Icon(Icons.chevron_left_rounded),
                  onPressed: onPrev,
                ),
                Expanded(
                  child: Text(
                    '$displayTitle • 8 mins left',
                    textAlign: TextAlign.center,
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      color: AppTokens.mutedCopy,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.chevron_right_rounded),
                  onPressed: onNext,
                ),
              ],
            ),
          ),
        ),
        body: _isLoading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(
                    child: Padding(
                      padding: const EdgeInsets.all(AppTokens.space24),
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          const Icon(Icons.error_outline, size: 36, color: Color(0xFFC92A2A)),
                          const SizedBox(height: AppTokens.space12),
                          Text('Failed to load chapter', style: AppTypography.titleSerif(fontSize: 18)),
                          const SizedBox(height: AppTokens.space8),
                          Text(_error!, textAlign: TextAlign.center, style: AppTypography.bodySans(fontSize: 13)),
                          const SizedBox(height: AppTokens.space16),
                          ElevatedButton(
                            onPressed: () => _loadChapter(_activeIdentifier ?? 1),
                            child: const Text('Retry'),
                          ),
                        ],
                      ),
                    ),
                  )
                : Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: AppTokens.maxReadingWidth),
                      child: SelectionArea(
                        onSelectionChanged: (content) {
                          _selectedText = content?.plainText;
                        },
                        contextMenuBuilder: (context, selectableRegionState) {
                          final text = _selectedText;
                          if (text == null || text.trim().isEmpty) {
                            return const SizedBox.shrink();
                          }
                          return KindleSelectionToolbar(
                            anchors: selectableRegionState.contextMenuAnchors,
                            onColorSelected: (color) {
                              selectableRegionState.hideToolbar();
                              _createHighlight(
                                selectedText: text,
                                color: color,
                                blocks: blocks,
                              );
                            },
                            onAddNote: () {
                              selectableRegionState.hideToolbar();
                              _showAddNoteSheet(
                                selectedText: text,
                                blocks: blocks,
                              );
                            },
                            onCopy: () {
                              Clipboard.setData(ClipboardData(text: text));
                              selectableRegionState.hideToolbar();
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text('Copied to clipboard'),
                                  duration: Duration(seconds: 1),
                                  behavior: SnackBarBehavior.floating,
                                ),
                              );
                            },
                          );
                        },
                        child: ListView.builder(
                          padding: EdgeInsets.symmetric(
                            horizontal: Responsive.horizontalPadding(context),
                            vertical: AppTokens.space24,
                          ),
                          itemCount: blocks.length,
                          itemBuilder: (context, idx) {
                            return buildReaderBlockWidget(
                              block: blocks[idx],
                              settings: settings,
                              theme: theme,
                              highlights: chapterHighlights,
                              onHighlightTap: _showExistingHighlightSheet,
                            );
                          },
                        ),
                      ),
                    ),
                  ),
      ),
    );
  }
}

class _TypographySheet extends ConsumerWidget {
  const _TypographySheet();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = ref.watch(readerSettingsProvider);
    final notifier = ref.read(readerSettingsProvider.notifier);

    final themePills = const [
      FilterPillItem(id: 'bone', label: 'Bone'),
      FilterPillItem(id: 'sepia', label: 'Sepia'),
      FilterPillItem(id: 'dark', label: 'Dark OLED'),
    ];

    final fontPills = const [
      FilterPillItem(id: 'serif', label: 'Serif'),
      FilterPillItem(id: 'sans', label: 'Sans'),
    ];

    return Padding(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Reading Typography', style: AppTypography.titleSerif(fontSize: 18)),
          const SizedBox(height: AppTokens.space20),

          // Theme Selector (DRY FilterPillsRow)
          Text('Theme Palette', style: AppTypography.labelCaps()),
          const SizedBox(height: AppTokens.space8),
          FilterPillsRow(
            items: themePills,
            selectedId: settings.themeMode.name,
            onSelected: (id) {
              final mode = ReadingThemeMode.values.firstWhere((m) => m.name == id);
              notifier.setThemeMode(mode);
            },
          ),
          const SizedBox(height: AppTokens.space20),

          // Font Family Selector
          Text('Font Style', style: AppTypography.labelCaps()),
          const SizedBox(height: AppTokens.space8),
          FilterPillsRow(
            items: fontPills,
            selectedId: settings.isSerif ? 'serif' : 'sans',
            onSelected: (id) => notifier.setFontFamily(id == 'serif'),
          ),
          const SizedBox(height: AppTokens.space20),

          // Font Size Slider
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('Font Size', style: AppTypography.labelCaps()),
              Text('${settings.fontSize.round()} pt', style: AppTypography.bodySans(fontSize: 13)),
            ],
          ),
          Row(
            children: [
              const Text('A', style: TextStyle(fontSize: 13, fontWeight: FontWeight.bold)),
              Expanded(
                child: Slider(
                  value: settings.fontSize,
                  min: 14.0,
                  max: 26.0,
                  divisions: 6,
                  onChanged: (val) => notifier.setFontSize(val),
                ),
              ),
              const Text('A', style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
            ],
          ),
        ],
      ),
    );
  }
}

class KindleSelectionToolbar extends StatelessWidget {
  final TextSelectionToolbarAnchors anchors;
  final ValueChanged<KindleHighlightColor> onColorSelected;
  final VoidCallback onAddNote;
  final VoidCallback onCopy;

  const KindleSelectionToolbar({
    super.key,
    required this.anchors,
    required this.onColorSelected,
    required this.onAddNote,
    required this.onCopy,
  });

  @override
  Widget build(BuildContext context) {
    return AdaptiveTextSelectionToolbar(
      anchors: anchors,
      children: [
        for (final color in KindleHighlightColor.values)
          _ColorButton(
            color: color,
            onPressed: () => onColorSelected(color),
          ),
        Container(
          width: 1,
          height: 20,
          margin: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
          color: Theme.of(context).dividerColor,
        ),
        IconButton(
          icon: const Icon(Icons.edit_note_rounded, size: 20),
          tooltip: 'Add Note',
          padding: const EdgeInsets.symmetric(horizontal: 8),
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          onPressed: onAddNote,
        ),
        IconButton(
          icon: const Icon(Icons.copy_rounded, size: 18),
          tooltip: 'Copy',
          padding: const EdgeInsets.symmetric(horizontal: 8),
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          onPressed: onCopy,
        ),
      ],
    );
  }
}

class _ColorButton extends StatelessWidget {
  final KindleHighlightColor color;
  final VoidCallback onPressed;

  const _ColorButton({
    required this.color,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: '${color.label} Highlight',
      child: InkWell(
        onTap: onPressed,
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(8),
          child: Container(
            width: 22,
            height: 22,
            decoration: BoxDecoration(
              color: color.cardColor,
              shape: BoxShape.circle,
              border: Border.all(
                color: Colors.black.withValues(alpha: 0.2),
                width: 1.5,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _AddNoteSheet extends StatefulWidget {
  final String selectedText;
  final void Function(KindleHighlightColor color, String? note) onSave;

  const _AddNoteSheet({
    required this.selectedText,
    required this.onSave,
  });

  @override
  State<_AddNoteSheet> createState() => _AddNoteSheetState();
}

class _AddNoteSheetState extends State<_AddNoteSheet> {
  late final TextEditingController _controller;
  KindleHighlightColor _selectedColor = KindleHighlightColor.yellow;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final bottomInset = MediaQuery.of(context).viewInsets.bottom;

    return Padding(
      padding: EdgeInsets.only(
        left: AppTokens.space24,
        right: AppTokens.space24,
        top: AppTokens.space20,
        bottom: AppTokens.space24 + bottomInset,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('Add Note', style: AppTypography.titleSerif(fontSize: 18)),
                IconButton(
                  icon: const Icon(Icons.close_rounded, size: 20),
                  onPressed: () => Navigator.of(context).pop(),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            // Quoted excerpt
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: Border(
                  left: BorderSide(
                    color: _selectedColor.darkColor,
                    width: 3.5,
                  ),
                ),
              ),
              child: Text(
                '“${widget.selectedText.trim()}”',
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  color: theme.colorScheme.onSurface,
                ).copyWith(fontStyle: FontStyle.italic),
              ),
            ),
            const SizedBox(height: AppTokens.space16),

            // Kindle Color Picker
            Text('Highlight Color', style: AppTypography.labelCaps()),
            const SizedBox(height: AppTokens.space8),
            Row(
              children: KindleHighlightColor.values.map((c) {
                final isSelected = c == _selectedColor;
                return GestureDetector(
                  onTap: () => setState(() => _selectedColor = c),
                  child: Container(
                    margin: const EdgeInsets.only(right: 12),
                    padding: const EdgeInsets.all(3),
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: isSelected ? theme.colorScheme.primary : Colors.transparent,
                        width: 2,
                      ),
                    ),
                    child: Container(
                      width: 26,
                      height: 26,
                      decoration: BoxDecoration(
                        color: c.cardColor,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: Colors.black.withValues(alpha: 0.2),
                          width: 1.5,
                        ),
                      ),
                      child: isSelected
                          ? Icon(Icons.check_rounded, size: 16, color: c.darkColor)
                          : null,
                    ),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: AppTokens.space16),

            // Note input
            TextField(
              controller: _controller,
              autofocus: true,
              maxLines: 3,
              style: AppTypography.bodySans(fontSize: 14),
              decoration: InputDecoration(
                hintText: 'Add a personal note or reflection...',
                hintStyle: AppTypography.bodySans(fontSize: 14, color: AppTokens.mutedCopy),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  borderSide: const BorderSide(color: AppTokens.crispBorder),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  borderSide: BorderSide(color: theme.colorScheme.primary, width: 1.5),
                ),
                contentPadding: const EdgeInsets.all(AppTokens.space12),
              ),
            ),
            const SizedBox(height: AppTokens.space20),

            // Action buttons
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('Cancel'),
                ),
                const SizedBox(width: AppTokens.space12),
                ElevatedButton.icon(
                  onPressed: () {
                    final note = _controller.text.trim();
                    widget.onSave(_selectedColor, note.isNotEmpty ? note : null);
                  },
                  icon: const Icon(Icons.check_rounded, size: 16),
                  label: const Text('Save Note'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ExistingHighlightSheet extends StatelessWidget {
  final Highlight highlight;
  final VoidCallback onDelete;

  const _ExistingHighlightSheet({
    required this.highlight,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = highlight.highlightColor;

    return Padding(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Container(
                    width: 14,
                    height: 14,
                    decoration: BoxDecoration(
                      color: color.cardColor,
                      shape: BoxShape.circle,
                      border: Border.all(color: Colors.black.withValues(alpha: 0.2)),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Text('Highlight', style: AppTypography.titleSerif(fontSize: 18)),
                ],
              ),
              IconButton(
                icon: const Icon(Icons.close_rounded, size: 20),
                onPressed: () => Navigator.of(context).pop(),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space12),

          // Quoted excerpt
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppTokens.space12),
            decoration: BoxDecoration(
              color: color.cardColor.withValues(alpha: 0.35),
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: Border(
                left: BorderSide(
                  color: color.darkColor,
                  width: 3.5,
                ),
              ),
            ),
            child: Text(
              '“${highlight.selectedText}”',
              style: AppTypography.bodySans(
                fontSize: 14,
                color: theme.colorScheme.onSurface,
              ).copyWith(fontStyle: FontStyle.italic),
            ),
          ),
          const SizedBox(height: AppTokens.space16),

          // Attached note
          if (highlight.note != null && highlight.note!.isNotEmpty) ...[
            Text('Note', style: AppTypography.labelCaps()),
            const SizedBox(height: AppTokens.space8),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: Border.all(color: AppTokens.crispBorder),
              ),
              child: Text(
                highlight.note!,
                style: AppTypography.bodySans(fontSize: 14),
              ),
            ),
            const SizedBox(height: AppTokens.space16),
          ],

          // Location badge
          if (highlight.location != null) ...[
            Text(
              'Location: ${highlight.location}',
              style: AppTypography.labelCaps(color: AppTokens.mutedCopy),
            ),
            const SizedBox(height: AppTokens.space16),
          ],

          // Delete action
          Align(
            alignment: Alignment.centerRight,
            child: TextButton.icon(
              onPressed: onDelete,
              icon: const Icon(Icons.delete_outline_rounded, size: 18, color: Color(0xFFC92A2A)),
              label: const Text('Delete Highlight', style: TextStyle(color: Color(0xFFC92A2A))),
            ),
          ),
        ],
      ),
    );
  }
}

