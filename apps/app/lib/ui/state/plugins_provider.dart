import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/plugin.dart';
import '../../data/services/api_service.dart';
import 'providers.dart';

class PluginsState {
  final List<PluginSummary> plugins;
  final bool isLoading;
  final bool isInstalling;
  final String? errorMessage;
  final String? successMessage;

  const PluginsState({
    this.plugins = const [],
    this.isLoading = false,
    this.isInstalling = false,
    this.errorMessage,
    this.successMessage,
  });

  List<PluginSummary> get enabledCatalogPlugins =>
      plugins.where((p) => p.enabled && p.hasCatalog).toList();

  PluginSummary? findByName(String? name) {
    if (name == null || name.isEmpty) return null;
    for (final plugin in plugins) {
      if (plugin.name == name) return plugin;
    }
    return null;
  }

  PluginsState copyWith({
    List<PluginSummary>? plugins,
    bool? isLoading,
    bool? isInstalling,
    String? errorMessage,
    bool clearError = false,
    String? successMessage,
    bool clearSuccess = false,
  }) {
    return PluginsState(
      plugins: plugins ?? this.plugins,
      isLoading: isLoading ?? this.isLoading,
      isInstalling: isInstalling ?? this.isInstalling,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      successMessage:
          clearSuccess ? null : (successMessage ?? this.successMessage),
    );
  }
}

class PluginsNotifier extends Notifier<PluginsState> {
  bool _isDisposed = false;

  @override
  PluginsState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    ref.listen(authProvider, (previous, next) {
      if (previous?.user != next.user && next.user != null) {
        loadPlugins(silent: true);
      }
    });
    Future.microtask(() => loadPlugins(silent: true));
    return const PluginsState(isLoading: true);
  }

  Future<void> loadPlugins({bool silent = false}) async {
    if (_isDisposed) return;
    state = state.copyWith(
      isLoading: true,
      clearError: true,
      clearSuccess: silent ? false : true,
    );
    try {
      final api = ref.read(apiServiceProvider);
      final list = await api.getPlugins();
      if (!_isDisposed) {
        state = state.copyWith(plugins: list, isLoading: false);
      }
    } catch (e) {
      if (!_isDisposed) {
        state = state.copyWith(
          isLoading: false,
          errorMessage: silent ? null : 'Failed to load plugins: $e',
        );
      }
    }
  }

  Future<void> discover() async {
    if (_isDisposed) return;
    state = state.copyWith(
      isLoading: true,
      clearError: true,
      clearSuccess: true,
    );
    try {
      final api = ref.read(apiServiceProvider);
      final list = await api.discoverPlugins();
      if (!_isDisposed) {
        state = state.copyWith(
          plugins: list,
          isLoading: false,
          successMessage: 'Plugin directory rescanned (${list.length} found).',
        );
      }
    } catch (e) {
      if (!_isDisposed) {
        state = state.copyWith(
          isLoading: false,
          errorMessage: 'Failed to rescan plugins: $e',
        );
      }
    }
  }

  Future<PluginSummary?> installZip({
    required String filename,
    required List<int> bytes,
  }) async {
    if (_isDisposed) return null;
    state = state.copyWith(
      isInstalling: true,
      clearError: true,
      clearSuccess: true,
    );
    try {
      final api = ref.read(apiServiceProvider);
      final installed = await api.installPluginZip(
        filename: filename,
        bytes: bytes,
      );
      final list = await api.getPlugins();
      if (!_isDisposed) {
        state = state.copyWith(
          plugins: list,
          isInstalling: false,
          successMessage:
              'Installed ${installed.displayName} v${installed.version}.',
        );
      }
      return installed;
    } catch (e) {
      if (!_isDisposed) {
        state = state.copyWith(
          isInstalling: false,
          errorMessage: 'Failed to install plugin: $e',
        );
      }
      return null;
    }
  }

  Future<bool> setEnabled(String name, bool enabled) async {
    if (_isDisposed) return false;
    state = state.copyWith(clearError: true, clearSuccess: true);
    try {
      final api = ref.read(apiServiceProvider);
      final updated = await api.setPluginEnabled(name, enabled);
      if (!_isDisposed) {
        final next = state.plugins
            .map((p) => p.name == name ? updated : p)
            .toList();
        state = state.copyWith(
          plugins: next,
          successMessage: enabled
              ? 'Enabled ${updated.displayName}.'
              : 'Disabled ${updated.displayName}.',
        );
      }
      return true;
    } catch (e) {
      if (!_isDisposed) {
        state = state.copyWith(
          errorMessage: 'Failed to update plugin "$name": $e',
        );
      }
      return false;
    }
  }

  Future<PluginDetail> fetchDetail(String name) {
    return ref.read(apiServiceProvider).getPluginDetail(name);
  }

  Future<PluginConfigResponse> fetchConfig(String name) {
    return ref.read(apiServiceProvider).getPluginConfig(name);
  }

  Future<PluginConfigResponse> saveConfig(
    String name,
    Map<String, dynamic> values,
  ) async {
    final api = ref.read(apiServiceProvider);
    final saved = await api.updatePluginConfig(name, values);
    await loadPlugins(silent: true);
    return saved;
  }
}

