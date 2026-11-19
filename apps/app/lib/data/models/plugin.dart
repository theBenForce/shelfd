const String kPluginSecretRedactedSentinel = '__SHELFD_REDACTED__';

class PluginPermissions {
  final List<String> networkDomains;
  final bool libraryRead;
  final bool libraryWrite;

  const PluginPermissions({
    this.networkDomains = const [],
    this.libraryRead = false,
    this.libraryWrite = false,
  });

  factory PluginPermissions.fromJson(Map<String, dynamic>? json) {
    if (json == null) return const PluginPermissions();
    final domains = (json['network_domains'] as List<dynamic>? ?? const [])
        .map((e) => e.toString())
        .where((e) => e.isNotEmpty)
        .toList();
    return PluginPermissions(
      networkDomains: domains,
      libraryRead: json['library_read'] as bool? ?? false,
      libraryWrite: json['library_write'] as bool? ?? false,
    );
  }
}

class PluginCommandDef {
  final String name;
  final String? title;
  final String? description;
  final Map<String, dynamic>? inputSchema;

  const PluginCommandDef({
    required this.name,
    this.title,
    this.description,
    this.inputSchema,
  });

  String get displayTitle =>
      (title != null && title!.trim().isNotEmpty) ? title!.trim() : name;

  factory PluginCommandDef.fromJson(Map<String, dynamic> json) {
    return PluginCommandDef(
      name: json['name'] as String? ?? '',
      title: json['title'] as String?,
      description: json['description'] as String?,
      inputSchema: json['input_schema'] is Map<String, dynamic>
          ? Map<String, dynamic>.from(json['input_schema'] as Map)
          : null,
    );
  }
}

class PluginCatalogDescriptor {
  final List<String> modes;
  final List<String> supportedFormats;
  final String? searchHint;
  final bool requiresAuth;

  const PluginCatalogDescriptor({
    this.modes = const [],
    this.supportedFormats = const [],
    this.searchHint,
    this.requiresAuth = false,
  });

