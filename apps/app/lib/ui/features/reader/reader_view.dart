import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/chapter.dart';
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

  int get _currentSpineIndex {
    if (_spine.isEmpty) return -1;
    return _spine.indexWhere((s) =>
        s.id == _currentChapter?.id ||
        (s.id.isNotEmpty && s.id == _activeIdentifier?.toString()) ||
        s.chapterIndex == _currentChapter?.chapterIndex);
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
      final target = _activeIdentifier ?? (_spine.isNotEmpty ? _spine.first.id : 1);
      _loadChapter(target);
    }
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
    final spineIdx = _currentSpineIndex;

    final blocks = parseReaderBlocks(_currentChapter?.content ?? '');

    final canPrev = _spine.isNotEmpty
        ? spineIdx > 0
        : (_currentChapter != null ? _currentChapter!.chapterIndex > 1 : false);
    final canNext = _spine.isNotEmpty
        ? (spineIdx >= 0 && spineIdx < _spine.length - 1)
        : true;

    VoidCallback? onPrev;
    if (canPrev) {
      onPrev = () {
        if (_spine.isNotEmpty && spineIdx > 0) {
          _loadChapter(_spine[spineIdx - 1].id);
        } else if (_currentChapter != null && _currentChapter!.chapterIndex > 1) {
          _loadChapter(_currentChapter!.chapterIndex - 1);
        }
      };
    }

    VoidCallback? onNext;
    if (canNext) {
      onNext = () {
        if (_spine.isNotEmpty && spineIdx >= 0 && spineIdx < _spine.length - 1) {
          _loadChapter(_spine[spineIdx + 1].id);
        } else if (_currentChapter != null) {
          _loadChapter(_currentChapter!.chapterIndex + 1);
        }
      };
    }

    final displayTitle = _currentChapter?.title ??
        (_spine.isNotEmpty && spineIdx >= 0 ? _spine[spineIdx].title : 'Reader');

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
                final spineProgress = _spine.isNotEmpty && spineIdx >= 0
                    ? (spineIdx + 1) / _spine.length
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
                  child: _spine.isNotEmpty
                      ? ListView.builder(
                          itemCount: _spine.length,
                          itemBuilder: (context, idx) {
                            final item = _spine[idx];
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
                          );
                        },
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
