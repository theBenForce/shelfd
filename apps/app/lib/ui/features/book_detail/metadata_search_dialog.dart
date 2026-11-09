import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/metadata_search_result.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

/// Shows the metadata search and auto-populate dialog or bottom sheet.
Future<MetadataSearchResult?> showMetadataSearchDialog(
  BuildContext context, {
  String? initialTitle,
  String? initialAuthor,
}) async {
  final isMobile = Responsive.isMobile(context);
  final colors = ShelfdColors.of(context);

  if (isMobile) {
    return showModalBottomSheet<MetadataSearchResult>(
      context: context,
      isScrollControlled: true,
      backgroundColor: colors.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.of(ctx).viewInsets.bottom),
        child: MetadataSearchDialogContent(
          initialTitle: initialTitle,
          initialAuthor: initialAuthor,
          isBottomSheet: true,
        ),
      ),
    );
  }

  return showDialog<MetadataSearchResult>(
    context: context,
    builder: (ctx) {
      final dialogColors = ShelfdColors.of(ctx);
      return Dialog(
        backgroundColor: dialogColors.surface,
        surfaceTintColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
          side: BorderSide(color: dialogColors.border),
        ),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760, maxHeight: 750),
          child: MetadataSearchDialogContent(
            initialTitle: initialTitle,
            initialAuthor: initialAuthor,
            isBottomSheet: false,
          ),
        ),
      );
    },
  );
}

class MetadataSearchDialogContent extends ConsumerStatefulWidget {
  final String? initialTitle;
  final String? initialAuthor;
  final bool isBottomSheet;

  const MetadataSearchDialogContent({
    super.key,
    this.initialTitle,
    this.initialAuthor,
    this.isBottomSheet = false,
  });

  @override
  ConsumerState<MetadataSearchDialogContent> createState() =>
      _MetadataSearchDialogContentState();
}

class _MetadataSearchDialogContentState
    extends ConsumerState<MetadataSearchDialogContent> {
  late final TextEditingController _queryController;
  String _selectedProvider = 'all';
  bool _isLoading = false;
  String? _errorMessage;
  List<MetadataSearchResult>? _results;

  @override
  void initState() {
    super.initState();
    final parts = <String>[];
    if (widget.initialTitle != null && widget.initialTitle!.trim().isNotEmpty) {
      parts.add(widget.initialTitle!.trim());
    }
    if (widget.initialAuthor != null &&
        widget.initialAuthor!.trim().isNotEmpty &&
        widget.initialAuthor != 'Unknown') {
      parts.add(widget.initialAuthor!.trim());
    }
    _queryController = TextEditingController(text: parts.join(' '));

    if (parts.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _performSearch();
      });
    }
  }

  @override
  void dispose() {
    _queryController.dispose();
    super.dispose();
  }

  Future<void> _performSearch() async {
    final q = _queryController.text.trim();
    if (q.isEmpty) return;

    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    try {
      final repo = ref.read(bookRepositoryProvider);
      final results = await repo.searchMetadata(
        query: q,
        provider: _selectedProvider == 'all' ? null : _selectedProvider,
      );
      if (mounted) {
        setState(() {
          _results = results;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return Container(
      padding: const EdgeInsets.all(AppTokens.space24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Header
          Row(
            children: [
              Icon(Icons.auto_fix_high, color: colors.accentPrimary, size: 24),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Search Metadata',
                      style: AppTypography.titleSerif(fontSize: 20, color: colors.ink),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      'Search Open Library, Google Books, and Apple Books to autofill details.',
                      style: AppTypography.bodySans(fontSize: 13, color: colors.secondaryInk),
                    ),
                  ],
                ),
              ),
              IconButton(
                icon: Icon(Icons.close, color: colors.mutedInk),
                onPressed: () => Navigator.of(context).pop(),
                splashRadius: 20,
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          // Search Inputs
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _queryController,
                  style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                  decoration: InputDecoration(
                    hintText: 'Search title, author, or ISBN...',
                    hintStyle: AppTypography.bodySans(fontSize: 14, color: colors.mutedInk),
                    prefixIcon: Icon(Icons.search, size: 20, color: colors.mutedInk),
                    filled: true,
                    fillColor: colors.containerLow,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space16,
                      vertical: AppTokens.space12,
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
                  ),
                  onSubmitted: (_) => _performSearch(),
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              DropdownButtonHideUnderline(
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
                  decoration: BoxDecoration(
                    color: colors.containerLow,
                    border: Border.all(color: colors.border),
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  child: DropdownButton<String>(
                    value: _selectedProvider,
                    dropdownColor: colors.surface,
                    style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                    items: const [
                      DropdownMenuItem(value: 'all', child: Text('All Providers')),
                      DropdownMenuItem(value: 'openlibrary', child: Text('Open Library')),
                      DropdownMenuItem(value: 'googlebooks', child: Text('Google Books')),
                      DropdownMenuItem(value: 'apple', child: Text('Apple Books')),
                    ],
                    onChanged: (val) {
                      if (val != null) {
                        setState(() => _selectedProvider = val);
                        _performSearch();
                      }
                    },
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              ElevatedButton.icon(
                icon: _isLoading
                    ? SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: colors.onAccent,
                        ),
                      )
                    : const Icon(Icons.search, size: 18),
                label: const Text('Search'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space20,
                    vertical: AppTokens.space16,
                  ),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                ),
                onPressed: _isLoading ? null : _performSearch,
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space16),

          if (_errorMessage != null)
            Container(
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: colors.errorBg,
                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                border: Border.all(color: colors.errorBorder),
              ),
              child: Text(
                _errorMessage!,
                style: AppTypography.bodySans(color: colors.errorText, fontSize: 13),
              ),
            ),

          // Results area
          Expanded(
            child: _isLoading && (_results == null || _results!.isEmpty)
                ? Center(child: CircularProgressIndicator(color: colors.accentPrimary))
                : _results == null
                    ? Center(
                        child: Text(
                          'Type a title or author above and press Search.',
                          style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
                        ),
                      )
                    : _results!.isEmpty
                        ? Center(
                            child: Text(
                              'No metadata matches found.',
                              style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
                            ),
                          )
                        : ListView.separated(
                            itemCount: _results!.length,
                            separatorBuilder: (ctx, i) =>
                                Divider(height: 1, color: colors.subtleBorder),
                            itemBuilder: (ctx, i) {
                              final item = _results![i];
                              return _SearchResultTile(
                                result: item,
                                onSelect: () => Navigator.of(context).pop(item),
                              );
                            },
                          ),
          ),
        ],
      ),
    );
  }
}

