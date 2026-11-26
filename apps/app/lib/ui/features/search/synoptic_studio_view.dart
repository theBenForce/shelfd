import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/genre.dart';
import '../../../data/models/synoptic.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class SynopticStudioView extends ConsumerStatefulWidget {
  const SynopticStudioView({super.key});

  @override
  ConsumerState<SynopticStudioView> createState() => _SynopticStudioViewState();
}

class _SynopticStudioViewState extends ConsumerState<SynopticStudioView> {
  late final TextEditingController _promptController;

  @override
  void initState() {
    super.initState();
    final initialQuery = ref.read(synopticProvider).query;
    _promptController = TextEditingController(text: initialQuery);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final library = ref.read(libraryProvider);
      if (library.books.isEmpty && !library.isLoading && library.error == null) {
        ref.read(libraryProvider.notifier).loadLibrary();
      }
    });
  }

  @override
  void dispose() {
    _promptController.dispose();
    super.dispose();
  }

  void _submit() {
    ref.read(synopticProvider.notifier).runQuery(_promptController.text);
  }

  void _openCitation(SynopticCitation citation) {
    if (citation.bookId.isEmpty) return;
    context.go('/books/${citation.bookId}/${citation.chapterIndex}');
  }

  @override
  Widget build(BuildContext context) {
    final synopticState = ref.watch(synopticProvider);
    final libraryState = ref.watch(libraryProvider);
    final colors = context.shelfdColors;
    final horizontalPad = Responsive.horizontalPadding(context);

    final sortedBooks = List<Book>.from(libraryState.books)
      ..sort((a, b) => a.title.toLowerCase().compareTo(b.title.toLowerCase()));
    final genres = libraryState.filteredGenres;
    final hasAnySelection = synopticState.selectedBookIds.isNotEmpty ||
        (synopticState.selectedGenreId != null &&
            synopticState.selectedGenreId!.isNotEmpty);

    return ListView(
      padding: EdgeInsets.fromLTRB(
        horizontalPad,
        AppTokens.space16,
        horizontalPad,
        AppTokens.space32,
      ),
      children: [
        _BookSelectorSection(
          books: sortedBooks,
          selectedBookIds: synopticState.selectedBookIds,
          isLoadingBooks: libraryState.isLoading,
          hasAnySelection: hasAnySelection,
          onToggleBook: (id) => ref.read(synopticProvider.notifier).toggleBook(id),
          onClear: () => ref.read(synopticProvider.notifier).clearSelection(),
          onRefresh: () =>
              ref.read(libraryProvider.notifier).loadLibrary(refresh: true),
        ),
        if (genres.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space16),
          _GenreFilterSection(
            genres: genres,
            selectedGenreId: synopticState.selectedGenreId,
            onSelectGenre: (id) =>
                ref.read(synopticProvider.notifier).selectGenre(id),
          ),
        ],
        const SizedBox(height: AppTokens.space16),
        _PromptSection(
          controller: _promptController,
          isLoading: synopticState.isLoading,
          hasValidScope: synopticState.hasValidScope,
          selectedBookCount: synopticState.selectedBookIds.length,
          selectedGenreName: _resolveGenreName(genres, synopticState.selectedGenreId),
          error: synopticState.error,
          onChanged: (value) =>
              ref.read(synopticProvider.notifier).setQuery(value),
          onSubmit: _submit,
        ),
        const SizedBox(height: AppTokens.space20),
        if (synopticState.isLoading)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: AppTokens.space32),
            child: Column(
              children: [
                CircularProgressIndicator(color: colors.accentPrimary),
                const SizedBox(height: AppTokens.space12),
                Text(
                  'Synthesizing passages across books...',
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    color: colors.secondaryInk,
                  ),
                ),
              ],
            ),
          )
        else if (synopticState.result != null)
          _SynopticResultView(
            result: synopticState.result!,
            onCitationTap: _openCitation,
          )
        else
          _EmptySynopticState(colors: colors),
      ],
    );
  }

  String? _resolveGenreName(List<Genre> genres, String? selectedGenreId) {
    if (selectedGenreId == null || selectedGenreId.isEmpty) return null;
    for (final genre in genres) {
      if (genre.id == selectedGenreId) return genre.name;
    }
    return selectedGenreId;
  }
}

class _BookSelectorSection extends StatelessWidget {
  final List<Book> books;
  final List<String> selectedBookIds;
  final bool isLoadingBooks;
  final bool hasAnySelection;
  final ValueChanged<String> onToggleBook;
  final VoidCallback onClear;
  final VoidCallback onRefresh;

