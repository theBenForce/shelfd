import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

int compareSeriesBooks(Book a, Book b) {
  final seqA = a.seriesSequence;
  final seqB = b.seriesSequence;
  if (seqA != null && seqB != null) {
    final comp = seqA.compareTo(seqB);
    if (comp != 0) return comp;
    return a.title.toLowerCase().compareTo(b.title.toLowerCase());
  } else if (seqA != null) {
    return -1;
  } else if (seqB != null) {
    return 1;
  } else {
    return a.title.toLowerCase().compareTo(b.title.toLowerCase());
  }
}

class SeriesDetailView extends ConsumerStatefulWidget {
  final String seriesId;
  final String? seriesName;

  const SeriesDetailView({
    super.key,
    required this.seriesId,
    this.seriesName,
  });

  @override
  ConsumerState<SeriesDetailView> createState() => _SeriesDetailViewState();
}

class _SeriesDetailViewState extends ConsumerState<SeriesDetailView> {
  bool _isLoading = true;
  String? _error;
  List<Book> _books = [];
  String? _seriesName;

  @override
  void initState() {
    super.initState();
    _loadSeriesBooks();
  }

  Future<void> _loadSeriesBooks() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final bookRepo = ref.read(bookRepositoryProvider);
      final pageData = await bookRepo.getBooksPage(
        seriesId: widget.seriesId,
        perPage: 100,
      );

      final sortedBooks = pageData.books.toList()..sort(compareSeriesBooks);

      String? resolvedName = widget.seriesName;
      if (resolvedName == null || resolvedName.isEmpty) {
        final allSeries = await bookRepo.getSeries();
        final match = allSeries.where((s) => s.id == widget.seriesId).firstOrNull;
        if (match != null) {
          resolvedName = match.name;
        }
      }

      if (mounted) {
        setState(() {
          _seriesName = resolvedName;
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
    final horizontalPad = Responsive.horizontalPadding(context);
    final title = _seriesName ??
        (widget.seriesName != null && widget.seriesName!.isNotEmpty
            ? widget.seriesName!
            : (_books.isNotEmpty && _books.first.series != null
                ? _books.first.series!.name
                : 'Series'));

    return Scaffold(
      backgroundColor: AppTokens.boneBackground,
      appBar: ShelfdTopBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_rounded),
          tooltip: 'Back to Library',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go('/series');
            }
          },
        ),
        title: title,
        subtitle: _books.isNotEmpty ? '${_books.length} Books in Series' : null,
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
                    padding: const EdgeInsets.symmetric(vertical: AppTokens.space16),
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            title,
                            style: AppTypography.titleSerif(
                              fontSize: 26,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                        if (_books.isNotEmpty)
                          StatusBadge(
                            label: '${_books.length} Books',
                            backgroundColor: AppTokens.boneContainer,
                            textColor: AppTokens.mutedCopy,
                          ),
                      ],
                    ),
                  ),
                  const Divider(color: AppTokens.crispBorder, height: 1),
                  Expanded(
                    child: _isLoading
                        ? const Center(child: CircularProgressIndicator())
                        : _error != null
                            ? Center(
                                child: Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    const Icon(
                                      Icons.error_outline_rounded,
                                      size: 48,
                                      color: Color(0xFFC92A2A),
                                    ),
                                    const SizedBox(height: AppTokens.space16),
                                    Text('Unable to load series books', style: AppTypography.titleSerif(fontSize: 18)),
                                    const SizedBox(height: AppTokens.space8),
                                    Text(_error!, style: AppTypography.bodySans(fontSize: 14)),
                                    const SizedBox(height: AppTokens.space16),
                                    OutlinedButton(
                                      onPressed: _loadSeriesBooks,
                                      child: const Text('Retry'),
                                    ),
                                  ],
                                ),
                              )
                            : _books.isEmpty
                                ? Center(
                                    child: Text(
                                      'No books found in this series',
                                      style: AppTypography.bodySans(color: AppTokens.mutedCopy),
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
                                      return _SeriesBookCard(
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

class _SeriesBookCard extends StatelessWidget {
  final Book book;
  final VoidCallback onTap;

  const _SeriesBookCard({
    required this.book,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Stack(
              children: [
                Container(
                  decoration: BoxDecoration(
                    color: AppTokens.boneContainer,
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    border: Border.all(color: AppTokens.crispBorder),
                    boxShadow: const [
                      BoxShadow(
                        color: Color(0x0A000000),
                        blurRadius: 8,
                        offset: Offset(0, 4),
                      ),
                    ],
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: book.coverUrl != null && book.coverUrl!.isNotEmpty
                      ? Image.network(
                          book.coverUrl!,
                          fit: BoxFit.cover,
                          width: double.infinity,
                          height: double.infinity,
                          errorBuilder: (context, error, stackTrace) =>
                              _buildPlaceholder(),
                        )
                      : _buildPlaceholder(),
                ),
                if (book.seriesSequence != null)
                  Positioned(
                    top: AppTokens.space8,
                    left: AppTokens.space8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: AppTokens.charcoalInk.withValues(alpha: 0.85),
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      ),
                      child: Text(
                        '#${book.seriesSequence!.toStringAsFixed(book.seriesSequence!.truncateToDouble() == book.seriesSequence! ? 0 : 1)}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ),
              ],
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
            ),
          ),
          const SizedBox(height: 2),
          Text(
            book.authorDisplay,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.bodySans(
              fontSize: 12,
              color: AppTokens.mutedCopy,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPlaceholder() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.book_outlined, size: 36, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppTokens.space8),
            child: Text(
              book.title,
              textAlign: TextAlign.center,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.captionSans(fontSize: 11),
            ),
          ),
        ],
      ),
    );
  }
}
