import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/chapter.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class ReaderView extends ConsumerStatefulWidget {
  final String bookId;
  final int chapterIndex;
  final Chapter? initialChapter;

  const ReaderView({
    super.key,
    required this.bookId,
    required this.chapterIndex,
    this.initialChapter,
  });

  @override
  ConsumerState<ReaderView> createState() => _ReaderViewState();
}

class _ReaderViewState extends ConsumerState<ReaderView> {
  Chapter? _currentChapter;
  bool _isLoading = false;
  String? _error;
  int _activeChapterIndex = 0;

  @override
  void initState() {
    super.initState();
    _activeChapterIndex = widget.chapterIndex;
    if (widget.initialChapter != null) {
      _currentChapter = widget.initialChapter;
    } else {
      _loadChapter(_activeChapterIndex);
    }
  }

  Future<void> _loadChapter(int index) async {
    setState(() {
      _isLoading = true;
      _error = null;
      _activeChapterIndex = index;
    });

    final readerRepo = ref.read(readerRepositoryProvider);
    try {
      final ch = await readerRepo.loadChapter(widget.bookId, index);
      if (mounted) {
        setState(() {
          _currentChapter = ch;
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

    final paragraphs = (_currentChapter?.content ?? '')
        .split('\n\n')
        .map((p) => p.trim())
        .where((p) => p.isNotEmpty)
        .toList();

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
                context.go('/library');
              }
            },
          ),
          title: Text(
            _currentChapter?.title ?? 'Chapter $_activeChapterIndex',
            style: AppTypography.titleSerif(
              fontSize: 16,
              color: theme.colorScheme.primary,
            ),
          ),
          actions: [
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
                  child: ListView.builder(
                    itemCount: 20, // Supported chapters range
                    itemBuilder: (context, idx) {
                      final isSelected = idx == _activeChapterIndex;
                      return ListTile(
                        selected: isSelected,
                        selectedTileColor: AppTokens.boneContainer,
                        title: Text(
                          'Chapter $idx',
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
                          Navigator.of(context).pop(); // close drawer
                          _loadChapter(idx);
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
                  onPressed: _activeChapterIndex > 0
                      ? () => _loadChapter(_activeChapterIndex - 1)
                      : null,
                ),
                Text(
                  'Chapter $_activeChapterIndex • 8 mins left',
                  style: AppTypography.bodySans(
                    fontSize: 12,
                    color: AppTokens.mutedCopy,
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.chevron_right_rounded),
                  onPressed: () => _loadChapter(_activeChapterIndex + 1),
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
                            onPressed: () => _loadChapter(_activeChapterIndex),
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
                        itemCount: paragraphs.length,
                        itemBuilder: (context, idx) {
                          return Padding(
                            padding: const EdgeInsets.only(bottom: AppTokens.space16),
                            child: Text(
                              paragraphs[idx],
                              style: AppTypography.readerText(
                                fontSize: settings.fontSize,
                                lineHeight: settings.lineHeight,
                                isSerif: settings.isSerif,
                                color: theme.colorScheme.onSurface,
                              ),
                            ),
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