final pluginsProvider = NotifierProvider<PluginsNotifier, PluginsState>(
  PluginsNotifier.new,
);

class CatalogBrowserState {
  final String? selectedPluginName;
  final String mode;
  final String query;
  final String selectedGroup;
  final List<CatalogItem> items;
  final List<CatalogGroup> groups;
  final String? nextCursor;
  final int? totalCount;
  final Set<String> selectedExternalIds;
  final Map<String, String> preferredFormats;
  final bool isLoading;
  final bool isLoadingMore;
  final bool isStaging;
  final String? stagingMessage;
  final String? errorMessage;
  final bool isAuthExpired;

  const CatalogBrowserState({
    this.selectedPluginName,
    this.mode = 'browse',
    this.query = '',
    this.selectedGroup = '',
    this.items = const [],
    this.groups = const [],
    this.nextCursor,
    this.totalCount,
    this.selectedExternalIds = const {},
    this.preferredFormats = const {},
    this.isLoading = false,
    this.isLoadingMore = false,
    this.isStaging = false,
    this.stagingMessage,
    this.errorMessage,
    this.isAuthExpired = false,
  });

  bool get hasMore => nextCursor != null && nextCursor!.isNotEmpty;

  CatalogBrowserState copyWith({
    String? selectedPluginName,
    bool clearSelectedPlugin = false,
    String? mode,
    String? query,
    String? selectedGroup,
    List<CatalogItem>? items,
    List<CatalogGroup>? groups,
    String? nextCursor,
    bool clearNextCursor = false,
    int? totalCount,
    bool clearTotalCount = false,
    Set<String>? selectedExternalIds,
    Map<String, String>? preferredFormats,
    bool? isLoading,
    bool? isLoadingMore,
    bool? isStaging,
    String? stagingMessage,
    bool clearStagingMessage = false,
    String? errorMessage,
    bool clearError = false,
    bool? isAuthExpired,
  }) {
    return CatalogBrowserState(
      selectedPluginName: clearSelectedPlugin
          ? null
          : (selectedPluginName ?? this.selectedPluginName),
      mode: mode ?? this.mode,
      query: query ?? this.query,
      selectedGroup: selectedGroup ?? this.selectedGroup,
      items: items ?? this.items,
      groups: groups ?? this.groups,
      nextCursor: clearNextCursor ? null : (nextCursor ?? this.nextCursor),
      totalCount: clearTotalCount ? null : (totalCount ?? this.totalCount),
      selectedExternalIds: selectedExternalIds ?? this.selectedExternalIds,
      preferredFormats: preferredFormats ?? this.preferredFormats,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      isStaging: isStaging ?? this.isStaging,
      stagingMessage: clearStagingMessage
          ? null
          : (stagingMessage ?? this.stagingMessage),
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      isAuthExpired: isAuthExpired ?? this.isAuthExpired,
    );
  }
}

class CatalogBrowserNotifier extends Notifier<CatalogBrowserState> {
  bool _isDisposed = false;

