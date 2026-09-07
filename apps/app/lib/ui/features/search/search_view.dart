import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/search_result.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
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
  int _navIndex = 1;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onBottomNavTapped(int index) {
    setState(() => _navIndex = index);
    if (index == 0) {
      context.go('/library');
    } else if (index == 2) {
      context.go('/connect');
    }
  }

  void _onSearchSubmitted(String query) {
    ref.read(searchProvider.notifier).search(query);
  }

  @override
  Widget build(BuildContext context) {
    final searchState = ref.watch(searchProvider);
    final horizontalPad = Responsive.horizontalPadding(context);

    return Scaffold(
      bottomNavigationBar: ShelfdBottomNav(
        currentIndex: _navIndex,
        onTap: _onBottomNavTapped,
      ),
      appBar: AppBar(
        title: Text('Semantic Search', style: AppTypography.titleSerif(fontSize: 20)),
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
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(1),
          child: Container(color: AppTokens.crispBorder, height: 1),
        ),
      ),
      body: SafeArea(
        child: Column(
          children: [
            // Search Input Header
            Padding(
              padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space16),
              child: TextField(
                controller: _searchController,
                textInputAction: TextInputAction.search,
                onSubmitted: _onSearchSubmitted,
                decoration: InputDecoration(
                  hintText: 'Ask your library anything in natural language...',
                  hintStyle: AppTypography.bodySans(fontSize: 14, color: AppTokens.mutedCopy),
                  filled: true,
                  fillColor: AppTokens.boneSurface,
                  prefixIcon: const Icon(
                    Icons.auto_awesome_rounded,
                    color: Color(0xFF7C3AED),
                    size: 20,
                  ),
                  suffixIcon: IconButton(
                    icon: const Icon(Icons.search_rounded),
                    onPressed: () => _onSearchSubmitted(_searchController.text),
                  ),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    borderSide: const BorderSide(color: AppTokens.crispBorder),
                  ),
                  contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                ),
              ),
            ),
            const Divider(color: AppTokens.crispBorder, height: 1),

            // Results / Empty State
            Expanded(
              child: searchState.isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : searchState.error != null
                      ? Center(
                          child: Text(
                            searchState.error!,
                            style: const TextStyle(color: Color(0xFFC92A2A)),
                          ),
                        )
                      : searchState.results.isEmpty
                          ? Center(
                              child: Padding(
                                padding: const EdgeInsets.all(AppTokens.space32),
                                child: Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    const Icon(
                                      Icons.saved_search_rounded,
                                      size: 48,
                                      color: AppTokens.mutedCopy,
                                    ),
                                    const SizedBox(height: AppTokens.space16),
                                    Text(
                                      'Find passages by meaning',
                                      style: AppTypography.titleSerif(fontSize: 18),
                                    ),
                                    const SizedBox(height: AppTokens.space8),
                                    Text(
                                      'Try questions like "Where do they escape across the glacier?" or "When does Paul first ride a sandworm?"',
                                      textAlign: TextAlign.center,
                                      style: AppTypography.bodySans(fontSize: 13),
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
                                    context.go('/reader/${hit.bookId}/${hit.chapterIndex}');
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

class _SearchHitCard extends StatelessWidget {
  final SemanticSearchHit hit;
  final VoidCallback onTap;

  const _SearchHitCard({required this.hit, required this.onTap});

  @override
  Widget build(BuildContext context) {
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
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        hit.bookTitle,
                        style: AppTypography.titleSerif(fontSize: 16, fontWeight: FontWeight.w600),
                      ),
                      const SizedBox(height: AppTokens.space4),
                      Text(
                        hit.authorName,
                        style: AppTypography.bodySans(fontSize: 13),
                      ),
                    ],
                  ),
                ),
                StatusBadge(
                  label: '${hit.matchPercentage}% match',
                  backgroundColor: AppTokens.matchBadgeBg,
                  textColor: AppTokens.matchBadgeText,
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space12),

            // Chapter Badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              ),
              child: Text(
                'Chapter ${hit.chapterIndex}: ${hit.chapterTitle}',
                style: AppTypography.bodySans(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: AppTokens.charcoalInk,
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space8),

            // Summary Excerpt (<100 tokens)
            Text(
              hit.summary,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.bodySans(fontSize: 13, lineHeight: 1.5),
            ),
            const SizedBox(height: AppTokens.space12),

            // Action: Read at passage
            InkWell(
              onTap: onTap,
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
                        color: AppTokens.charcoalInk,
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
