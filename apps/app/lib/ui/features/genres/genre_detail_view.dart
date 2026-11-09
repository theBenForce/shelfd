import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/genre.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class GenreDetailView extends ConsumerStatefulWidget {
  final String genreId;
  final String? genreName;

  const GenreDetailView({
    super.key,
    required this.genreId,
    this.genreName,
  });

  @override
  ConsumerState<GenreDetailView> createState() => _GenreDetailViewState();
}

class _GenreDetailViewState extends ConsumerState<GenreDetailView> {
  bool _isLoading = true;
  String? _error;
  List<Book> _books = [];
  Genre? _genre;

  @override
  void initState() {
    super.initState();
    _loadGenreBooks();
  }

  Future<void> _loadGenreBooks() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final bookRepo = ref.read(bookRepositoryProvider);
      final genres = await bookRepo.getGenres();
      final match = genres.where((g) => g.id == widget.genreId).firstOrNull;

      final pageData = await bookRepo.getBooksPage(
        genreId: widget.genreId,
        genreName: widget.genreName ?? match?.name,
        perPage: 100,
      );

      final sortedBooks = pageData.books.toList()
        ..sort((a, b) => a.title.toLowerCase().compareTo(b.title.toLowerCase()));

      if (mounted) {
        setState(() {
          _genre = match;
          _books = sortedBooks;
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

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final horizontalPad = Responsive.horizontalPadding(context);
    final name = _genre?.name ?? widget.genreName ?? 'Genre';

    return Scaffold(
      backgroundColor: colors.background,
      appBar: ShelfdTopBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_rounded),
          tooltip: 'Back to Library',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/genres');
            }
          },
        ),
        title: name,
        subtitle: _books.isNotEmpty ? '${_books.length} Books in Genre' : null,
      ),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: AppTokens.maxLibraryWidth),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: horizontalPad),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: AppTokens.space20),
                    child: Row(
                      children: [
                        Container(
                          width: 56,
                          height: 56,
                          decoration: BoxDecoration(
                            color: colors.container,
                            shape: BoxShape.circle,
                            border: Border.all(color: colors.subtleBorder),
                          ),
                          child: Icon(
                            Icons.category_outlined,
                            color: colors.accentPrimary,
                            size: 26,
                          ),
                        ),
                        const SizedBox(width: AppTokens.space16),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                name,
                                style: AppTypography.titleSerif(
                                  fontSize: 26,
                                  fontWeight: FontWeight.w600,
                                  color: colors.ink,
                                ),
                              ),
                              const SizedBox(height: 4),
                              StatusBadge(
                                label: '${_books.length} Books',
                                backgroundColor: colors.container,
                                textColor: colors.secondaryInk,
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  Divider(color: colors.subtleBorder, height: 1),
                  Expanded(
                    child: _isLoading
                        ? Center(child: CircularProgressIndicator(color: colors.accentPrimary))
                        : _error != null
                            ? Center(
                                child: Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    Icon(
                                      Icons.error_outline_rounded,
                                      size: 48,
                                      color: colors.errorText,
                                    ),
                                    const SizedBox(height: AppTokens.space16),
                                    Text(
                                      'Unable to load genre books',
                                      style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
                                    ),
                                    const SizedBox(height: AppTokens.space8),
                                    Text(
                                      _error!,
                                      style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
                                    ),
                                    const SizedBox(height: AppTokens.space16),
                                    OutlinedButton(
                                      style: OutlinedButton.styleFrom(
                                        foregroundColor: colors.ink,
                                        side: BorderSide(color: colors.border),
                                      ),
                                      onPressed: _loadGenreBooks,
                                      child: const Text('Retry'),
                                    ),
                                  ],
                                ),
                              )
                            : _books.isEmpty
                                ? Center(
                                    child: Text(
                                      'No books found in this genre',
                                      style: AppTypography.bodySans(color: colors.mutedInk),
                                    ),
                                  )
                                : GridView.builder(
                                    padding: const EdgeInsets.symmetric(vertical: AppTokens.space16),
                                    gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                                      maxCrossAxisExtent: 200.0,
                                      childAspectRatio: 0.55,
                                      crossAxisSpacing: AppTokens.space16,
                                      mainAxisSpacing: AppTokens.space24,
                                    ),
                                    itemCount: _books.length,
                                    itemBuilder: (context, index) {
                                      final book = _books[index];
                                      return _GenreBookCard(
                                        book: book,
                                        onTap: () => context.go('/books/${book.id}'),
                                      );
                                    },
                                  ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _GenreBookCard extends StatelessWidget {
  final Book book;
  final VoidCallback onTap;

  const _GenreBookCard({
    required this.book,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: TactileBookCover(
              coverUrl: book.coverUrl,
              title: book.title,
              showFallbackTitle: false,
              showSpineCrease: true,
              showShadow: true,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              bottomOverlay: book.readingProgress > 0
                  ? LinearProgressIndicator(
                      value: book.readingProgress.clamp(0.0, 1.0),
                      minHeight: 3,
                      backgroundColor: colors.containerHigh.withValues(alpha: 0.6),
                      valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
                    )
                  : null,
            ),
          ),
          const SizedBox(height: AppTokens.space8),
          Text(
            book.title,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            book.authorDisplay,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.bodySans(
              fontSize: 12,
              color: colors.secondaryInk,
            ),
          ),
        ],
      ),
    );
  }
}