  @override
  CatalogBrowserState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    return const CatalogBrowserState();
  }

  Future<void> initialize({String? preferredPlugin}) async {
    if (_isDisposed) return;
    var pluginsState = ref.read(pluginsProvider);
    if (pluginsState.plugins.isEmpty) {
      await ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
      if (_isDisposed) return;
      pluginsState = ref.read(pluginsProvider);
    }

    final catalogPlugins = pluginsState.enabledCatalogPlugins;
    if (catalogPlugins.isEmpty) {
      state = state.copyWith(
        clearSelectedPlugin: true,
        items: const [],
        groups: const [],
        selectedExternalIds: const {},
        clearNextCursor: true,
        clearError: true,
        isAuthExpired: false,
      );
      return;
    }

    String targetName = catalogPlugins.first.name;
    if (preferredPlugin != null &&
        catalogPlugins.any((p) => p.name == preferredPlugin)) {
      targetName = preferredPlugin;
    } else if (state.selectedPluginName != null &&
        catalogPlugins.any((p) => p.name == state.selectedPluginName)) {
      targetName = state.selectedPluginName!;
    }

    await selectPlugin(targetName);
  }

  Future<void> selectPlugin(String pluginName) async {
    if (_isDisposed) return;
    final summary = ref.read(pluginsProvider).findByName(pluginName);
    final modes = summary?.resolvedCatalogModes ?? const ['browse'];
    final defaultMode = modes.contains('browse') ? 'browse' : modes.first;

    state = CatalogBrowserState(
      selectedPluginName: pluginName,
      mode: defaultMode,
    );

    if (summary != null &&
        (summary.catalog?.requiresAuth ?? false) &&
        !summary.configured) {
      state = state.copyWith(
        isAuthExpired: true,
        errorMessage:
            'Configure credentials for ${summary.displayName} to browse its catalog.',
      );
      return;
    }

    if (defaultMode == 'browse') {
      await loadCatalog(reset: true);
    }
  }

  Future<void> setMode(String mode) async {
    if (_isDisposed || state.mode == mode) return;
    state = state.copyWith(
      mode: mode,
      selectedExternalIds: const {},
      clearError: true,
      isAuthExpired: false,
    );
    if (mode == 'browse' || state.query.trim().isNotEmpty) {
      await loadCatalog(reset: true);
    }
  }

  Future<void> search(String query) async {
    if (_isDisposed) return;
    final trimmed = query.trim();
    final summary =
        ref.read(pluginsProvider).findByName(state.selectedPluginName);
    final modes = summary?.resolvedCatalogModes ?? const ['browse'];
    final nextMode = (trimmed.isNotEmpty && modes.contains('search'))
        ? 'search'
        : (modes.contains('browse') ? 'browse' : state.mode);

    state = state.copyWith(
      query: trimmed,
      mode: nextMode,
      selectedExternalIds: const {},
      clearError: true,
      isAuthExpired: false,
    );
    await loadCatalog(reset: true);
  }

  Future<void> selectGroup(String groupId) async {
    if (_isDisposed || state.selectedGroup == groupId) return;
    state = state.copyWith(
      selectedGroup: groupId,
      selectedExternalIds: const {},
      clearError: true,
    );
    await loadCatalog(reset: true);
  }

  Future<void> loadCatalog({bool reset = true}) async {
    if (_isDisposed) return;
    final pluginName = state.selectedPluginName;
    if (pluginName == null || pluginName.isEmpty) return;

    if (reset) {
      state = state.copyWith(
        isLoading: true,
        clearError: true,
        isAuthExpired: false,
        clearNextCursor: true,
      );
    } else {
      if (!state.hasMore || state.isLoadingMore) return;
      state = state.copyWith(isLoadingMore: true, clearError: true);
    }

    try {
      final api = ref.read(apiServiceProvider);
      final page = await api.getPluginCatalog(
        pluginName,
        mode: state.mode,
        query: state.query.isNotEmpty ? state.query : null,
        group: state.selectedGroup.isNotEmpty ? state.selectedGroup : null,
        cursor: reset ? null : state.nextCursor,
      );
      if (_isDisposed) return;

      final mergedItems = reset ? page.items : [...state.items, ...page.items];
      final nextGroups = page.groups.isNotEmpty ? page.groups : state.groups;

      state = state.copyWith(
        items: mergedItems,
        groups: reset ? page.groups : nextGroups,
        nextCursor: page.nextCursor,
        clearNextCursor: page.nextCursor == null || page.nextCursor!.isEmpty,
        totalCount: page.totalCount,
        clearTotalCount: page.totalCount == null && reset,
        isLoading: false,
        isLoadingMore: false,
        isAuthExpired: false,
      );
    } catch (e) {
      if (_isDisposed) return;
      final isAuthErr = e is ApiException && e.statusCode == 401;
      final msg = e is ApiException ? e.message : e.toString();
      state = state.copyWith(
        isLoading: false,
        isLoadingMore: false,
        isAuthExpired: isAuthErr,
        errorMessage: msg,
      );
    }
  }

  Future<void> loadMore() => loadCatalog(reset: false);

  void toggleItemSelection(String externalId) {
    final next = Set<String>.from(state.selectedExternalIds);
    if (next.contains(externalId)) {
      next.remove(externalId);
    } else {
      next.add(externalId);
    }
    state = state.copyWith(selectedExternalIds: next);
  }

  void selectAllAvailable() {
    final ids = state.items
        .where((i) => i.isAvailable)
        .map((i) => i.externalId)
        .toSet();
    state = state.copyWith(selectedExternalIds: ids);
  }

  void clearSelection() {
    state = state.copyWith(selectedExternalIds: const {});
  }

  void setPreferredFormat(String externalId, String format) {
    final next = Map<String, String>.from(state.preferredFormats);
    next[externalId] = format;
    state = state.copyWith(preferredFormats: next);
  }

  void handlePluginEvent(Map<String, dynamic> data) {
    if (_isDisposed) return;
    final plugin = data['plugin']?.toString();
    if (plugin == null || plugin != state.selectedPluginName) return;
    final kind = data['kind']?.toString();
    final inner = data['data'];
    if (kind == 'progress' && inner is Map) {
      final msg = inner['message']?.toString() ?? inner['title']?.toString();
      if (msg != null && msg.isNotEmpty) {
        state = state.copyWith(stagingMessage: msg);
      }
    }
  }

  Future<CatalogStageResult?> stageSelected() async {
    if (_isDisposed || state.selectedExternalIds.isEmpty) return null;
    final pluginName = state.selectedPluginName;
    if (pluginName == null || pluginName.isEmpty) return null;

    final reqItems = state.selectedExternalIds
        .map(
          (id) => CatalogStageItemRequest(
            externalId: id,
            preferredFormat: state.preferredFormats[id],
          ),
        )
        .toList();

    state = state.copyWith(
      isStaging: true,
      stagingMessage:
          'Staging ${reqItems.length} ${reqItems.length == 1 ? 'book' : 'books'} for review...',
      clearError: true,
      isAuthExpired: false,
    );

    try {
      final api = ref.read(apiServiceProvider);
      final result = await api.stagePluginCatalogItems(pluginName, reqItems);
      if (_isDisposed) return result;

      final stagedByExtId = <String, CatalogStagedEntry>{
        for (final r in result.results)
          if (!r.isFailed) r.externalId: r,
      };
      final failedMessages = result.results
          .where((r) => r.isFailed)
          .map((r) => '${r.title ?? r.externalId}: ${r.error ?? 'Failed'}')
          .toList();

      final updatedItems = state.items.map((item) {
        final stagedEntry = stagedByExtId[item.externalId];
        if (stagedEntry != null) {
          return item.copyWith(
            status: 'staged',
            uploadJobId: stagedEntry.uploadJobId,
          );
        }
        return item;
      }).toList();

      final remainingSelected = Set<String>.from(state.selectedExternalIds)
        ..removeAll(stagedByExtId.keys);

      state = state.copyWith(
        items: updatedItems,
        selectedExternalIds: remainingSelected,
        isStaging: false,
        clearStagingMessage: true,
        errorMessage:
            failedMessages.isNotEmpty ? failedMessages.join('; ') : null,
      );

      await ref.read(uploadProvider.notifier).loadStagedJobs();
      if (!_isDisposed) {
        ref.read(queueProvider.notifier).refresh();
      }
      return result;
    } catch (e) {
      if (_isDisposed) return null;
      final isAuthErr = e is ApiException && e.statusCode == 401;
      final msg = e is ApiException ? e.message : e.toString();
      state = state.copyWith(
        isStaging: false,
        clearStagingMessage: true,
        isAuthExpired: isAuthErr,
        errorMessage: msg,
      );
      return null;
    }
  }
}

final catalogBrowserProvider =
    NotifierProvider<CatalogBrowserNotifier, CatalogBrowserState>(
  CatalogBrowserNotifier.new,
);