  factory PluginCatalogDescriptor.fromJson(Map<String, dynamic> json) {
    return PluginCatalogDescriptor(
      modes: (json['modes'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      supportedFormats:
          (json['supported_formats'] as List<dynamic>? ?? const [])
              .map((e) => e.toString())
              .where((e) => e.isNotEmpty)
              .toList(),
      searchHint: json['search_hint'] as String?,
      requiresAuth: json['requires_auth'] as bool? ?? false,
    );
  }
}

class PluginSummary {
  final String name;
  final String displayName;
  final String version;
  final String? description;
  final String? homepage;
  final bool enabled;
  final bool daemon;
  final String status;
  final String? lastError;
  final List<String> capabilities;
  final List<String> catalogModes;
  final List<String> supportedFormats;
  final List<String> apiScopes;
  final PluginPermissions permissions;
  final Map<String, dynamic>? configSchema;
  final List<PluginCommandDef> commands;
  final PluginCatalogDescriptor? catalog;
  final bool configured;

  const PluginSummary({
    required this.name,
    required this.displayName,
    required this.version,
    this.description,
    this.homepage,
    this.enabled = false,
    this.daemon = false,
    this.status = 'disabled',
    this.lastError,
    this.capabilities = const [],
    this.catalogModes = const [],
    this.supportedFormats = const [],
    this.apiScopes = const [],
    this.permissions = const PluginPermissions(),
    this.configSchema,
    this.commands = const [],
    this.catalog,
    this.configured = true,
  });

  bool get hasCatalog =>
      capabilities.any((c) => c.toLowerCase() == 'catalog');

  bool get hasConfigSchema {
    final props = configSchema?['properties'];
    return props is Map && props.isNotEmpty;
  }

  List<String> get resolvedCatalogModes {
    if (catalog != null && catalog!.modes.isNotEmpty) {
      return catalog!.modes;
    }
    if (catalogModes.isNotEmpty) {
      return catalogModes;
    }
    return const ['browse'];
  }

  List<String> get resolvedSupportedFormats {
    if (catalog != null && catalog!.supportedFormats.isNotEmpty) {
      return catalog!.supportedFormats;
    }
    return supportedFormats;
  }

  factory PluginSummary.fromJson(Map<String, dynamic> json) {
    final name = json['name'] as String? ?? '';
    final rawDisplay = json['display_name'] as String?;
    return PluginSummary(
      name: name,
      displayName: (rawDisplay != null && rawDisplay.trim().isNotEmpty)
          ? rawDisplay.trim()
          : name,
      version: json['version'] as String? ?? '0.0.0',
      description: json['description'] as String?,
      homepage: json['homepage'] as String?,
      enabled: json['enabled'] as bool? ?? false,
      daemon: json['daemon'] as bool? ?? false,
      status: json['status'] as String? ?? 'disabled',
      lastError: json['last_error'] as String?,
      capabilities: (json['capabilities'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .toList(),
      catalogModes: (json['catalog_modes'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .toList(),
      supportedFormats:
          (json['supported_formats'] as List<dynamic>? ?? const [])
              .map((e) => e.toString())
              .toList(),
      apiScopes: (json['api_scopes'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .toList(),
      permissions: PluginPermissions.fromJson(
        json['permissions'] is Map<String, dynamic>
            ? json['permissions'] as Map<String, dynamic>
            : null,
      ),
      configSchema: json['config_schema'] is Map
          ? Map<String, dynamic>.from(json['config_schema'] as Map)
          : null,
      commands: (json['commands'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(PluginCommandDef.fromJson)
          .toList(),
      catalog: json['catalog'] is Map<String, dynamic>
          ? PluginCatalogDescriptor.fromJson(
              json['catalog'] as Map<String, dynamic>,
            )
          : null,
      configured: json['configured'] as bool? ?? true,
    );
  }
}

class PluginDetail {
  final PluginSummary summary;
  final List<String> logs;

  const PluginDetail({
    required this.summary,
    this.logs = const [],
  });

  factory PluginDetail.fromJson(Map<String, dynamic> json) {
    return PluginDetail(
      summary: PluginSummary.fromJson(json),
      logs: (json['logs'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .toList(),
    );
  }
}

class PluginConfigResponse {
  final Map<String, dynamic> config;
  final List<String> secretFieldsSet;

  const PluginConfigResponse({
    this.config = const {},
    this.secretFieldsSet = const [],
  });

  factory PluginConfigResponse.fromJson(Map<String, dynamic> json) {
    final rawConfig = json['config'];
    return PluginConfigResponse(
      config: rawConfig is Map
          ? Map<String, dynamic>.from(rawConfig)
          : const {},
      secretFieldsSet:
          (json['secret_fields_set'] as List<dynamic>? ?? const [])
              .map((e) => e.toString())
              .toList(),
    );
  }
}

class CatalogFormat {
  final String format;
  final String? label;
  final int? sizeBytes;

  const CatalogFormat({
    required this.format,
    this.label,
    this.sizeBytes,
  });

  String get displayLabel =>
      (label != null && label!.trim().isNotEmpty)
          ? label!.trim()
          : format.toUpperCase();

  factory CatalogFormat.fromJson(Map<String, dynamic> json) {
    return CatalogFormat(
      format: json['format'] as String? ?? '',
      label: json['label'] as String?,
      sizeBytes: (json['size_bytes'] as num?)?.toInt(),
    );
  }
}

class CatalogGroup {
  final String id;
  final String label;
  final int? count;

  const CatalogGroup({
    required this.id,
    required this.label,
    this.count,
  });

  String get displayLabel => count != null ? '$label ($count)' : label;

  factory CatalogGroup.fromJson(Map<String, dynamic> json) {
    return CatalogGroup(
      id: json['id'] as String? ?? '',
      label: json['label'] as String? ?? '',
      count: (json['count'] as num?)?.toInt(),
    );
  }
}

class CatalogItem {
  final String externalId;
  final String title;
  final List<String> authors;
  final String? summary;
  final String? coverUrl;
  final String? group;
  final String? publisher;
  final String? language;
  final List<String> genres;
  final List<CatalogFormat> formats;
  final String status;
  final String? uploadJobId;
  final String? bookId;

  const CatalogItem({
    required this.externalId,
    required this.title,
    this.authors = const [],
    this.summary,
    this.coverUrl,
    this.group,
    this.publisher,
    this.language,
    this.genres = const [],
    this.formats = const [],
    this.status = 'available',
    this.uploadJobId,
    this.bookId,
  });

  bool get isInLibrary => status == 'in_library';
  bool get isStaged => status == 'staged';
  bool get isAvailable => status == 'available';

  CatalogItem copyWith({
    String? status,
    String? uploadJobId,
    String? bookId,
  }) {
    return CatalogItem(
      externalId: externalId,
      title: title,
      authors: authors,
      summary: summary,
      coverUrl: coverUrl,
      group: group,
      publisher: publisher,
      language: language,
      genres: genres,
      formats: formats,
      status: status ?? this.status,
      uploadJobId: uploadJobId ?? this.uploadJobId,
      bookId: bookId ?? this.bookId,
    );
  }

  factory CatalogItem.fromJson(Map<String, dynamic> json) {
    final rawStatus = json['status'] as String?;
    return CatalogItem(
      externalId: json['external_id'] as String? ?? '',
      title: json['title'] as String? ?? 'Untitled',
      authors: (json['authors'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      summary: json['summary'] as String?,
      coverUrl: json['cover_url'] as String?,
      group: json['group'] as String?,
      publisher: json['publisher'] as String?,
      language: json['language'] as String?,
      genres: (json['genres'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      formats: (json['formats'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(CatalogFormat.fromJson)
          .toList(),
      status: (rawStatus != null && rawStatus.isNotEmpty)
          ? rawStatus
          : 'available',
      uploadJobId: json['upload_job_id'] as String?,
      bookId: json['book_id'] as String?,
    );
  }
}

class CatalogPage {
  final List<CatalogItem> items;
  final List<CatalogGroup> groups;
  final String? nextCursor;
  final int? totalCount;

  const CatalogPage({
    this.items = const [],
    this.groups = const [],
    this.nextCursor,
    this.totalCount,
  });

  bool get hasMore => nextCursor != null && nextCursor!.isNotEmpty;

  factory CatalogPage.fromJson(Map<String, dynamic> json) {
    final rawNext = json['next_cursor'] as String?;
    return CatalogPage(
      items: (json['items'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(CatalogItem.fromJson)
          .toList(),
      groups: (json['groups'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(CatalogGroup.fromJson)
          .toList(),
      nextCursor: (rawNext != null && rawNext.isNotEmpty) ? rawNext : null,
      totalCount: (json['total_count'] as num?)?.toInt(),
    );
  }
}

class CatalogStageItemRequest {
  final String externalId;
  final String? preferredFormat;

  const CatalogStageItemRequest({
    required this.externalId,
    this.preferredFormat,
  });

  Map<String, dynamic> toJson() {
    return {
      'external_id': externalId,
      if (preferredFormat != null && preferredFormat!.isNotEmpty)
        'preferred_format': preferredFormat,
    };
  }
}

class CatalogStagedEntry {
  final String externalId;
  final String? title;
  final String? uploadJobId;
  final String status;
  final String? error;

  const CatalogStagedEntry({
    required this.externalId,
    this.title,
    this.uploadJobId,
    required this.status,
    this.error,
  });

  bool get isFailed => status == 'failed';

  factory CatalogStagedEntry.fromJson(Map<String, dynamic> json) {
    return CatalogStagedEntry(
      externalId: json['external_id'] as String? ?? '',
      title: json['title'] as String?,
      uploadJobId: json['upload_job_id'] as String?,
      status: json['status'] as String? ?? 'staged',
      error: json['error'] as String?,
    );
  }
}

class CatalogStageResult {
  final String? batchId;
  final bool isAsync;
  final List<CatalogStagedEntry> results;
  final String? message;

  const CatalogStageResult({
    this.batchId,
    this.isAsync = false,
    this.results = const [],
    this.message,
  });

  factory CatalogStageResult.fromJson(Map<String, dynamic> json) {
    return CatalogStageResult(
      batchId: json['batch_id'] as String?,
      isAsync: json['async'] as bool? ?? false,
      results: (json['results'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(CatalogStagedEntry.fromJson)
          .toList(),
      message: json['message'] as String?,
    );
  }
}