class _SearchResultTile extends StatelessWidget {
  final MetadataSearchResult result;
  final VoidCallback onSelect;

  const _SearchResultTile({
    required this.result,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return InkWell(
      onTap: onSelect,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          vertical: AppTokens.space12,
          horizontal: AppTokens.space8,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Cover thumbnail
            TactileBookCover(
              coverUrl: result.coverUrl,
              title: result.title,
              width: 52,
              height: 76,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              showFallbackTitle: false,
              showSpineCrease: true,
              showShadow: false,
            ),
            const SizedBox(width: AppTokens.space16),

            // Metadata details
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          result.title,
                          style: AppTypography.titleSerif(fontSize: 16, color: colors.ink),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 8),
                      _ProviderBadge(provider: result.provider),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    result.author.isNotEmpty ? result.author : 'Unknown Author',
                    style: AppTypography.bodySans(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      color: colors.secondaryInk,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),

                  // Series / Publisher / Year line
                  Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      if (result.series != null && result.series!.isNotEmpty)
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: colors.container,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            'Series: ${result.series}${result.seriesSequence != null ? ' #${result.seriesSequence}' : ''}',
                            style: AppTypography.mono(
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                              color: colors.ink,
                            ),
                          ),
                        ),
                      if (result.publishedYear != null)
                        Text(
                          '${result.publishedYear}',
                          style: AppTypography.mono(fontSize: 12, color: colors.secondaryInk),
                        ),
                      if (result.publisher != null && result.publisher!.isNotEmpty)
                        Text(
                          '• ${result.publisher}',
                          style: AppTypography.bodySans(fontSize: 12, color: colors.secondaryInk),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      if (result.isbn != null && result.isbn!.isNotEmpty)
                        Text(
                          '• ISBN: ${result.isbn}',
                          style: AppTypography.mono(fontSize: 12, color: colors.mutedInk),
                        ),
                    ],
                  ),

                  if (result.description != null && result.description!.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      result.description!,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.bodySans(
                        fontSize: 12,
                        color: colors.mutedInk,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: AppTokens.space12),

            // Select button
            OutlinedButton(
              onPressed: onSelect,
              style: OutlinedButton.styleFrom(
                foregroundColor: colors.ink,
                side: BorderSide(color: colors.border),
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              ),
              child: const Text('Apply'),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProviderBadge extends StatelessWidget {
  final String provider;

  const _ProviderBadge({required this.provider});

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    String label = provider;
    Color bg = colors.container;
    Color fg = colors.mutedInk;

    if (provider == 'openlibrary') {
      label = 'Open Library';
      bg = colors.accentContainer;
      fg = colors.accentPrimary;
    } else if (provider == 'googlebooks') {
      label = 'Google Books';
      bg = AppTokens.amberAccent.withValues(alpha: colors.isDark ? 0.22 : 0.2);
      fg = colors.isDark ? AppTokens.amberAccent : const Color(0xFFB45309);
    } else if (provider == 'apple') {
      label = 'Apple Books';
      bg = colors.accentSecondaryContainer;
      fg = colors.accentSecondary;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
      ),
      child: Text(
        label,
        style: AppTypography.mono(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: fg,
        ),
      ),
    );
  }
}
