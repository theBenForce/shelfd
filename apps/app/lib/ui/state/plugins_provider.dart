import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/plugin.dart';
import '../../data/services/api_service.dart';
import 'providers.dart';

String generatePkceVerifier([int length = 64]) {
  const chars =
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~';
  final rand = Random.secure();
  return List.generate(length, (_) => chars[rand.nextInt(chars.length)]).join();
}

String computePkceS256Challenge(String verifier) {
  final digest = _sha256(utf8.encode(verifier));
  return base64UrlEncode(digest).replaceAll('=', '');
}

Uint8List _sha256(List<int> data) {
  const k = <int>[
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1,
    0x923f82a4, 0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
    0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786,
    0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147,
    0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
    0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
    0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a,
    0x5b9cca4f, 0x682e6ff3, 0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
    0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ];

  int rotr(int x, int n) =>
      ((x & 0xffffffff) >>> n) | ((x << (32 - n)) & 0xffffffff);

  final bitLen = data.length * 8;
  final padded = <int>[...data, 0x80];
  while ((padded.length % 64) != 56) {
    padded.add(0);
  }
  final byteData = ByteData(8);
  final highBits = (bitLen ~/ 0x100000000) & 0xffffffff;
  final lowBits = bitLen & 0xffffffff;
  byteData.setUint32(0, highBits, Endian.big);
  byteData.setUint32(4, lowBits, Endian.big);
  for (int i = 0; i < 8; i++) {
    padded.add(byteData.getUint8(i));
  }

  int h0 = 0x6a09e667;
  int h1 = 0xbb67ae85;
  int h2 = 0x3c6ef372;
  int h3 = 0xa54ff53a;
  int h4 = 0x510e527f;
  int h5 = 0x9b05688c;
  int h6 = 0x1f83d9ab;
  int h7 = 0x5be0cd19;

  final w = List<int>.filled(64, 0);
  final chunkBytes = Uint8List.fromList(padded);
  final view = ByteData.sublistView(chunkBytes);

  for (int offset = 0; offset < chunkBytes.length; offset += 64) {
    for (int i = 0; i < 16; i++) {
      w[i] = view.getUint32(offset + i * 4, Endian.big);
    }
    for (int i = 16; i < 64; i++) {
      final s0 = rotr(w[i - 15], 7) ^
          rotr(w[i - 15], 18) ^
          ((w[i - 15] & 0xffffffff) >>> 3);
      final s1 = rotr(w[i - 2], 17) ^
          rotr(w[i - 2], 19) ^
          ((w[i - 2] & 0xffffffff) >>> 10);
      w[i] = (w[i - 16] + s0 + w[i - 7] + s1) & 0xffffffff;
    }

    int a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;

    for (int i = 0; i < 64; i++) {
      final s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      final ch = (e & f) ^ ((~e) & g);
      final temp1 = (h + s1 + ch + k[i] + w[i]) & 0xffffffff;
      final s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      final maj = (a & b) ^ (a & c) ^ (b & c);
      final temp2 = (s0 + maj) & 0xffffffff;

      h = g;
      g = f;
      f = e;
      e = (d + temp1) & 0xffffffff;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) & 0xffffffff;
    }

    h0 = (h0 + a) & 0xffffffff;
    h1 = (h1 + b) & 0xffffffff;
    h2 = (h2 + c) & 0xffffffff;
    h3 = (h3 + d) & 0xffffffff;
    h4 = (h4 + e) & 0xffffffff;
    h5 = (h5 + f) & 0xffffffff;
    h6 = (h6 + g) & 0xffffffff;
    h7 = (h7 + h) & 0xffffffff;
  }

  final out = ByteData(32);
  out.setUint32(0, h0, Endian.big);
  out.setUint32(4, h1, Endian.big);
  out.setUint32(8, h2, Endian.big);
  out.setUint32(12, h3, Endian.big);
  out.setUint32(16, h4, Endian.big);
  out.setUint32(20, h5, Endian.big);
  out.setUint32(24, h6, Endian.big);
  out.setUint32(28, h7, Endian.big);
  return out.buffer.asUint8List();
}

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

  List<PluginSummary> get enabledActionPlugins =>
      plugins.where((p) => p.enabled && p.hasActions).toList();

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

class PluginJobsState {
  final List<PluginJob> jobs;
  final List<PluginJob> activeErrors;
  final bool isLoading;

  const PluginJobsState({
    this.jobs = const [],
    this.activeErrors = const [],
    this.isLoading = false,
  });

  List<PluginJob> get activeJobs =>
      jobs.where((j) => j.isActive).toList(growable: false);

  bool get hasActiveJobs => activeJobs.isNotEmpty;

  PluginJob? get currentJob => activeJobs.isNotEmpty ? activeJobs.first : null;

  PluginJob? get latestError =>
      activeErrors.isNotEmpty ? activeErrors.first : null;

  double get currentItemProgress {
    final active = currentJob;
    if (active == null) return 0.0;
    return active.progressFraction;
  }

