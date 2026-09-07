import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../../data/models/book.dart';
import '../../state/providers.dart';

class LibraryView extends ConsumerStatefulWidget {
  const LibraryView({super.key});

  @override
  ConsumerState<LibraryView> createState() => _LibraryViewState();
}

class _LibraryViewState extends ConsumerState<LibraryView> {
  int _navIndex = 0;

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(libraryProvider.notifier).loadLibrary());
  }

  void _onBottomNavTapped(int index) {
    setState(() => _navIndex = index);
    if (index == 1) {
      context.go('/search');
    } else if (index == 2) {
      context.go('/connect');
    }
  }

  @override
  Widget build(BuildContext context) {
    final libraryState = ref.watch(libraryProvider);
    final horizontalPad = Responsive.horizontalPadding(context);

    final filterItems = const [
      FilterPillItem(id: 'all', label: 'All Books'),
      FilterPillItem(id: 'unread', label: 'Unread'),
      FilterPillItem(id: 'series', label: 'Series'),
      FilterPillItem(id: 'authors', label: 'Authors'),
    ];

    return Scaffold(
      appBar: ShelfdTopBar(
        title: 'Shelfd',
        subtitle: 'Connected to Homelab NAS',
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh_rounded),
            tooltip: 'Rescan Library',
            onPressed: () async {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Scanning library...')),
              );
              await ref.read(bookRepositoryProvider).triggerScan();
              if (context.mounted) {
                ref.read(libraryProvider.notifier).loadLibrary();
              }
            },
          ),
          IconButton(
            icon: const Icon(Icons.search_rounded),
            tooltip: 'Semantic Search',
            onPressed: () => context.go('/search'),
          ),
        ],
      ),
      bottomNavigationBar: ShelfdBottomNav(
        currentIndex: _navIndex,
        onTap: _onBottomNavTapped,
      ),
      body: SafeArea(
        child: Column(
          children: [
            // Filter Pills Row (DRY shared component)
            Padding(
              padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space12),
              child: FilterPillsRow(
                items: filterItems,
                selectedId: libraryState.activeFilter,
                onSelected: (filterId) {
                  ref.read(libraryProvider.notifier).setFilter(filterId);
                },
              ),
            ),
            const Divider(color: AppTokens.crispBorder, height: 1),

            // Main Book Grid Content
            Expanded(
              child: libraryState.isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : libraryState.filteredBooks.isEmpty
                      ? Center(
                          child: Padding(
                            padding: const EdgeInsets.all(AppTokens.space32),
                            child: Column(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                const Icon(
                                  Icons.menu_book_outlined,
                                  size: 48,
                                  color: AppTokens.mutedCopy,
                                ),
                                const SizedBox(height: AppTokens.space16),
                                Text(
                                  'Your shelf is quiet',
                                  style: AppTypography.titleSerif(fontSize: 20),
                                ),
                                const SizedBox(height: AppTokens.space8),
                                Text(
                                  'Place EPUBs into your /library directory or tap refresh to scan.',
                                  textAlign: TextAlign.center,
                                  style: AppTypography.bodySans(fontSize: 14),
                                ),
                              ],
                            ),
                          ),
                        )
                      : GridView.builder(
                          padding: EdgeInsets.symmetric(
                            horizontal: horizontalPad,
                            vertical: AppTokens.space16,
                          ),
                          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: Responsive.isDesktop(context)
                                ? 4
                                : Responsive.isTablet(context)
                                    ? 3
                                    : 2,
                            childAspectRatio: 0.58,
                            crossAxisSpacing: AppTokens.space16,
                            mainAxisSpacing: AppTokens.space24,
                          ),
                          itemCount: libraryState.filteredBooks.length,
                          itemBuilder: (context, index) {
                            final book = libraryState.filteredBooks[index];
                            return _BookCard(
                              book: book,
                              onTap: () {
                                context.go('/reader/${book.id}/0');
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

class _BookCard extends StatelessWidget {
  final Book book;
  final VoidCallback onTap;

  const _BookCard({required this.book, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final progressPercent = (book.readingProgress * 100).round();

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Cover Art Container (2:3 Aspect Ratio)
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(AppTokens.radiusMd),
                ),
                border: const Border(
                  bottom: BorderSide(color: AppTokens.crispBorder),
                ),
              ),
              child: ClipRRect(
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(AppTokens.radiusMd),
                ),
                child: book.coverUrl != null
                    ? Image.network(
                        book.coverUrl!,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) =>
                            _CoverFallback(title: book.title),
                      )
                    : _CoverFallback(title: book.title),
              ),
            ),
          ),

          // Metadata area
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  book.title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(fontSize: 15, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  book.authorDisplay,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(fontSize: 12),
                ),
                if (progressPercent > 0) ...[
                  const SizedBox(height: AppTokens.space8),
                  StatusBadge(
                    label: '$progressPercent% read',
                    backgroundColor: AppTokens.matchBadgeBg,
                    textColor: AppTokens.matchBadgeText,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _CoverFallback extends StatelessWidget {
  final String title;

  const _CoverFallback({required this.title});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.book_rounded, size: 36, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            title,
            maxLines: 3,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 12),
          ),
        ],
      ),
    );
  }
}
