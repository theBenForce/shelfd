import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/author.dart';
import '../../../data/models/book.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class AuthorDetailView extends ConsumerStatefulWidget {
  final String authorId;
  final String? authorName;

  const AuthorDetailView({
    super.key,
    required this.authorId,
    this.authorName,
  });

  @override
  ConsumerState<AuthorDetailView> createState() => _AuthorDetailViewState();
}

class _AuthorDetailViewState extends ConsumerState<AuthorDetailView> {
  bool _isLoading = true;
  String? _error;
  List<Book> _books = [];
  Author? _author;

  @override
  void initState() {
    super.initState();
    _loadAuthorBooks();
  }

  Future<void> _loadAuthorBooks() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final bookRepo = ref.read(bookRepositoryProvider);
      final authors = await bookRepo.getAuthors();
      final match = authors.where((a) => a.id == widget.authorId).firstOrNull;

      final pageData = await bookRepo.getBooksPage(
        authorId: widget.authorId,
        perPage: 100,
      );

      final sortedBooks = pageData.books.toList()
        ..sort((a, b) => a.title.toLowerCase().compareTo(b.title.toLowerCase()));

      if (mounted) {
        setState(() {
          _author = match;
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
    final name = _author?.name ?? widget.authorName ?? (_books.isNotEmpty ? _books.first.authorDisplay : 'Author');

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
              context.go('/authors');
            }
          },
        ),
        title: name,
        subtitle: _books.isNotEmpty ? '${_books.length} Books by Author' : null,
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
                        AuthorAvatar(
                          name: name,
                          photoUrl: _author?.photoUrl,
                          size: 72,
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
                                      'Unable to load author books',
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
                                      onPressed: _loadAuthorBooks,
                                      child: const Text('Retry'),
                                    ),
                                  ],
                                ),
                              )
                            : _books.isEmpty
                                ? Center(
                                    child: Text(
                                      'No books found for this author',
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
                                      return _AuthorBookCard(
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

class _AuthorBookCard extends StatelessWidget {
  final Book book;
  final VoidCallback onTap;

  const _AuthorBookCard({
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
              topLeftBadge: book.series != null
                  ? Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: AppTokens.espressoInk.withValues(alpha: 0.88),
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      ),
                      child: Text(
                        book.seriesSequence != null
                            ? '${book.series!.name} #${book.seriesSequence!.toStringAsFixed(book.seriesSequence!.truncateToDouble() == book.seriesSequence! ? 0 : 1)}'
                            : book.series!.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.mono(
                          color: Colors.white,
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
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
          if (book.readingProgress > 0) ...[
            const SizedBox(height: 4),
            ClipRRect(
              borderRadius: BorderRadius.circular(2),
              child: LinearProgressIndicator(
                value: book.readingProgress.clamp(0.0, 1.0),
                minHeight: 3,
                backgroundColor: colors.container,
                valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