  int get completedItemsCount {
    if (jobs.isEmpty) return 0;
    final active = currentJob;
    if (active != null && activeJobs.length == 1 && active.total > 1) {
      return (active.current > 0 ? active.current - 1 : 0).clamp(0, active.total);
    }
    return jobs.where((j) => j.isCompleted || j.isFailed).length;
  }

  int get totalItemsCount {
    if (jobs.isEmpty) return 0;
    final active = currentJob;
    if (active != null && activeJobs.length == 1 && active.total > 1) {
      return active.total;
    }
    return jobs.length;
  }

  double get totalJobsProgress {
    if (jobs.isEmpty) return 0.0;
    final active = currentJob;
    if (active != null && activeJobs.length == 1 && active.total > 1) {
      return active.progressFraction;
    }
    final completed = jobs.where((j) => j.isCompleted || j.isFailed).length;
    final activeProgress = active?.progressFraction ?? 0.0;
    return ((completed + activeProgress) / jobs.length).clamp(0.0, 1.0);
  }

  String get currentJobDisplayTitle {
    final active = currentJob;
    if (active == null) return 'Processing plugin job...';
    if (active.itemTitle != null &&
        active.itemTitle!.trim().isNotEmpty &&
        active.itemTitle!.trim() != active.title) {
      return '${active.title} • ${active.itemTitle!.trim()}';
    }
    return active.title.isNotEmpty ? active.title : active.action;
  }

  String get currentStageDescription {
    final active = currentJob;
    if (active == null) return 'Queued';
    if (active.message != null && active.message!.trim().isNotEmpty) {
      return active.message!.trim();
    }
    return active.status == 'running' ? 'Processing...' : 'Queued';
  }

  PluginJobsState copyWith({
    List<PluginJob>? jobs,
    List<PluginJob>? activeErrors,
    bool? isLoading,
  }) {
    return PluginJobsState(
      jobs: jobs ?? this.jobs,
      activeErrors: activeErrors ?? this.activeErrors,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

class PluginJobsNotifier extends Notifier<PluginJobsState> {
  bool _isDisposed = false;
  final Set<String> _acknowledgedErrorJobIds = {};

  @override
  PluginJobsState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    ref.listen(authProvider, (previous, next) {
      if (previous?.user != next.user && next.user != null) {
        loadJobs();
      }
    });
    Future.microtask(loadJobs);
    return const PluginJobsState();
  }

  Future<void> loadJobs() async {
    if (_isDisposed) return;
    state = state.copyWith(isLoading: true);
    try {
      final api = ref.read(apiServiceProvider);
      final list = await api.getPluginJobs();
      if (_isDisposed) return;
      final errors = list
          .where((j) => j.isFailed && !_acknowledgedErrorJobIds.contains(j.id))
          .toList();
      final activeOnly = list.where((j) => j.isActive).toList();
      state = state.copyWith(
        jobs: activeOnly,
        activeErrors: errors,
        isLoading: false,
      );
    } catch (_) {
      if (!_isDisposed) {
        state = state.copyWith(isLoading: false);
      }
    }
  }

  void enqueueOrTrackJob(PluginJob job) {
    if (_isDisposed) return;
    final existingIdx = state.jobs.indexWhere((j) => j.id == job.id);
    List<PluginJob> nextJobs;
    if (existingIdx >= 0) {
      nextJobs = List<PluginJob>.from(state.jobs);
      nextJobs[existingIdx] = job;
    } else {
      nextJobs = [...state.jobs.where((j) => j.isActive), job];
    }

    List<PluginJob> nextErrors = state.activeErrors;
    if (job.isFailed && !_acknowledgedErrorJobIds.contains(job.id)) {
      if (!nextErrors.any((e) => e.id == job.id)) {
        nextErrors = [...nextErrors, job];
      }
    }

    // Remove finished jobs if no more active jobs remain
    final stillHasActive = nextJobs.any((j) => j.isActive);
    state = state.copyWith(
      jobs: stillHasActive ? nextJobs : const [],
      activeErrors: nextErrors,
    );
  }

  void handlePluginEvent(Map<String, dynamic> data) {
    if (_isDisposed) return;
    final kind = data['kind']?.toString();
    if (kind != 'job_progress') return;
    final rawJob = data['job'];
    if (rawJob is! Map) return;
    final job = PluginJob.fromJson(Map<String, dynamic>.from(rawJob));
    enqueueOrTrackJob(job);
  }

  void acknowledgeError(String jobId) {
    if (_isDisposed) return;
    _acknowledgedErrorJobIds.add(jobId);
    final nextErrors =
        state.activeErrors.where((j) => j.id != jobId).toList(growable: false);
    state = state.copyWith(activeErrors: nextErrors);
  }

  Future<void> dismissJob(String jobId) async {
    if (_isDisposed) return;
    _acknowledgedErrorJobIds.add(jobId);
    state = state.copyWith(
      jobs: state.jobs.where((j) => j.id != jobId).toList(),
      activeErrors: state.activeErrors.where((j) => j.id != jobId).toList(),
    );
    try {
      await ref.read(apiServiceProvider).dismissPluginJob(jobId);
    } catch (_) {}
  }
}

final pluginJobsProvider =
    NotifierProvider<PluginJobsNotifier, PluginJobsState>(
  PluginJobsNotifier.new,
);
