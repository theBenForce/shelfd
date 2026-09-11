import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../core/app_shell.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../upload/upload_drop_target.dart';

enum LibraryViewMode {
  books,
  series,
  authors,
}

class LibraryView extends ConsumerStatefulWidget {
  final LibraryViewMode mode;
  final String? initialFilter;

  const LibraryView({
    super.key,
    this.mode = LibraryViewMode.books,
    this.initialFilter,
  });

  @override
  ConsumerState<LibraryView> createState() => _LibraryViewState();
}

class _LibraryViewState extends ConsumerState<LibraryView> {
  late final ScrollController _scrollController;

  @override
  void initState() {
    super.initState();
    _scrollController = ScrollController()..addListener(_onScroll);
    final initial = (widget.initialFilter != null && widget.initialFilter!.isNotEmpty)
        ? widget.initialFilter!
        : (widget.mode == LibraryViewMode.series
            ? 'series'
            : widget.mode == LibraryViewMode.authors
                ? 'authors'
                : 'all');
    Future.microtask(() {
      ref.read(libraryProvider.notifier).setFilter(initial);
      ref.read(libraryProvider.notifier).loadLibrary();
    });
  }

  @override
  void didUpdateWidget(LibraryView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.mode != oldWidget.mode) {
      final newFilter = widget.mode == LibraryViewMode.series
          ? 'series'
          : widget.mode == LibraryViewMode.authors
              ? 'authors'
              : (widget.initialFilter ?? 'all');
      ref.read(libraryProvider.notifier).setFilter(newFilter);
    } else {
      final current = (widget.initialFilter != null && widget.initialFilter!.isNotEmpty)
          ? widget.initialFilter!
          : 'all';
      final old = (oldWidget.initialFilter != null && oldWidget.initialFilter!.isNotEmpty)
          ? oldWidget.initialFilter!
          : 'all';
      if (current != old) {
        ref.read(libraryProvider.notifier).setFilter(current);
      }
    }
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
      context.go('/books');
    } else if (index == 1) {
      context.go('/series');
    } else if (index == 2) {
      context.go('/authors');
    } else if (index == 3) {
      context.go('/search');
    } else if (index == 4) {
      context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context) {
    final libraryState = ref.watch(libraryProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);

    final isSeries = widget.mode == LibraryViewMode.series || libraryState.activeFilter == 'series';
    final isAuthors = widget.mode == LibraryViewMode.authors || libraryState.activeFilter == 'authors';

    final String pageTitle = isSeries ? 'Series' : (isAuthors ? 'Authors' : 'Books');
    final String badgeLabel = isSeries
        ? '${libraryState.filteredSeries.length} Series'
        : isAuthors
            ? '${libraryState.filteredAuthors.length} Authors'
            : (libraryState.totalBooks > 0
                ? '${libraryState.totalBooks} Books'
                : '${libraryState.books.length} Books');

    final filterItems = const [
      FilterPillItem(id: 'all', label: 'All Books'),
      FilterPillItem(id: 'unread', label: 'Unread'),
      FilterPillItem(id: 'series', label: 'Series'),
      FilterPillItem(id: 'authors', label: 'Authors'),
    ];

    Future<void> handleScan() async {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Scanning library for new books...')),
      );
      await ref.read(bookRepositoryProvider).triggerScan();
    }

    if (!isSeries &&
        !isAuthors &&
        libraryState.activeFilter != 'all' &&
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
        const ShelfdUploadsBadgeButton(),
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

    final selectedFilterId = isSeries ? 'series' : (isAuthors ? 'authors' : libraryState.activeFilter);

    final bodyContent = SafeArea(
      child: Column(
        children: [
          // Desktop Header: Title and count badge
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
                        pageTitle,
                        style: AppTypography.titleSerif(
                          fontSize: 32,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(width: AppTokens.space12),
                      StatusBadge(
                        label: badgeLabel,
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
                  selectedId: selectedFilterId,
                  onSelected: (filterId) {
                    if (filterId == 'series') {
                      context.go('/series');
                    } else if (filterId == 'authors') {
                      context.go('/authors');
                    } else {
                      ref.read(libraryProvider.notifier).setFilter(filterId);
                      if (filterId == 'all') {
                        context.go('/books');
                      } else {
                        context.go('/books?filter=$filterId');
                      }
                    }
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

    final currentNavIndex = isSeries ? 1 : isAuthors ? 2 : 0;
    return ShelfdDropTarget(
      child: ShelfdAdaptiveScaffold(
        currentIndex: currentNavIndex,
        onNavTap: _onNavTapped,
        currentPath: isSeries ? '/series' : isAuthors ? '/authors' : '/books',
        onNavigate: (path) => context.go(path),
        onRescan: handleScan,
        onUpload: () => pickAndUploadEpub(context, ref),
        appBar: topBar,
        body: bodyContent,
      ),
    );
  }

  Widget _buildBodyContent(BuildContext context, LibraryState libraryState, double horizontalPad) {
    final isSeries = widget.mode == LibraryViewMode.series || libraryState.activeFilter == 'series';
    final isAuthors = widget.mode == LibraryViewMode.authors || libraryState.activeFilter == 'authors';

    if (isSeries) {
      return _buildSeriesContent(context, libraryState, horizontalPad);
    } else if (isAuthors) {
      return _buildAuthorsContent(context, libraryState, horizontalPad);
    } else {
      return _buildBooksContent(context, libraryState, horizontalPad);
    }
  }

  Widget _buildSeriesContent(BuildContext context, LibraryState state, double horizontalPad) {
    final seriesItems = state.seriesGridItems;
    if (seriesItems.isEmpty) {
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
          itemCount: seriesItems.length,
          itemBuilder: (context, index) {
            final item = seriesItems[index];
            return ShelfdGridCard(
              item: item,
              onTap: () {
                context.go('/series/${item.id}');
              },
            );
          },
        ),
      ),
    );
  }

  Widget _buildAuthorsContent(BuildContext context, LibraryState state, double horizontalPad) {
    final authorItems = state.authorGridItems;
    if (authorItems.isEmpty) {
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
          itemCount: authorItems.length,
          itemBuilder: (context, index) {
            final item = authorItems[index];
            return ShelfdGridCard(
              item: item,
              onTap: () {
                context.go('/authors/${item.id}');
              },
            );
          },
        ),
      ),
    );
  }

  Widget _buildBooksContent(BuildContext context, LibraryState state, double horizontalPad) {
    final items = state.groupedBookItems;
    if (items.isEmpty) {
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
                itemCount: items.length,
                itemBuilder: (context, index) {
                  final item = items[index];
                  return ShelfdGridCard(
                    item: item,
                    onTap: () {
                      switch (item) {
                        case BookGridItem(:final book):
                          context.go('/books/${book.id}');
                        case SeriesGridItem(:final series):
                          context.go('/series/${series.id}');
                        case AuthorGridItem(:final author):
                          context.go('/authors/${author.id}');
                      }
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
