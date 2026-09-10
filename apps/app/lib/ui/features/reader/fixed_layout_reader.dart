import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'html_view.dart';

enum SpreadMode {
  auto,
  twoPage,
  singlePage,
}

class FixedSpread {
  final int spreadIndex;
  final SpineItem? leftPage;
  final SpineItem rightPage;
  final bool isSingle;
  final bool isGatefold;

  const FixedSpread({
    required this.spreadIndex,
    this.leftPage,
    required this.rightPage,
    required this.isSingle,
    this.isGatefold = false,
  });

  String get label {
    if (isSingle) {
      final idx = rightPage.chapterIndex > 0 ? rightPage.chapterIndex : spreadIndex + 1;
      return 'Page $idx';
    }
    final leftIdx = leftPage!.chapterIndex > 0 ? leftPage!.chapterIndex : spreadIndex * 2;
    final rightIdx = rightPage.chapterIndex > 0 ? rightPage.chapterIndex : leftIdx + 1;
    return 'Pages $leftIdx–$rightIdx';
  }

  bool containsChapter(dynamic identifier) {
    if (identifier == null) return false;
    final idStr = identifier.toString();
    if (rightPage.id == idStr || rightPage.chapterIndex.toString() == idStr) return true;
    if (leftPage != null && (leftPage!.id == idStr || leftPage!.chapterIndex.toString() == idStr)) return true;
    return false;
  }
}

class FixedLayoutReader extends ConsumerStatefulWidget {
  final Book book;
  final dynamic initialChapterIdentifier;
  final VoidCallback? onClose;

  const FixedLayoutReader({
    super.key,
    required this.book,
    this.initialChapterIdentifier,
    this.onClose,
  });

  @override
  ConsumerState<FixedLayoutReader> createState() => _FixedLayoutReaderState();
}

class _FixedLayoutReaderState extends ConsumerState<FixedLayoutReader> {
  SpreadMode _spreadMode = SpreadMode.auto;
  int _currentSpreadIndex = 0;
  final FocusNode _focusNode = FocusNode();
  bool _controlsVisible = true;

  @override
  void initState() {
    super.initState();
    _resolveInitialSpread();
  }

  void _toggleControls() {
    setState(() {
      _controlsVisible = !_controlsVisible;
    });
  }

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  void _resolveInitialSpread() {
    final spine = widget.book.spine;
    if (spine.isEmpty) return;

    final spreads = _getSpreads(allowTwoPage: true);
    if (widget.initialChapterIdentifier != null) {
      final matchIdx = spreads.indexWhere((s) => s.containsChapter(widget.initialChapterIdentifier));
      if (matchIdx >= 0) {
        _currentSpreadIndex = matchIdx;
        return;
      }
    }
    _currentSpreadIndex = 0;
  }

  List<FixedSpread> _getSpreads({required bool allowTwoPage}) {
    final spine = widget.book.spine;
    if (spine.isEmpty) return [];

    if (!allowTwoPage) {
      return List.generate(
        spine.length,
        (i) => FixedSpread(
          spreadIndex: i,
          rightPage: spine[i],
          isSingle: true,
          isGatefold: spine[i].isGatefold,
        ),
      );
    }

    final spreads = <FixedSpread>[];
    // Cover is always standalone recto (Page 1 / Index 0)
    spreads.add(FixedSpread(
      spreadIndex: 0,
      rightPage: spine[0],
      isSingle: true,
      isGatefold: spine[0].isGatefold,
    ));

    int i = 1;
    while (i < spine.length) {
      final current = spine[i];
      if (current.isGatefold) {
        spreads.add(FixedSpread(
          spreadIndex: spreads.length,
          rightPage: current,
          isSingle: true,
          isGatefold: true,
        ));
        i++;
        continue;
      }

      if (i + 1 < spine.length) {
        final next = spine[i + 1];
        if (next.isGatefold) {
          spreads.add(FixedSpread(
            spreadIndex: spreads.length,
            rightPage: current,
            isSingle: true,
          ));
          i++;
        } else {
          spreads.add(FixedSpread(
            spreadIndex: spreads.length,
            leftPage: current,
            rightPage: next,
            isSingle: false,
          ));
          i += 2;
        }
      } else {
        spreads.add(FixedSpread(
          spreadIndex: spreads.length,
          rightPage: current,
          isSingle: true,
        ));
        i++;
      }
    }

    return spreads;
  }

