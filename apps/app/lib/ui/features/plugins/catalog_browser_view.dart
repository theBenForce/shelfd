import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/plugin.dart';
import '../../core/app_shell.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../settings/plugin_config_dialog.dart';

class CatalogBrowserView extends ConsumerStatefulWidget {
  final String? initialPlugin;

  const CatalogBrowserView({
    super.key,
    this.initialPlugin,
  });

  @override
  ConsumerState<CatalogBrowserView> createState() => _CatalogBrowserViewState();
}

class _CatalogBrowserViewState extends ConsumerState<CatalogBrowserView> {
  final TextEditingController _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref
          .read(catalogBrowserProvider.notifier)
          .initialize(preferredPlugin: widget.initialPlugin);
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _openConfigDialog(PluginSummary plugin) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => PluginConfigDialog(plugin: plugin),
    );
    if (saved == true && mounted) {
      await ref.read(catalogBrowserProvider.notifier).loadCatalog(reset: true);
    }
  }

  Future<void> _handleStageSelected() async {
    final result =
        await ref.read(catalogBrowserProvider.notifier).stageSelected();
    if (!mounted || result == null) return;
    if (result.isAsync) {
      final msg = result.message ?? 'Staging started in background.';
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(msg),
          action: SnackBarAction(
            label: 'View Tasks',
            onPressed: () {
              if (mounted) {
                showBackgroundTasksPanel(context);
              }
            },
          ),
        ),
      );
      return;
    }

    final okCount = result.results.where((r) => !r.isFailed).length;
    final msg = result.message ??
        'Staged $okCount ${okCount == 1 ? 'book' : 'books'} for review.';
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(msg),
        action: SnackBarAction(
          label: 'Review Queue',
          onPressed: () {
            if (mounted) {
              context.go('/uploads');
            }
          },
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final pluginsState = ref.watch(pluginsProvider);
    final catalogState = ref.watch(catalogBrowserProvider);
    final uploadState = ref.watch(uploadProvider);
    final hasAppShell = context.findAncestorWidgetOfExactType<AppShell>() != null;

    final enabledCatalogPlugins = pluginsState.enabledCatalogPlugins;
    final selectedPlugin =
        pluginsState.findByName(catalogState.selectedPluginName);
    final stagedCount = uploadState.stagedJobs.length;

    final topBar = AppBar(
      backgroundColor: colors.background,
      elevation: 0,
      scrolledUnderElevation: 0,
      automaticallyImplyLeading: false,
      titleSpacing: AppTokens.space24,
      title: Row(
        children: [
          IconButton(
            tooltip: 'Back to Uploads',
            onPressed: () => context.go('/uploads'),
            icon: Icon(Icons.arrow_back_rounded, size: 20, color: colors.ink),
          ),
          const SizedBox(width: AppTokens.space8),
          Expanded(
            child: Text(
              'Storefront Catalog',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.titleSerif(fontSize: 22, color: colors.ink),
            ),
          ),
        ],
      ),
      actions: [
        OutlinedButton.icon(
          key: const Key('catalog_open_staged_queue_button'),
          style: OutlinedButton.styleFrom(
            foregroundColor: colors.ink,
            side: BorderSide(color: colors.border),
          ),
          onPressed: () => context.go('/uploads'),
          icon: const Icon(Icons.fact_check_outlined, size: 16),
          label: Text(
            stagedCount > 0 ? 'Staged Queue ($stagedCount)' : 'Staged Queue',
          ),
        ),
        const SizedBox(width: AppTokens.space8),
        TextButton.icon(
          onPressed: () => context.go('/settings'),
          icon: Icon(Icons.extension_outlined, size: 16, color: colors.secondaryInk),
          label: Text(
            'Plugins',
            style: AppTypography.bodySans(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: colors.secondaryInk,
            ),
          ),
        ),
        const SizedBox(width: AppTokens.space16),
      ],
    );

    final body = Align(
      alignment: Alignment.topCenter,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 1200),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: enabledCatalogPlugins.isEmpty
              ? _buildNoPluginsEmptyState(context, colors, pluginsState.isLoading)
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildSourceSelectorBar(
                      context,
                      colors,
                      enabledCatalogPlugins,
                      selectedPlugin,
                    ),
                    const SizedBox(height: AppTokens.space16),
                    if (selectedPlugin != null) ...[
                      _buildSearchAndFilters(
                        context,
                        colors,
                        selectedPlugin,
                        catalogState,
                      ),
                      const SizedBox(height: AppTokens.space16),
                    ],
                    if (catalogState.isAuthExpired && selectedPlugin != null) ...[
                      _buildAuthExpiredCard(
                        colors,
                        selectedPlugin,
                        catalogState.errorMessage,
                      ),
                      const SizedBox(height: AppTokens.space16),
                    ] else if (catalogState.errorMessage != null) ...[
                      _buildErrorBanner(colors, catalogState.errorMessage!),
                      const SizedBox(height: AppTokens.space16),
                    ],
                    _buildSelectionToolbar(context, colors, catalogState),
                    const SizedBox(height: AppTokens.space12),
                    if (catalogState.isLoading)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: AppTokens.space48),
                        child: Center(child: CircularProgressIndicator()),
                      )
                    else if (catalogState.items.isEmpty)
                      _buildEmptyResults(colors, catalogState)
                    else ...[
                      _buildItemsGrid(context, colors, catalogState),
                      if (catalogState.hasMore) ...[
                        const SizedBox(height: AppTokens.space20),
                        Center(
                          child: OutlinedButton.icon(
                            key: const Key('catalog_load_more_button'),
                            style: OutlinedButton.styleFrom(
                              foregroundColor: colors.ink,
                              side: BorderSide(color: colors.border),
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppTokens.space20,
                                vertical: AppTokens.space12,
                              ),
                            ),
                            onPressed: catalogState.isLoadingMore
                                ? null
                                : () => ref
                                    .read(catalogBrowserProvider.notifier)
                                    .loadMore(),
                            icon: catalogState.isLoadingMore
                                ? SizedBox(
                                    width: 14,
                                    height: 14,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      color: colors.accentPrimary,
                                    ),
                                  )
                                : const Icon(Icons.expand_more_rounded, size: 18),
                            label: Text(
                              catalogState.isLoadingMore
                                  ? 'Loading...'
                                  : 'Load More Books',
                            ),
                          ),
                        ),
                      ],
                    ],
                  ],
                ),
        ),
      ),
    );

    return Scaffold(
      backgroundColor: hasAppShell ? Colors.transparent : colors.background,
      appBar: topBar,
      body: body,
    );
  }

  Widget _buildNoPluginsEmptyState(
    BuildContext context,
    ShelfdColors colors,
    bool isLoading,
  ) {
    if (isLoading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: AppTokens.space48),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space32),
      child: Column(
        children: [
          Icon(Icons.storefront_outlined, size: 36, color: colors.mutedInk),
          const SizedBox(height: AppTokens.space12),
          Text(
            'No Storefront Plugins Enabled',
            style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space8),
          Text(
            'Enable Project Gutenberg or install a storefront plugin (.zip) in Settings to browse and stage books.',
            style: AppTypography.bodySans(fontSize: 13, color: colors.mutedInk),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: AppTokens.space16),
          OutlinedButton.icon(
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.ink,
              side: BorderSide(color: colors.border),
            ),
            onPressed: () => context.go('/settings'),
            icon: const Icon(Icons.settings_outlined, size: 16),
            label: const Text('Open Plugin Settings'),
          ),
        ],
      ),
    );
  }

  Widget _buildSourceSelectorBar(
    BuildContext context,
    ShelfdColors colors,
    List<PluginSummary> plugins,
    PluginSummary? selectedPlugin,
  ) {
    return Row(
      children: [
        Expanded(
          child: FilterPillsRow(
            items: plugins
                .map((p) => FilterPillItem(id: p.name, label: p.displayName))
                .toList(),
            selectedId: selectedPlugin?.name ?? plugins.first.name,
            onSelected: (id) {
              _searchController.clear();
              ref.read(catalogBrowserProvider.notifier).selectPlugin(id);
            },
          ),
        ),
        if (selectedPlugin != null &&
            (selectedPlugin.hasConfigSchema ||
                selectedPlugin.commands.isNotEmpty)) ...[
          const SizedBox(width: AppTokens.space12),
          OutlinedButton.icon(
            key: const Key('catalog_configure_plugin_button'),
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.ink,
              side: BorderSide(color: colors.border),
            ),
            onPressed: () => _openConfigDialog(selectedPlugin),
            icon: const Icon(Icons.tune_rounded, size: 16),
            label: const Text('Configure'),
          ),
        ],
      ],
    );
  }

  Widget _buildSearchAndFilters(
    BuildContext context,
    ShelfdColors colors,
    PluginSummary plugin,
    CatalogBrowserState state,
  ) {
    final modes = plugin.resolvedCatalogModes;
    final supportsSearch = modes.contains('search');
    final supportsBoth = modes.contains('browse') && supportsSearch;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (supportsBoth) ...[
          FilterPillsRow(
            items: const [
              FilterPillItem(id: 'browse', label: 'Browse'),
              FilterPillItem(id: 'search', label: 'Search'),
            ],
            selectedId: state.mode,
            onSelected: (m) =>
                ref.read(catalogBrowserProvider.notifier).setMode(m),
          ),
          const SizedBox(height: AppTokens.space12),
        ],
        if (supportsSearch)
          Row(
            children: [
              Expanded(
                child: TextField(
                  key: const Key('catalog_search_input'),
                  controller: _searchController,
                  style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                  onSubmitted: (val) =>
                      ref.read(catalogBrowserProvider.notifier).search(val),
                  decoration: InputDecoration(
                    hintText: plugin.catalog?.searchHint ??
                        'Search ${plugin.displayName} by title, author, or topic...',
                    isDense: true,
                    prefixIcon:
                        Icon(Icons.search_rounded, size: 18, color: colors.mutedInk),
                    suffixIcon: _searchController.text.isNotEmpty
                        ? IconButton(
                            tooltip: 'Clear search',
                            icon: Icon(Icons.clear_rounded,
                                size: 16, color: colors.mutedInk),
                            onPressed: () {
                              _searchController.clear();
                              setState(() {});
                              ref
                                  .read(catalogBrowserProvider.notifier)
                                  .search('');
                            },
                          )
                        : null,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                      borderSide: BorderSide(color: colors.border),
                    ),
                    enabledBorder: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                      borderSide: BorderSide(color: colors.border),
                    ),
                  ),
                  onChanged: (_) => setState(() {}),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              FilledButton.icon(
                key: const Key('catalog_search_submit_button'),
                style: FilledButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space16,
                    vertical: AppTokens.space12,
                  ),
                ),
                onPressed: state.isLoading
                    ? null
                    : () => ref
                        .read(catalogBrowserProvider.notifier)
                        .search(_searchController.text),
                icon: const Icon(Icons.search_rounded, size: 16),
                label: const Text('Search'),
              ),
            ],
          ),
        if (state.groups.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space12),
          FilterPillsRow(
            items: [
              const FilterPillItem(id: '', label: 'All'),
              ...state.groups.map(
                (g) => FilterPillItem(id: g.id, label: g.displayLabel),
              ),
            ],
            selectedId: state.selectedGroup,
            onSelected: (gid) =>
                ref.read(catalogBrowserProvider.notifier).selectGroup(gid),
          ),
        ],
      ],
    );
  }

  Widget _buildAuthExpiredCard(
    ShelfdColors colors,
    PluginSummary plugin,
    String? message,
  ) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppTokens.space16),
      decoration: BoxDecoration(
        color: colors.errorBg,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        border: Border.all(color: colors.errorBorder),
      ),
      child: Row(
        children: [
          Icon(Icons.lock_outline_rounded, size: 20, color: colors.errorText),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Storefront Authentication Required',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.errorText,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  message ??
                      'Your session for ${plugin.displayName} is missing or expired.',
                  style: AppTypography.captionSans(color: colors.errorText),
                ),
              ],
            ),
          ),
          const SizedBox(width: AppTokens.space12),
          OutlinedButton.icon(
            key: const Key('catalog_update_credentials_button'),
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.errorText,
              side: BorderSide(color: colors.errorBorder),
            ),
            onPressed: () => _openConfigDialog(plugin),
            icon: const Icon(Icons.key_rounded, size: 16),
            label: const Text('Update Credentials'),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorBanner(ShelfdColors colors, String error) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppTokens.space12),
      decoration: BoxDecoration(
        color: colors.errorBg,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(color: colors.errorBorder),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline_rounded, size: 18, color: colors.errorText),
          const SizedBox(width: AppTokens.space8),
          Expanded(
            child: Text(
              error,
              style: AppTypography.bodySans(fontSize: 13, color: colors.errorText),
            ),
          ),
          const SizedBox(width: AppTokens.space8),
          TextButton(
            onPressed: () =>
                ref.read(catalogBrowserProvider.notifier).loadCatalog(reset: true),
            child: Text(
              'Retry',
              style: AppTypography.bodySans(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: colors.errorText,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSelectionToolbar(
    BuildContext context,
    ShelfdColors colors,
    CatalogBrowserState state,
  ) {
    final availableCount = state.items
        .where(
          (i) =>
              i.isAvailable && !state.stagingExternalIds.contains(i.externalId),
        )
        .length;
    final selectedCount = state.selectedExternalIds.length;
    final countLabel = state.totalCount != null
        ? 'Showing ${state.items.length} of ${state.totalCount}'
        : '${state.items.length} ${state.items.length == 1 ? 'book' : 'books'}';

    return Row(
      children: [
        Expanded(
          child: Text(
            countLabel,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.captionSans(color: colors.mutedInk),
          ),
        ),
        if (availableCount > 0) ...[
          TextButton(
            onPressed: selectedCount == availableCount
                ? () => ref.read(catalogBrowserProvider.notifier).clearSelection()
                : () =>
                    ref.read(catalogBrowserProvider.notifier).selectAllAvailable(),
            child: Text(
              selectedCount == availableCount ? 'Clear Selection' : 'Select All',
              style: AppTypography.bodySans(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: colors.secondaryInk,
              ),
            ),
          ),
          const SizedBox(width: AppTokens.space8),
        ],
        FilledButton.icon(
          key: const Key('stage_selected_catalog_button'),
          style: FilledButton.styleFrom(
            backgroundColor: colors.accentPrimary,
            foregroundColor: colors.onAccent,
            padding: const EdgeInsets.symmetric(
              horizontal: AppTokens.space16,
              vertical: AppTokens.space12,
            ),
          ),
          onPressed:
              (selectedCount > 0 && !state.isStaging) ? _handleStageSelected : null,
          icon: const Icon(Icons.download_for_offline_outlined, size: 16),
          label: Text('Stage Selected ($selectedCount)'),
        ),
      ],
    );
  }

  Widget _buildEmptyResults(ShelfdColors colors, CatalogBrowserState state) {
    return BentoCard(
      padding: const EdgeInsets.all(AppTokens.space32),
      child: Center(
        child: Column(
          children: [
            Icon(Icons.menu_book_outlined, size: 32, color: colors.mutedInk),
            const SizedBox(height: AppTokens.space8),
            Text(
              state.mode == 'search' && state.query.isEmpty
                  ? 'Enter a search query above to find books.'
                  : 'No catalog items matched your filter.',
              style: AppTypography.bodySans(
                fontSize: 14,
                color: colors.mutedInk,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildItemsGrid(
    BuildContext context,
    ShelfdColors colors,
    CatalogBrowserState state,
  ) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final maxWidth = constraints.maxWidth;
        final columns = maxWidth >= 860 ? 2 : 1;
        const spacing = AppTokens.space12;
        final cardWidth =
            columns == 1 ? maxWidth : (maxWidth - spacing) / 2.0;

        return Wrap(
          spacing: spacing,
          runSpacing: spacing,
          children: state.items.map((item) {
            final isSelected =
                state.selectedExternalIds.contains(item.externalId);
            final isItemStaging = state.isItemStaging(item.externalId);
            return SizedBox(
              width: cardWidth,
              child: _buildCatalogCard(
                context,
                colors,
                item,
                isSelected,
                isItemStaging,
              ),
            );
          }).toList(),
        );
      },
    );
  }

  Widget _buildCatalogCard(
    BuildContext context,
    ShelfdColors colors,
    CatalogItem item,
    bool isSelected,
    bool isItemStaging,
  ) {
    final canSelect = item.isAvailable && !isItemStaging;
    final authorText =
        item.authors.isNotEmpty ? item.authors.join(', ') : 'Unknown Author';

    return BentoCard(
      key: Key('catalog_item_${item.externalId}'),
      backgroundColor: isSelected ? colors.accentContainer : colors.surface,
      onTap: canSelect
          ? () => ref
              .read(catalogBrowserProvider.notifier)
              .toggleItemSelection(item.externalId)
          : (isItemStaging
              ? () => showBackgroundTasksPanel(context)
              : (item.isInLibrary && item.bookId != null
                  ? () => context.go('/books/${item.bookId}')
                  : (item.isStaged ? () => context.go('/uploads') : null))),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            child: Container(
              width: 56,
              height: 80,
              color: colors.containerLow,
              child: (item.coverUrl != null && item.coverUrl!.isNotEmpty)
                  ? Image.network(
                      item.coverUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (_, _, _) => Icon(
                        Icons.book_outlined,
                        size: 24,
                        color: colors.mutedInk,
                      ),
                    )
                  : Icon(
                      Icons.book_outlined,
                      size: 24,
                      color: colors.mutedInk,
                    ),
            ),
          ),
          const SizedBox(width: AppTokens.space12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Text(
                        item.title,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.titleSerif(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    _buildProvenanceControl(
                      colors,
                      item,
                      isSelected,
                      isItemStaging,
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  authorText,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 12,
                    color: colors.secondaryInk,
                  ),
                ),
                if (item.group != null && item.group!.isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Text(
                    item.group!,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.captionSans(color: colors.mutedInk),
                  ),
                ],
                if (item.formats.isNotEmpty) ...[
                  const SizedBox(height: AppTokens.space8),
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: item.formats.map((f) {
                      return Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: colors.containerLow,
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                          border: Border.all(color: colors.subtleBorder),
                        ),
                        child: Text(
                          f.displayLabel,
                          style: AppTypography.mono(
                            fontSize: 10,
                            fontWeight: FontWeight.w600,
                            color: colors.secondaryInk,
                          ),
                        ),
                      );
                    }).toList(),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildProvenanceControl(
    ShelfdColors colors,
    CatalogItem item,
    bool isSelected,
    bool isItemStaging,
  ) {
    if (item.isInLibrary) {
      return StatusBadge(
        label: 'In Library',
        backgroundColor: colors.accentSecondaryContainer,
        textColor: colors.accentSecondary,
      );
    }
    if (item.isStaged) {
      return StatusBadge(
        label: 'Staged',
        backgroundColor: colors.accentContainer,
        textColor: colors.accentPrimary,
      );
    }
    if (isItemStaging) {
      return StatusBadge(
        label: 'Staging...',
        backgroundColor: colors.accentContainer,
        textColor: colors.accentPrimary,
      );
    }
    return Checkbox(
      key: Key('catalog_checkbox_${item.externalId}'),
      value: isSelected,
      activeColor: colors.accentPrimary,
      checkColor: colors.onAccent,
      visualDensity: VisualDensity.compact,
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      onChanged: (_) => ref
          .read(catalogBrowserProvider.notifier)
          .toggleItemSelection(item.externalId),
    );
  }
}
