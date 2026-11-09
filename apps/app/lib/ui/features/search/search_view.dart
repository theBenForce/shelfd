import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/search_result.dart';
import '../../core/app_shell.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class SearchView extends ConsumerStatefulWidget {
  const SearchView({super.key});

  @override
  ConsumerState<SearchView> createState() => _SearchViewState();
}

class _SearchViewState extends ConsumerState<SearchView> {
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onNavTapped(int index) {
    if (index == 0) {
      context.go('/books');
    } else if (index == 1) {
      context.go('/series');
    } else if (index == 2) {
      context.go('/authors');
    } else if (index == 3) {
      // already on search
    } else if (index == 4) {
      context.go('/settings');
    }
  }

  void _onSearchSubmitted(String query) {
    ref.read(searchProvider.notifier).search(query);
  }

  @override
  Widget build(BuildContext context) {
    final searchState = ref.watch(searchProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);
    final colors = context.shelfdColors;

    final hasAppShell = context.findAncestorWidgetOfExactType<AppShell>() != null;

    final appBarWidget = isDesktop
        ? null
        : AppBar(
            backgroundColor: colors.background,
            foregroundColor: colors.ink,
            title: Text(
              'Semantic Search',
              style: AppTypography.titleSerif(fontSize: 20, color: colors.ink),
            ),
            leading: IconButton(
              icon: const Icon(Icons.arrow_back_rounded),
              onPressed: () {
                if (context.canPop()) {
                  context.pop();
                } else {
                  context.go('/books');
                }
              },
            ),
            bottom: PreferredSize(
              preferredSize: const Size.fromHeight(1),
              child: Container(color: colors.subtleBorder, height: 1),
            ),
          );

    final bodyContent = SafeArea(
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: AppTokens.maxSearchWidth),
          child: Column(
            children: [
              if (isDesktop)
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    horizontalPad,
                    AppTokens.space24,
                    horizontalPad,
                    AppTokens.space8,
                  ),
                  child: Row(
                    children: [
                      Text(
                        'Semantic Search',
                        style: AppTypography.displaySerif(
                          fontSize: 30,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                    ],
                  ),
                ),
              // Search Input Header
              Padding(
                padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space16),
                child: TextField(
                  controller: _searchController,
                  textInputAction: TextInputAction.search,
                  onSubmitted: _onSearchSubmitted,
                  style: AppTypography.bodySans(fontSize: 15, color: colors.ink),
                  decoration: InputDecoration(
                    hintText: 'Ask your library anything in natural language...',
                    hintStyle: AppTypography.bodySans(fontSize: 14, color: colors.mutedInk),
                    filled: true,
                    fillColor: colors.containerLow,
                    prefixIcon: Icon(
                      Icons.auto_awesome_rounded,
                      color: colors.accentPrimary,
                      size: 20,
                    ),
                    suffixIcon: IconButton(
                      icon: Icon(Icons.search_rounded, color: colors.secondaryInk),
                      onPressed: () => _onSearchSubmitted(_searchController.text),
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      borderSide: BorderSide(color: colors.border),
                    ),
                    enabledBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      borderSide: BorderSide(color: colors.border),
                    ),
                    focusedBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                    ),
                    contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                  ),
                ),
              ),
              Divider(color: colors.subtleBorder, height: 1),