  const _BookSelectorSection({
    required this.books,
    required this.selectedBookIds,
    required this.isLoadingBooks,
    required this.hasAnySelection,
    required this.onToggleBook,
    required this.onClear,
    required this.onRefresh,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    final selectedSet = selectedBookIds.toSet();

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Wrap(
                  spacing: AppTokens.space8,
                  runSpacing: AppTokens.space4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(
                      'Select Books',
                      style: AppTypography.titleSerif(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
                    ),
                    StatusBadge(
                      label: '${selectedBookIds.length} selected',
                      backgroundColor: selectedBookIds.length >= 2
                          ? (colors.isDark
                              ? colors.accentSecondaryContainer
                              : AppTokens.completedBadgeBg)
                          : colors.containerLow,
                      textColor: selectedBookIds.length >= 2
                          ? colors.accentSecondary
                          : colors.secondaryInk,
                    ),
                  ],
                ),
              ),
              if (hasAnySelection)
                TextButton(
                  onPressed: onClear,
                  style: TextButton.styleFrom(
                    foregroundColor: colors.accentPrimary,
                    visualDensity: VisualDensity.compact,
                  ),
                  child: const Text('Clear'),
                ),
              IconButton(
                tooltip: 'Refresh books',
                onPressed: isLoadingBooks ? null : onRefresh,
                icon: Icon(
                  Icons.refresh_rounded,
                  size: 18,
                  color: colors.secondaryInk,
                ),
                visualDensity: VisualDensity.compact,
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space12),
          if (isLoadingBooks && books.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: AppTokens.space12),
              child: Row(
                children: [
                  SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: colors.accentPrimary,
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Text(
                    'Loading library books...',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.secondaryInk,
                    ),
                  ),
                ],
              ),
            )
          else if (books.isEmpty)
            Text(
              'No books available yet. Add books to your library to compare them.',
              style: AppTypography.bodySans(
                fontSize: 13,
                color: colors.secondaryInk,
              ),
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final maxLabelWidth = (constraints.maxWidth - 48).clamp(120.0, 280.0);
                return Wrap(
                  spacing: AppTokens.space8,
                  runSpacing: AppTokens.space8,
                  children: books.map((book) {
                    final isSelected = selectedSet.contains(book.id);
                    return FilterChip(
                      selected: isSelected,
                      onSelected: (_) => onToggleBook(book.id),
                      label: ConstrainedBox(
                        constraints: BoxConstraints(maxWidth: maxLabelWidth),
                        child: Text(
                          book.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            fontWeight:
                                isSelected ? FontWeight.w600 : FontWeight.w500,
                            color: isSelected ? colors.onAccent : colors.ink,
                          ),
                        ),
                      ),
                      selectedColor: colors.accentPrimary,
                      checkmarkColor: colors.onAccent,
                      backgroundColor: colors.containerLow,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                        side: BorderSide(
                          color: isSelected
                              ? colors.accentPrimary
                              : colors.subtleBorder,
                        ),
                      ),
                    );
                  }).toList(),
                );
              },
            ),
        ],
      ),
    );
  }
}

class _GenreFilterSection extends StatelessWidget {
  final List<Genre> genres;
  final String? selectedGenreId;
  final ValueChanged<String?> onSelectGenre;

  const _GenreFilterSection({
    required this.genres,
    required this.selectedGenreId,
    required this.onSelectGenre,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Or filter by genre',
          style: AppTypography.bodySans(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            color: colors.secondaryInk,
          ),
        ),
        const SizedBox(height: AppTokens.space8),
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              Padding(
                padding: const EdgeInsets.only(right: AppTokens.space8),
                child: ChoiceChip(
                  label: Text(
                    'All genres',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: selectedGenreId == null
                          ? FontWeight.w600
                          : FontWeight.w500,
                      color: selectedGenreId == null
                          ? colors.onAccent
                          : colors.secondaryInk,
                    ),
                  ),
                  selected: selectedGenreId == null,
                  onSelected: (_) => onSelectGenre(null),
                  selectedColor: colors.accentPrimary,
                  backgroundColor: colors.containerLow,
                  showCheckmark: false,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                    side: BorderSide(
                      color: selectedGenreId == null
                          ? colors.accentPrimary
                          : colors.subtleBorder,
                    ),
                  ),
                ),
              ),
              ...genres.map((genre) {
                final isSelected = selectedGenreId == genre.id;
                return Padding(
                  padding: const EdgeInsets.only(right: AppTokens.space8),
                  child: ChoiceChip(
                    label: Text(
                      genre.name,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        fontWeight:
                            isSelected ? FontWeight.w600 : FontWeight.w500,
                        color: isSelected ? colors.onAccent : colors.secondaryInk,
                      ),
                    ),
                    selected: isSelected,
                    onSelected: (_) => onSelectGenre(genre.id),
                    selectedColor: colors.accentPrimary,
                    backgroundColor: colors.containerLow,
                    showCheckmark: false,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      side: BorderSide(
                        color: isSelected
                            ? colors.accentPrimary
                            : colors.subtleBorder,
                      ),
                    ),
                  ),
                );
              }),
            ],
          ),
        ),
      ],
    );
  }
}

