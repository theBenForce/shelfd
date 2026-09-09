import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/author.dart';
import '../../../data/models/book.dart';
import '../../../data/models/series.dart';
import '../../core/app_shell.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../upload/upload_drop_target.dart';

class LibraryView extends ConsumerStatefulWidget {
  const LibraryView({super.key});

  @override
  ConsumerState<LibraryView> createState() => _LibraryViewState();
}

class _LibraryViewState extends ConsumerState<LibraryView> {
  int _navIndex = 0;
  late final ScrollController _scrollController;

  @override
  void initState() {
    super.initState();
    _scrollController = ScrollController()..addListener(_onScroll);
    Future.microtask(() => ref.read(libraryProvider.notifier).loadLibrary());
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final maxScroll = _scrollController.position.maxScrollExtent;
    final currentScroll = _scrollController.position.pixels;
    if (currentScroll >= maxScroll - 300) {
      ref.read(libraryProvider.notifier).loadMoreBooks();
    }
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _onNavTapped(int index) {
    if (index == 0) {
      setState(() => _navIndex = 0);
    } else if (index == 1) {
      context.go('/search');
    } else if (index == 2) {
      context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context) {
    final libraryState = ref.watch(libraryProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);

    final filterItems = const [
      FilterPillItem(id: 'all', label: 'All Books'),
      FilterPillItem(id: 'unread', label: 'Unread'),
      FilterPillItem(id: 'series', label: 'Series'),
      FilterPillItem(id: 'authors', label: 'Authors'),
    ];

    Future<void> handleScan() async {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Scanning library...')),
      );
      await ref.read(bookRepositoryProvider).triggerScan();
      if (context.mounted) {
        ref.read(libraryProvider.notifier).loadLibrary(refresh: true);
      }
    }

    if (libraryState.activeFilter != 'all' &&
        libraryState.activeFilter != 'series' &&
        libraryState.activeFilter != 'authors' &&
        libraryState.filteredBooks.length < 8 &&
        libraryState.hasMore &&
        !libraryState.isLoading &&
        !libraryState.isLoadingMore) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          ref.read(libraryProvider.notifier).loadMoreBooks();
        }
      });
    }

    final hasAppShell = context.findAncestorWidgetOfExactType<AppShell>() != null;

    final topBar = ShelfdTopBar(
      title: 'Shelfd',
      subtitle: 'Connected to Homelab NAS',
      actions: [
        IconButton(
          icon: const Icon(Icons.upload_file_outlined),
          tooltip: 'Upload EPUB',
          onPressed: () => pickAndUploadEpub(context, ref),
        ),
        IconButton(
          icon: const Icon(Icons.refresh_rounded),
          tooltip: 'Rescan Library',
          onPressed: handleScan,
        ),
        IconButton(
          icon: const Icon(Icons.search_rounded),
          tooltip: 'Semantic Search',
          onPressed: () => context.go('/search'),
        ),
      ],
    );

    final bodyContent = SafeArea(
      child: Column(
        children: [
              // Desktop Header: "Library" title and book count badge (Stitch spec)
              if (isDesktop)
                Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
                    child: Padding(
                      padding: EdgeInsets.fromLTRB(
                        horizontalPad,
                        AppTokens.space24,
                        horizontalPad,
                        AppTokens.space8,
                      ),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.baseline,
                        textBaseline: TextBaseline.alphabetic,
                        children: [
                          Text(
                            'Library',
                            style: AppTypography.titleSerif(
                              fontSize: 32,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(width: AppTokens.space12),
                          StatusBadge(
                            label: libraryState.activeFilter == 'series'
                                ? '${libraryState.filteredSeries.length} Series'
                                : libraryState.activeFilter == 'authors'
                                    ? '${libraryState.filteredAuthors.length} Authors'
                                    : (libraryState.totalBooks > 0
                                        ? '${libraryState.totalBooks} Books'
                                        : '${libraryState.books.length} Books'),
                            backgroundColor: AppTokens.boneContainer,
                            textColor: AppTokens.mutedCopy,
                          ),
                          const Spacer(),
                          OutlinedButton.icon(
                            style: OutlinedButton.styleFrom(
                              foregroundColor: AppTokens.charcoalInk,
                              side: const BorderSide(color: AppTokens.crispBorder),
                              backgroundColor: AppTokens.boneSurface,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                              ),
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppTokens.space16,
                                vertical: AppTokens.space12,
                              ),
                            ),
                            onPressed: () => pickAndUploadEpub(context, ref),
                            icon: const Icon(Icons.upload_file_outlined, size: 18),
                            label: const Text(
                              'Upload EPUB',
                              style: TextStyle(fontWeight: FontWeight.w600),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),

            // Filter Pills Row (DRY shared component)
            Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
                child: Padding(
                  padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space12),
                  child: FilterPillsRow(
                    items: filterItems,
                    selectedId: libraryState.activeFilter,
                    onSelected: (filterId) {
                      ref.read(libraryProvider.notifier).setFilter(filterId);
                    },
                  ),
                ),
              ),
            ),
            const Divider(color: AppTokens.crispBorder, height: 1),

            // Main Book Grid Content
            Expanded(
              child: libraryState.isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : libraryState.error != null
                      ? Center(
                          child: Padding(
                            padding: const EdgeInsets.all(AppTokens.space32),
                            child: Column(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                const Icon(
                                  Icons.error_outline_rounded,
                                  size: 48,
                                  color: Color(0xFFC92A2A),
                                ),
                                const SizedBox(height: AppTokens.space16),
                                Text(
                                  'Unable to load library',
                                  style: AppTypography.titleSerif(fontSize: 20),
                                ),
                                const SizedBox(height: AppTokens.space8),
                                Text(
                                  libraryState.error!,
                                  textAlign: TextAlign.center,
                                  style: AppTypography.bodySans(fontSize: 14),
                                ),
                                const SizedBox(height: AppTokens.space16),
                                Row(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    OutlinedButton(
                                      onPressed: () => ref.read(libraryProvider.notifier).loadLibrary(),
                                      child: const Text('Retry'),
                                    ),
                                    const SizedBox(width: AppTokens.space12),
                                    ElevatedButton(
                                      style: ElevatedButton.styleFrom(
                                        backgroundColor: AppTokens.charcoalInk,
                                        foregroundColor: Colors.white,
                                      ),
                                      onPressed: () => context.go('/connect'),
                                      child: const Text('Reconnect / Sign In'),
                                    ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        )
                      : _buildBodyContent(context, libraryState, horizontalPad),
            ),
          ],
        ),
      );
    if (hasAppShell) {
      return ShelfdDropTarget(
        child: Scaffold(
          backgroundColor: Colors.transparent,
          appBar: isDesktop ? null : topBar,
          body: bodyContent,
        ),
      );
    }

    return ShelfdDropTarget(
      child: ShelfdAdaptiveScaffold(
        currentIndex: _navIndex,
        onNavTap: _onNavTapped,
        onRescan: handleScan,
        onUpload: () => pickAndUploadEpub(context, ref),
        appBar: topBar,
        body: bodyContent,
      ),
    );
  }

  Widget _buildBodyContent(BuildContext context, LibraryState libraryState, double horizontalPad) {
    if (libraryState.activeFilter == 'series') {
      return _buildSeriesContent(context, libraryState, horizontalPad);
    } else if (libraryState.activeFilter == 'authors') {
      return _buildAuthorsContent(context, libraryState, horizontalPad);
    } else {
      return _buildBooksContent(context, libraryState, horizontalPad);
    }
  }

  Widget _buildSeriesContent(BuildContext context, LibraryState state, double horizontalPad) {
    if (state.filteredSeries.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(
                Icons.collections_bookmark_outlined,
                size: 48,
                color: AppTokens.mutedCopy,
              ),
              const SizedBox(height: AppTokens.space16),
              Text(
                'No series found',
                style: AppTypography.titleSerif(fontSize: 20),
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                'Books with series metadata will be grouped together here.',
                textAlign: TextAlign.center,
                style: AppTypography.bodySans(fontSize: 14),
              ),
            ],
          ),
        ),
      );
    }

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
        child: GridView.builder(
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPad,
            vertical: AppTokens.space16,
          ),
          gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
            maxCrossAxisExtent: 200.0,
            childAspectRatio: 0.55,
            crossAxisSpacing: AppTokens.space16,
            mainAxisSpacing: AppTokens.space24,
          ),
          itemCount: state.filteredSeries.length,
          itemBuilder: (context, index) {
            final series = state.filteredSeries[index];
            return _SeriesCard(
              series: series,
              onTap: () {
                context.go('/series/${series.id}');
              },
            );
          },
        ),
      ),
    );
  }

  Widget _buildAuthorsContent(BuildContext context, LibraryState state, double horizontalPad) {
    if (state.filteredAuthors.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(
                Icons.person_outline_rounded,
                size: 48,
                color: AppTokens.mutedCopy,
              ),
              const SizedBox(height: AppTokens.space16),
              Text(
                'No authors found',
                style: AppTypography.titleSerif(fontSize: 20),
              ),
              const SizedBox(height: AppTokens.space8),
              Text(
                'Books in your library will be organized by author here.',
                textAlign: TextAlign.center,
                style: AppTypography.bodySans(fontSize: 14),
              ),
            ],
          ),
        ),
      );
    }

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
        child: GridView.builder(
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPad,
            vertical: AppTokens.space16,
          ),
          gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
            maxCrossAxisExtent: 220.0,
            childAspectRatio: 0.85,
            crossAxisSpacing: AppTokens.space16,
            mainAxisSpacing: AppTokens.space16,
          ),
          itemCount: state.filteredAuthors.length,
          itemBuilder: (context, index) {
            final author = state.filteredAuthors[index];
            return _AuthorCard(
              author: author,
              onTap: () {
                context.go('/author/${author.id}');
              },
            );
          },
        ),
      ),
    );
  }

  Widget _buildBooksContent(BuildContext context, LibraryState state, double horizontalPad) {
    if (state.filteredBooks.isEmpty) {
      return Center(
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
              const SizedBox(height: AppTokens.space16),
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: AppTokens.charcoalInk,
                  side: const BorderSide(color: AppTokens.crispBorder),
                  backgroundColor: AppTokens.boneSurface,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space16,
                    vertical: AppTokens.space12,
                  ),
                ),
                onPressed: () => pickAndUploadEpub(context, ref),
                icon: const Icon(Icons.upload_file_outlined, size: 18),
                label: const Text(
                  'Upload EPUB',
                  style: TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
      );
    }

    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
        child: Column(
          children: [
            Expanded(
              child: GridView.builder(
                controller: _scrollController,
                padding: EdgeInsets.symmetric(
                  horizontal: horizontalPad,
                  vertical: AppTokens.space16,
                ),
                gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                  maxCrossAxisExtent: 200.0,
                  childAspectRatio: 0.55,
                  crossAxisSpacing: AppTokens.space16,
                  mainAxisSpacing: AppTokens.space24,
                ),
                itemCount: state.filteredBooks.length,
                itemBuilder: (context, index) {
                  final book = state.filteredBooks[index];
                  return _BookCard(
                    book: book,
                    onTap: () {
                      context.go('/book/${book.id}');
                    },
                  );
                },
              ),
            ),
            if (state.isLoadingMore)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: AppTokens.space16),
                child: Center(
                  child: SizedBox(
                    width: 24,
                    height: 24,
                    child: CircularProgressIndicator(strokeWidth: 2.5),
                  ),
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
                  style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
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

class _SeriesCard extends StatelessWidget {
  final Series series;
  final VoidCallback onTap;

  const _SeriesCard({required this.series, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final bookCountLabel = series.bookCount == 1 ? '1 Book' : '${series.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
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
              child: Stack(
                fit: StackFit.expand,
                children: [
                  ClipRRect(
                    borderRadius: const BorderRadius.vertical(
                      top: Radius.circular(AppTokens.radiusMd),
                    ),
                    child: series.coverUrl != null && series.coverUrl!.isNotEmpty
                        ? Image.network(
                            series.coverUrl!,
                            fit: BoxFit.cover,
                            errorBuilder: (context, error, stackTrace) =>
                                _SeriesFallback(name: series.name),
                          )
                        : _SeriesFallback(name: series.name),
                  ),
                  Positioned(
                    top: AppTokens.space8,
                    right: AppTokens.space8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: AppTokens.charcoalInk.withValues(alpha: 0.85),
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.collections_bookmark_outlined, size: 12, color: Colors.white),
                          const SizedBox(width: 4),
                          Text(
                            bookCountLabel,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  series.name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  'Series',
                  style: AppTypography.captionSans(
                    fontSize: 11,
                    color: AppTokens.mutedCopy,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SeriesFallback extends StatelessWidget {
  final String name;

  const _SeriesFallback({required this.name});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.collections_bookmark_rounded, size: 36, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            name,
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

class _AuthorCard extends StatelessWidget {
  final Author author;
  final VoidCallback onTap;

  const _AuthorCard({required this.author, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final bookCountLabel = author.bookCount == 1 ? '1 Book' : '${author.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          AuthorAvatar(
            name: author.name,
            photoUrl: author.photoUrl,
            size: 72,
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            author.name,
            maxLines: 2,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 15, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space8),
          StatusBadge(
            label: bookCountLabel,
            backgroundColor: AppTokens.boneContainer,
            textColor: AppTokens.mutedCopy,
          ),
        ],
      ),
    );
  }
}