  void _goToSpread(int index, int totalSpreads) {
    if (index >= 0 && index < totalSpreads) {
      setState(() {
        _currentSpreadIndex = index;
      });
      _saveProgress(index, totalSpreads);
    }
  }

  void _saveProgress(int index, int totalSpreads) {
    if (totalSpreads <= 0) return;
    final progress = (index + 1) / totalSpreads;
    ref.read(readerRepositoryProvider).updateProgress(widget.book.id, progress);
  }

  String _buildPageUrl(SpineItem page) {
    final apiService = ref.read(apiServiceProvider);
    final baseUrl = apiService.baseUrl;
    final cleanBase = (baseUrl.isNotEmpty && baseUrl.endsWith('/'))
        ? baseUrl.substring(0, baseUrl.length - 1)
        : baseUrl;
    final identifier = page.id.isNotEmpty ? page.id : page.chapterIndex;
    if (cleanBase.isNotEmpty) {
      return '$cleanBase/api/v1/books/${widget.book.id}/chapters/$identifier/html?fit=scale';
    }
    return '/api/v1/books/${widget.book.id}/chapters/$identifier/html?fit=scale';
  }

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;

    final spreads = _currentSpreads;
    if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
      if (_currentSpreadIndex > 0) {
        _goToSpread(_currentSpreadIndex - 1, spreads.length);
        return KeyEventResult.handled;
      }
    } else if (event.logicalKey == LogicalKeyboardKey.arrowRight ||
        event.logicalKey == LogicalKeyboardKey.space) {
      if (_currentSpreadIndex < spreads.length - 1) {
        _goToSpread(_currentSpreadIndex + 1, spreads.length);
        return KeyEventResult.handled;
      }
    }
    return KeyEventResult.ignored;
  }

  List<FixedSpread> _currentSpreads = const [];

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final screenWidth = constraints.maxWidth;
        final screenHeight = constraints.maxHeight;

        final isWide = screenWidth >= 900 && screenWidth > screenHeight;
        final bool allowTwoPage;
        switch (_spreadMode) {
          case SpreadMode.auto:
            allowTwoPage = isWide;
            break;
          case SpreadMode.twoPage:
            allowTwoPage = true;
            break;
          case SpreadMode.singlePage:
            allowTwoPage = false;
            break;
        }

        final spreads = _getSpreads(allowTwoPage: allowTwoPage);
        _currentSpreads = spreads;

        if (_currentSpreadIndex >= spreads.length) {
          _currentSpreadIndex = max(0, spreads.length - 1);
        }

        final currentSpread = spreads.isNotEmpty ? spreads[_currentSpreadIndex] : null;
        final canPrev = _currentSpreadIndex > 0;
        final canNext = _currentSpreadIndex < spreads.length - 1;

        return Focus(
          focusNode: _focusNode,
          autofocus: true,
          onKeyEvent: _handleKeyEvent,
          child: Scaffold(
            backgroundColor: const Color(0xFF141416),
            body: SafeArea(
              child: Stack(
                children: [
                  // Stage / Reading Viewport
                  Positioned.fill(
                    child: currentSpread == null
                        ? const Center(
                            child: Text('No pages found', style: TextStyle(color: Colors.white70)),
                          )
                        : GestureDetector(
                            behavior: HitTestBehavior.translucent,
                            onTap: _toggleControls,
                            child: _buildSpreadStage(
                              context,
                              currentSpread,
                              screenWidth,
                              screenHeight - (_controlsVisible ? 120 : 0),
                            ),
                          ),
                  ),

                  // Left & Right Edge Chevrons
                  if (canPrev)
                    Positioned(
                      left: 16,
                      top: 0,
                      bottom: 0,
                      child: Center(
                        child: Material(
                          color: Colors.black45,
                          shape: const CircleBorder(),
                          clipBehavior: Clip.antiAlias,
                          child: IconButton(
                            iconSize: 28,
                            padding: const EdgeInsets.all(12),
                            icon: const Icon(Icons.chevron_left_rounded, color: Colors.white),
                            tooltip: 'Previous Page',
                            onPressed: () => _goToSpread(_currentSpreadIndex - 1, spreads.length),
                          ),
                        ),
                      ),
                    ),
                  if (canNext)
                    Positioned(
                      right: 16,
                      top: 0,
                      bottom: 0,
                      child: Center(
                        child: Material(
                          color: Colors.black45,
                          shape: const CircleBorder(),
                          clipBehavior: Clip.antiAlias,
                          child: IconButton(
                            iconSize: 28,
                            padding: const EdgeInsets.all(12),
                            icon: const Icon(Icons.chevron_right_rounded, color: Colors.white),
                            tooltip: 'Next Page',
                            onPressed: () => _goToSpread(_currentSpreadIndex + 1, spreads.length),
                          ),
                        ),
                      ),
                    ),

                  // Top Header Bar
                  AnimatedPositioned(
                    duration: const Duration(milliseconds: 200),
                    top: _controlsVisible ? 0 : -70,
                    left: 0,
                    right: 0,
                    child: _buildTopBar(context, currentSpread, spreads.length),
                  ),

                  // Bottom Navigation Bar
                  AnimatedPositioned(
                    duration: const Duration(milliseconds: 200),
                    bottom: _controlsVisible ? 0 : -80,
                    left: 0,
                    right: 0,
                    child: _buildBottomBar(context, currentSpread, spreads.length),
                  ),
                ],
              ),
            ),
            endDrawer: _buildTocDrawer(context, spreads),
          ),
        );
      },
    );
  }

  Widget _buildTopBar(BuildContext context, FixedSpread? spread, int totalSpreads) {
    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: const Color(0xFF1E1E22).withValues(alpha: 0.95),
        border: const Border(bottom: BorderSide(color: Color(0xFF2E2E34))),
      ),
      child: Row(
        children: [
          IconButton(
            icon: const Icon(Icons.arrow_back_rounded, color: Colors.white),
            tooltip: 'Back to Library',
            onPressed: () {
              if (widget.onClose != null) {
                widget.onClose!();
              } else if (context.canPop()) {
                context.pop();
              } else {
                context.go('/books/${widget.book.id}');
              }
            },
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.book.title,
                  style: AppTypography.titleSerif(fontSize: 15, color: Colors.white),
                  overflow: TextOverflow.ellipsis,
                ),
                if (spread != null)
                  Text(
                    '${spread.label} of ${widget.book.spine.length}',
                    style: AppTypography.bodySans(fontSize: 12, color: Colors.white60),
                  ),
              ],
            ),
          ),

          // Spread Mode Toggle
          IconButton(
            icon: Icon(
              _spreadMode == SpreadMode.auto
                  ? Icons.auto_stories_rounded
                  : _spreadMode == SpreadMode.twoPage
                      ? Icons.menu_book_rounded
                      : Icons.crop_portrait_rounded,
              color: Colors.white70,
            ),
            tooltip: _spreadMode == SpreadMode.auto
                ? 'Spread Mode: Auto (Responsive)'
                : _spreadMode == SpreadMode.twoPage
                    ? 'Spread Mode: Two Pages (Facing)'
                    : 'Spread Mode: Single Page',
            onPressed: () {
              setState(() {
                if (_spreadMode == SpreadMode.auto) {
                  _spreadMode = SpreadMode.twoPage;
                } else if (_spreadMode == SpreadMode.twoPage) {
                  _spreadMode = SpreadMode.singlePage;
                } else {
                  _spreadMode = SpreadMode.auto;
                }
              });
            },
          ),

          // Bookmark Button
          IconButton(
            icon: const Icon(Icons.bookmark_add_outlined, color: Colors.white70),
            tooltip: 'Bookmark Page',
            onPressed: () {
              if (spread == null) return;
              final title = spread.label;
              final progress = (spread.spreadIndex + 1) / totalSpreads;
              ref.read(bookDetailProvider(widget.book.id).notifier).addBookmark(
                    title: title,
                    progress: progress,
                    chapterId: spread.rightPage.id,
                  );
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text('Bookmarked "$title"'),
                  duration: const Duration(seconds: 2),
                  behavior: SnackBarBehavior.floating,
                ),
              );
            },
          ),

          // Table of Contents
          Builder(
            builder: (ctx) => IconButton(
              icon: const Icon(Icons.list_rounded, color: Colors.white70),
              tooltip: 'Table of Contents',
              onPressed: () => Scaffold.of(ctx).openEndDrawer(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBottomBar(BuildContext context, FixedSpread? spread, int totalSpreads) {
    final canPrev = _currentSpreadIndex > 0;
    final canNext = _currentSpreadIndex < totalSpreads - 1;

    return Container(
      height: 64,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: const Color(0xFF1E1E22).withValues(alpha: 0.95),
        border: const Border(top: BorderSide(color: Color(0xFF2E2E34))),
      ),
      child: Row(
        children: [
          IconButton(
            icon: const Icon(Icons.chevron_left_rounded, color: Colors.white),
            tooltip: 'Previous',
            onPressed: canPrev ? () => _goToSpread(_currentSpreadIndex - 1, totalSpreads) : null,
          ),
          Expanded(
            child: SliderTheme(
              data: SliderTheme.of(context).copyWith(
                activeTrackColor: Colors.white70,
                inactiveTrackColor: Colors.white24,
                thumbColor: Colors.white,
                trackHeight: 3,
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 6),
              ),
              child: Slider(
                value: _currentSpreadIndex.toDouble(),
                min: 0,
                max: max(0, totalSpreads - 1).toDouble(),
                divisions: totalSpreads > 1 ? totalSpreads - 1 : 1,
                onChanged: (val) => _goToSpread(val.round(), totalSpreads),
              ),
            ),
          ),
          IconButton(
            icon: const Icon(Icons.chevron_right_rounded, color: Colors.white),
            tooltip: 'Next',
            onPressed: canNext ? () => _goToSpread(_currentSpreadIndex + 1, totalSpreads) : null,
          ),
          const SizedBox(width: 8),
          Text(
            spread != null ? spread.label : '',
            style: AppTypography.bodySans(fontSize: 13, color: Colors.white70),
          ),
        ],
      ),
    );
  }

  Widget _buildSpreadStage(
    BuildContext context,
    FixedSpread spread,
    double availWidth,
    double availHeight,
  ) {
    if (spread.isSingle || spread.leftPage == null) {
      // Single Page (Cover, Gatefold, or Single-page mode)
      final page = spread.rightPage;
      final pw = page.pageWidth ?? 1000.0;
      final ph = page.pageHeight ?? 1000.0;

      final stageW = availWidth - 80;
      final stageH = availHeight - 40;
      final scale = min(stageW / pw, stageH / ph).clamp(0.05, 2.0);

      final displayW = pw * scale;
      final displayH = ph * scale;
      final viewKey = 'fixed_${widget.book.id}_${page.chapterIndex}';
      final url = _buildPageUrl(page);

      return Center(
        child: Container(
          width: displayW,
          height: displayH,
          decoration: BoxDecoration(
            color: Colors.black,
            boxShadow: const [
              BoxShadow(
                color: Colors.black87,
                blurRadius: 24,
                spreadRadius: 4,
                offset: Offset(0, 8),
              ),
            ],
            borderRadius: BorderRadius.circular(4),
          ),
          clipBehavior: Clip.antiAlias,
          child: buildHtmlFrame(
            viewKey: viewKey,
            url: url,
            width: displayW,
            height: displayH,
          ),
        ),
      );
    } else {
      // Two-Page Facing Spread [leftPage, rightPage]
      final leftPage = spread.leftPage!;
      final rightPage = spread.rightPage;

      final leftW = leftPage.pageWidth ?? 1000.0;
      final leftH = leftPage.pageHeight ?? 1000.0;
      final rightW = rightPage.pageWidth ?? 1000.0;
      final rightH = rightPage.pageHeight ?? 1000.0;

      final combinedW = leftW + rightW;
      final combinedH = max(leftH, rightH);

      final stageW = availWidth - 80;
      final stageH = availHeight - 40;
      final scale = min(stageW / combinedW, stageH / combinedH).clamp(0.05, 2.0);

      final leftDisplayW = leftW * scale;
      final rightDisplayW = rightW * scale;
      final displayH = combinedH * scale;

      final leftViewKey = 'fixed_${widget.book.id}_${leftPage.chapterIndex}';
      final rightViewKey = 'fixed_${widget.book.id}_${rightPage.chapterIndex}';
      final leftUrl = _buildPageUrl(leftPage);
      final rightUrl = _buildPageUrl(rightPage);

      return Center(
        child: Container(
          decoration: BoxDecoration(
            boxShadow: const [
              BoxShadow(
                color: Colors.black87,
                blurRadius: 28,
                spreadRadius: 6,
                offset: Offset(0, 10),
              ),
            ],
            borderRadius: BorderRadius.circular(4),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Left Page (Verso)
              Container(
                width: leftDisplayW,
                height: displayH,
                decoration: const BoxDecoration(
                  color: Colors.black,
                  borderRadius: BorderRadius.horizontal(left: Radius.circular(4)),
                ),
                clipBehavior: Clip.antiAlias,
                child: buildHtmlFrame(
                  viewKey: leftViewKey,
                  url: leftUrl,
                  width: leftDisplayW,
                  height: displayH,
                ),
              ),

              // Open-book Spine Crease / Gutter
              Container(
                width: 2,
                height: displayH,
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    colors: [
                      Colors.black54,
                      Colors.black12,
                      Colors.black54,
                    ],
                  ),
                ),
              ),

              // Right Page (Recto)
              Container(
                width: rightDisplayW,
                height: displayH,
                decoration: const BoxDecoration(
                  color: Colors.black,
                  borderRadius: BorderRadius.horizontal(right: Radius.circular(4)),
                ),
                clipBehavior: Clip.antiAlias,
                child: buildHtmlFrame(
                  viewKey: rightViewKey,
                  url: rightUrl,
                  width: rightDisplayW,
                  height: displayH,
                ),
              ),
            ],
          ),
        ),
      );
    }
  }

  Widget _buildTocDrawer(BuildContext context, List<FixedSpread> spreads) {
    final spine = widget.book.spine;

    return Drawer(
      backgroundColor: const Color(0xFF1E1E22),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.all(AppTokens.space20),
              child: Text(
                'Pages & Chapters',
                style: AppTypography.titleSerif(fontSize: 20, color: Colors.white),
              ),
            ),
            const Divider(color: Color(0xFF2E2E34), height: 1),
            Expanded(
              child: ListView.builder(
                itemCount: spine.length,
                itemBuilder: (context, idx) {
                  final item = spine[idx];
                  final matchingSpreadIdx = spreads.indexWhere((s) => s.containsChapter(item.id));
                  final isCurrent = matchingSpreadIdx == _currentSpreadIndex;

                  return ListTile(
                    selected: isCurrent,
                    selectedTileColor: const Color(0xFF2A2A32),
                    title: Text(
                      item.title.isNotEmpty ? item.title : 'Page ${item.chapterIndex}',
                      style: isCurrent
                          ? AppTypography.titleSerif(fontSize: 14, color: Colors.white)
                          : AppTypography.bodySans(fontSize: 14, color: Colors.white70),
                    ),
                    subtitle: Text(
                      item.isGatefold ? 'Gatefold Spread' : (item.pageSpread ?? 'Standard Page'),
                      style: const TextStyle(fontSize: 11, color: Colors.white38),
                    ),
                    trailing: isCurrent
                        ? const Icon(Icons.check_rounded, size: 18, color: Colors.white)
                        : null,
                    onTap: () {
                      Navigator.of(context).pop();
                      if (matchingSpreadIdx >= 0) {
                        _goToSpread(matchingSpreadIdx, spreads.length);
                      }
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