class _PromptSection extends StatelessWidget {
  final TextEditingController controller;
  final bool isLoading;
  final bool hasValidScope;
  final int selectedBookCount;
  final String? selectedGenreName;
  final String? error;
  final ValueChanged<String> onChanged;
  final VoidCallback onSubmit;

  const _PromptSection({
    required this.controller,
    required this.isLoading,
    required this.hasValidScope,
    required this.selectedBookCount,
    required this.selectedGenreName,
    required this.error,
    required this.onChanged,
    required this.onSubmit,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    final scopeSummary = selectedGenreName != null
        ? 'Genre: $selectedGenreName${selectedBookCount > 0 ? ' + $selectedBookCount books' : ''}'
        : selectedBookCount >= 2
            ? '$selectedBookCount books selected'
            : 'Pick at least 2 books or a genre filter';

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: controller,
            textInputAction: TextInputAction.search,
            minLines: 1,
            maxLines: 3,
            onChanged: onChanged,
            onSubmitted: (_) => onSubmit(),
            style: AppTypography.bodySans(fontSize: 15, color: colors.ink),
            decoration: InputDecoration(
              hintText:
                  'Compare themes, characters, or arguments across books...',
              hintStyle:
                  AppTypography.bodySans(fontSize: 14, color: colors.mutedInk),
              filled: true,
              fillColor: colors.containerLow,
              prefixIcon: Icon(
                Icons.compare_arrows_rounded,
                color: colors.accentPrimary,
                size: 20,
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
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          Row(
            children: [
              Expanded(
                child: Text(
                  scopeSummary,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 12,
                    color: hasValidScope
                        ? colors.accentSecondary
                        : colors.mutedInk,
                    fontWeight:
                        hasValidScope ? FontWeight.w600 : FontWeight.w400,
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              FilledButton.icon(
                onPressed: isLoading ? null : onSubmit,
                icon: const Icon(Icons.auto_awesome_rounded, size: 18),
                label: const Text('Compare'),
                style: FilledButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space16,
                    vertical: AppTokens.space12,
                  ),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                ),
              ),
            ],
          ),
          if (error != null && error!.isNotEmpty) ...[
            const SizedBox(height: AppTokens.space12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: colors.errorBg,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: Border.all(color: colors.errorBorder),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.error_outline_rounded,
                    size: 18,
                    color: colors.errorText,
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Text(
                      error!,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        color: colors.errorText,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _SynopticResultView extends StatelessWidget {
  final SynopticResult result;
  final ValueChanged<SynopticCitation> onCitationTap;

  const _SynopticResultView({
    required this.result,
    required this.onCitationTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    final booksLabel = result.booksCovered == 1
        ? '1 book'
        : '${result.booksCovered} books';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        BentoCard(
          padding: const EdgeInsets.all(AppTokens.space16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.summarize_outlined,
                    size: 18,
                    color: colors.accentPrimary,
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Text(
                      'Executive Summary',
                      style: AppTypography.titleSerif(
                        fontSize: 17,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
                    ),
                  ),
                  StatusBadge(
                    label: booksLabel,
                    backgroundColor: colors.isDark
                        ? colors.accentSecondaryContainer
                        : AppTokens.completedBadgeBg,
                    textColor: colors.accentSecondary,
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space12),
              Text(
                result.summary.isNotEmpty
                    ? result.summary
                    : 'No synthesis summary returned.',
                style: AppTypography.readerBodySerif(
                  fontSize: 15,
                  lineHeight: 1.6,
                  color: colors.ink,
                ),
              ),
            ],
          ),
        ),
        if (result.consensusPoints.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space20),
          _SectionHeading(
            icon: Icons.handshake_outlined,
            title: 'Consensus Points',
            color: colors.accentSecondary,
          ),
          const SizedBox(height: AppTokens.space8),
          ...result.consensusPoints.map(
            (point) => Padding(
              padding: const EdgeInsets.only(bottom: AppTokens.space12),
              child: _SynopticPointCard(
                point: point,
                citations: result.citations,
                accentColor: colors.accentSecondary,
                onCitationTap: onCitationTap,
              ),
            ),
          ),
        ],
        if (result.divergencePoints.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space16),
          _SectionHeading(
            icon: Icons.call_split_rounded,
            title: 'Divergence Points',
            color: colors.accentPrimary,
          ),
          const SizedBox(height: AppTokens.space8),
          ...result.divergencePoints.map(
            (point) => Padding(
              padding: const EdgeInsets.only(bottom: AppTokens.space12),
              child: _SynopticPointCard(
                point: point,
                citations: result.citations,
                accentColor: colors.accentPrimary,
                onCitationTap: onCitationTap,
              ),
            ),
          ),
        ],
        if (result.citations.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space16),
          _SectionHeading(
            icon: Icons.menu_book_rounded,
            title: 'Citations',
            color: colors.secondaryInk,
          ),
          const SizedBox(height: AppTokens.space8),
          Wrap(
            spacing: AppTokens.space8,
            runSpacing: AppTokens.space8,
            children: result.citations
                .map(
                  (citation) => _CitationChip(
                    citation: citation,
                    onTap: () => onCitationTap(citation),
                  ),
                )
                .toList(),
          ),
        ],
      ],
    );
  }
}

class _SectionHeading extends StatelessWidget {
  final IconData icon;
  final String title;
  final Color color;

  const _SectionHeading({
    required this.icon,
    required this.title,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    return Row(
      children: [
        Icon(icon, size: 18, color: color),
        const SizedBox(width: AppTokens.space8),
        Text(
          title,
          style: AppTypography.titleSerif(
            fontSize: 16,
            fontWeight: FontWeight.w600,
            color: colors.ink,
          ),
        ),
      ],
    );
  }
}

class _SynopticPointCard extends StatelessWidget {
  final SynopticPoint point;
  final List<SynopticCitation> citations;
  final Color accentColor;
  final ValueChanged<SynopticCitation> onCitationTap;

  const _SynopticPointCard({
    required this.point,
    required this.citations,
    required this.accentColor,
    required this.onCitationTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    final linkedCitations = <SynopticCitation>[];
    for (final idx in point.citationIndices) {
      if (idx >= 0 && idx < citations.length) {
        linkedCitations.add(citations[idx]);
      }
    }

    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Container(
        decoration: BoxDecoration(
          border: Border(
            left: BorderSide(color: accentColor, width: 3),
          ),
        ),
        padding: const EdgeInsets.only(left: AppTokens.space12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (point.topic.isNotEmpty)
              Text(
                point.topic,
                style: AppTypography.titleSerif(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
            if (point.topic.isNotEmpty && point.description.isNotEmpty)
              const SizedBox(height: AppTokens.space4),
            if (point.description.isNotEmpty)
              Text(
                point.description,
                style: AppTypography.bodySans(
                  fontSize: 14,
                  color: colors.secondaryInk,
                ),
              ),
            if (linkedCitations.isNotEmpty) ...[
              const SizedBox(height: AppTokens.space12),
              Wrap(
                spacing: AppTokens.space8,
                runSpacing: AppTokens.space8,
                children: linkedCitations
                    .map(
                      (citation) => _CitationChip(
                        citation: citation,
                        onTap: () => onCitationTap(citation),
                      ),
                    )
                    .toList(),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _CitationChip extends StatelessWidget {
  final SynopticCitation citation;
  final VoidCallback onTap;

  const _CitationChip({
    required this.citation,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.shelfdColors;
    return ActionChip(
      onPressed: onTap,
      avatar: Icon(
        Icons.bookmark_outline_rounded,
        size: 16,
        color: colors.accentPrimary,
      ),
      label: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 240),
        child: Text(
          citation.chipLabel,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: AppTypography.mono(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            color: colors.ink,
          ),
        ),
      ),
      backgroundColor: colors.containerLow,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
        side: BorderSide(color: colors.subtleBorder),
      ),
    );
  }
}

class _EmptySynopticState extends StatelessWidget {
  final ShelfdColors colors;

  const _EmptySynopticState({required this.colors});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space16,
        vertical: AppTokens.space24,
      ),
      child: Column(
        children: [
          Icon(
            Icons.auto_stories_rounded,
            size: 44,
            color: colors.mutedInk,
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            'Compare ideas across books',
            textAlign: TextAlign.center,
            style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
          ),
          const SizedBox(height: AppTokens.space8),
          Text(
            'Select two or more books (or pick a genre) and ask how their themes, characters, or arguments agree and diverge.',
            textAlign: TextAlign.center,
            style: AppTypography.bodySans(
              fontSize: 13,
              color: colors.secondaryInk,
            ),
          ),
        ],
      ),
    );
  }
}