              // Results / Empty State
              Expanded(
                child: searchState.isLoading
                    ? Center(child: CircularProgressIndicator(color: colors.accentPrimary))
                    : searchState.error != null
                        ? Center(
                            child: Container(
                              padding: const EdgeInsets.all(AppTokens.space16),
                              decoration: BoxDecoration(
                                color: colors.errorBg,
                                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                                border: Border.all(color: colors.errorBorder),
                              ),
                              child: Text(
                                searchState.error!,
                                style: AppTypography.bodySans(fontSize: 13, color: colors.errorText),
                              ),
                            ),
                          )
                        : searchState.results.isEmpty
                            ? Center(
                                child: Padding(
                                  padding: const EdgeInsets.all(AppTokens.space32),
                                  child: Column(
                                    mainAxisAlignment: MainAxisAlignment.center,
                                    children: [
                                      Icon(
                                        Icons.saved_search_rounded,
                                        size: 48,
                                        color: colors.mutedInk,
                                      ),
                                      const SizedBox(height: AppTokens.space16),
                                      Text(
                                        'Find passages by meaning',
                                        style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
                                      ),
                                      const SizedBox(height: AppTokens.space8),
                                      Text(
                                        'Try questions like "Where do they escape across the glacier?" or "When does Paul first ride a sandworm?"',
                                        textAlign: TextAlign.center,
                                        style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
                                      ),
                                    ],
                                  ),
                                ),
                              )
                            : ListView.builder(
                                padding: EdgeInsets.symmetric(
                                  horizontal: horizontalPad,
                                  vertical: AppTokens.space16,
                                ),
                                itemCount: searchState.results.length,
                                itemBuilder: (context, index) {
                                  final hit = searchState.results[index];
                                  return _SearchHitCard(
                                    hit: hit,
                                    onTap: () {
                                      final target = (hit.chapterId != null && hit.chapterId!.isNotEmpty)
                                          ? hit.chapterId!
                                          : hit.chapterIndex;
                                      context.go('/books/${hit.bookId}/$target');
                                    },
                                  );
                                },
                              ),
              ),
            ],
          ),
        ),
      ),
    );

    if (hasAppShell) {
      return Scaffold(
        backgroundColor: Colors.transparent,
        appBar: appBarWidget,
        body: bodyContent,
      );
    }

    return ShelfdAdaptiveScaffold(
      currentIndex: 3,
      onNavTap: _onNavTapped,
      currentPath: '/search',
      onNavigate: (path) => context.go(path),
      appBar: appBarWidget,
      body: bodyContent,
    );
  }
}

class _SearchHitCard extends StatelessWidget {
  final SemanticSearchHit hit;
  final VoidCallback onTap;

  const _SearchHitCard({required this.hit, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;

    return Padding(
      padding: const EdgeInsets.only(bottom: AppTokens.space16),
      child: BentoCard(
        onTap: onTap,
        padding: const EdgeInsets.all(AppTokens.space16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Book and Author Header
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 36,
                  height: 52,
                  child: TactileBookCover(
                    title: hit.bookTitle,
                    author: hit.authorName,
                    showFallbackTitle: false,
                    borderRadius: AppTokens.radiusSm,
                  ),
                ),
                const SizedBox(width: AppTokens.space12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        hit.bookTitle,
                        style: AppTypography.titleSerif(
                          fontSize: 17,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space4),
                      Text(
                        hit.authorName,
                        style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
                      ),
                    ],
                  ),
                ),
                StatusBadge(
                  label: '${hit.matchPercentage}% match',
                  backgroundColor: colors.isDark
                      ? colors.accentSecondaryContainer
                      : AppTokens.completedBadgeBg,
                  textColor: colors.accentSecondary,
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            // Chapter Badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: colors.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: colors.subtleBorder),
              ),
              child: Text(
                'Chapter ${hit.chapterIndex}: ${hit.chapterTitle}',
                style: AppTypography.mono(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: colors.secondaryInk,
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space8),

            // Summary Excerpt (<100 tokens)
            Container(
              padding: const EdgeInsets.only(left: AppTokens.space12),
              decoration: BoxDecoration(
                border: Border(
                  left: BorderSide(color: colors.accentPrimary.withValues(alpha: 0.5), width: 2),
                ),
              ),
              child: Text(
                hit.summary,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.readerBodySerif(
                  fontSize: 14,
                  lineHeight: 1.55,
                  color: colors.ink,
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space12),

            // Action: Read at passage
            InkWell(
              onTap: onTap,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'Read Chapter at this passage →',
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: colors.accentPrimary,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
