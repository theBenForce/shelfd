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

class PluginAuthFieldSpec {
  final String key;
  final String title;
  final String? description;
  final bool secret;
  final bool required;
  final String? placeholder;

  const PluginAuthFieldSpec({
    required this.key,
    required this.title,
    this.description,
    this.secret = false,
    this.required = false,
    this.placeholder,
  });

  factory PluginAuthFieldSpec.fromJson(Map<String, dynamic> json) {
    final key = json['key'] as String? ?? '';
    final rawTitle = json['title'] as String?;
    return PluginAuthFieldSpec(
      key: key,
      title: (rawTitle != null && rawTitle.trim().isNotEmpty)
          ? rawTitle.trim()
          : key,
      description: json['description'] as String?,
      secret: json['secret'] as bool? ?? false,
      required: json['required'] as bool? ?? false,
      placeholder: json['placeholder'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'key': key,
      'title': title,
      if (description != null) 'description': description,
      if (secret) 'secret': secret,
      if (required) 'required': required,
      if (placeholder != null) 'placeholder': placeholder,
    };
  }
}

class PluginAuthSpec {
  final List<String> methods;
  final String? title;
  final String? description;
  final String callbackParam;
  final List<PluginAuthFieldSpec> fields;

  const PluginAuthSpec({
    this.methods = const [],
    this.title,
    this.description,
    this.callbackParam = 'code',
    this.fields = const [],
  });

  bool get supportsOAuthPkce => methods.contains('oauth_pkce');
  bool get supportsToken => methods.contains('token');

  factory PluginAuthSpec.fromJson(Map<String, dynamic> json) {
    final rawCallback = json['callback_param'] as String?;
    return PluginAuthSpec(
      methods: (json['methods'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      title: json['title'] as String?,
      description: json['description'] as String?,
      callbackParam: (rawCallback != null && rawCallback.trim().isNotEmpty)
          ? rawCallback.trim()
          : 'code',
      fields: (json['fields'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => PluginAuthFieldSpec.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'methods': methods,
      if (title != null) 'title': title,
      if (description != null) 'description': description,
      'callback_param': callbackParam,
      if (fields.isNotEmpty) 'fields': fields.map((f) => f.toJson()).toList(),
    };
  }
}

class PluginAuthStatus {
  final bool linked;
  final String? accountLabel;
  final String? clientId;
  final String? expiresAt;
  final String? authError;
  final List<String> methods;
  final Map<String, dynamic>? extra;

  const PluginAuthStatus({
    this.linked = false,
    this.accountLabel,
    this.clientId,
    this.expiresAt,
    this.authError,
    this.methods = const [],
    this.extra,
  });

  factory PluginAuthStatus.fromJson(Map<String, dynamic> json) {
    final isLinked = (json['linked'] as bool?) ??
        (json['connected'] as bool?) ??
        false;
    return PluginAuthStatus(
      linked: isLinked,
      accountLabel: json['account_label'] as String?,
      clientId: json['client_id'] as String?,
      expiresAt: json['expires_at'] as String?,
      authError: json['auth_error'] as String?,
      methods: (json['methods'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      extra: json['extra'] is Map
          ? Map<String, dynamic>.from(json['extra'] as Map)
          : null,
    );
  }
}

class PluginAuthBeginResult {
  final String authorizeUrl;
  final String? clientId;
  final String? state;

  const PluginAuthBeginResult({
    required this.authorizeUrl,
    this.clientId,
    this.state,
  });

  factory PluginAuthBeginResult.fromJson(Map<String, dynamic> json) {
    return PluginAuthBeginResult(
      authorizeUrl: json['authorize_url'] as String? ?? '',
      clientId: json['client_id'] as String?,
      state: json['state'] as String?,
    );
  }
}

class PluginActionDef {
  final String name;
  final String title;
  final String? description;
  final String? icon;
  final List<String> targets;
  final List<String> mediaTypes;
  final String mode;
  final String execution;

  const PluginActionDef({
    required this.name,
    required this.title,
    this.description,
    this.icon,
    this.targets = const ['book'],
    this.mediaTypes = const [],
    this.mode = 'immediate',
    this.execution = 'sync',
  });

  bool get isForm => mode == 'form';
  bool get isImmediate => mode != 'form';
  bool get isJob => execution == 'job';

  bool matches({required String targetType, required String mediaType}) {
    final normalizedTarget = targetType.toLowerCase().trim();
    if (targets.isNotEmpty &&
        !targets.any((t) => t.toLowerCase().trim() == normalizedTarget)) {
      return false;
    }
    if (mediaTypes.isEmpty) {
      return true;
    }
    final normalizedMedia = mediaType.toLowerCase().trim();
    return mediaTypes.any((m) => m.toLowerCase().trim() == normalizedMedia);
  }

  bool matchesTarget(
    String targetType, {
    required bool hasEpub,
    required bool hasAudio,
  }) {
    final normalizedTarget = targetType.toLowerCase().trim();
    if (targets.isNotEmpty &&
        !targets.any((t) => t.toLowerCase().trim() == normalizedTarget)) {
      return false;
    }
    if (mediaTypes.isEmpty) {
      return true;
    }
    for (final mt in mediaTypes) {
      final lower = mt.toLowerCase().trim();
      if (lower == 'ebook' && hasEpub) return true;
      if (lower == 'audiobook' && hasAudio) return true;
    }
    return false;
  }

  factory PluginActionDef.fromJson(Map<String, dynamic> json) {
    final name = json['name'] as String? ?? '';
    final rawTitle = json['title'] as String?;
    final rawMode = json['mode'] as String?;
    final rawExec = json['execution'] as String?;
    return PluginActionDef(
      name: name,
      title: (rawTitle != null && rawTitle.trim().isNotEmpty)
          ? rawTitle.trim()
          : name,
      description: json['description'] as String?,
      icon: json['icon'] as String?,
      targets: (json['targets'] as List<dynamic>? ?? const ['book'])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      mediaTypes: (json['media_types'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      mode: (rawMode != null && rawMode.trim().isNotEmpty)
          ? rawMode.trim()
          : 'immediate',
      execution: (rawExec != null && rawExec.trim().isNotEmpty)
          ? rawExec.trim()
          : 'sync',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'title': title,
      if (description != null) 'description': description,
      if (icon != null) 'icon': icon,
      'targets': targets,
      if (mediaTypes.isNotEmpty) 'media_types': mediaTypes,
      'mode': mode,
      'execution': execution,
    };
  }
}

class PluginActionTarget {
  final String type;
  final String id;
  final String? title;
  final List<String>? bookIds;

  const PluginActionTarget({
    required this.type,
    required this.id,
    this.title,
    this.bookIds,
  });

  factory PluginActionTarget.fromJson(Map<String, dynamic> json) {
    return PluginActionTarget(
      type: json['type'] as String? ?? 'book',
      id: json['id'] as String? ?? '',
      title: json['title'] as String?,
      bookIds: (json['book_ids'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'id': id,
      if (title != null && title!.isNotEmpty) 'title': title,
      if (bookIds != null && bookIds!.isNotEmpty) 'book_ids': bookIds,
    };
  }
}

class PluginFormEntry {
  final String id;
  final int index;
  final String title;
  final String? subtitle;
  final String? iconUri;
  final String? imageUrl;

  const PluginFormEntry({
    required this.id,
    required this.index,
    required this.title,
    this.subtitle,
    this.iconUri,
    this.imageUrl,
  });

  factory PluginFormEntry.fromJson(Map<String, dynamic> json) {
    final idx = (json['index'] as num?)?.toInt() ?? 0;
    return PluginFormEntry(
      id: json['id']?.toString() ?? '$idx',
      index: idx,
      title: json['title'] as String? ?? 'Item ${idx + 1}',
      subtitle: json['subtitle'] as String?,
      iconUri: json['icon_uri'] as String?,
      imageUrl: json['image_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'index': index,
      'title': title,
      if (subtitle != null) 'subtitle': subtitle,
      if (iconUri != null) 'icon_uri': iconUri,
      if (imageUrl != null) 'image_url': imageUrl,
    };
  }
}

class PluginFieldVisibilityRule {
  final String field;
  final String? equals;
  final List<String> inValues;

  const PluginFieldVisibilityRule({
    required this.field,
    this.equals,
    this.inValues = const [],
  });

  bool evaluate(Map<String, dynamic> formValues) {
    final raw = formValues[field];
    final current = raw?.toString() ?? '';
    if (equals != null && equals!.isNotEmpty) {
      return current == equals;
    }
    if (inValues.isNotEmpty) {
      return inValues.contains(current);
    }
    return true;
  }

  factory PluginFieldVisibilityRule.fromJson(Map<String, dynamic> json) {
    return PluginFieldVisibilityRule(
      field: json['field'] as String? ?? '',
      equals: json['equals']?.toString(),
      inValues: (json['in'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'field': field,
      if (equals != null) 'equals': equals,
      if (inValues.isNotEmpty) 'in': inValues,
    };
  }
}

class PluginOptionItem {
  final String value;
  final String label;
  final String? subtitle;
  final String? description;
  final String? imageUrl;
  final String? iconUri;
  final String? badge;
  final int? count;
  final Map<String, dynamic>? meta;

  const PluginOptionItem({
    required this.value,
    required this.label,
    this.subtitle,
    this.description,
    this.imageUrl,
    this.iconUri,
    this.badge,
    this.count,
    this.meta,
  });

  factory PluginOptionItem.fromJson(Map<String, dynamic> json) {
    final val = json['value']?.toString() ?? '';
    final rawLabel = json['label'] as String?;
    return PluginOptionItem(
      value: val,
      label: (rawLabel != null && rawLabel.trim().isNotEmpty)
          ? rawLabel.trim()
          : val,
      subtitle: json['subtitle'] as String?,
      description: json['description'] as String?,
      imageUrl: json['image_url'] as String?,
      iconUri: json['icon_uri'] as String?,
      badge: json['badge'] as String?,
      count: (json['count'] as num?)?.toInt(),
      meta: json['meta'] is Map
          ? Map<String, dynamic>.from(json['meta'] as Map)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'value': value,
      'label': label,
      if (subtitle != null) 'subtitle': subtitle,
      if (description != null) 'description': description,
      if (imageUrl != null) 'image_url': imageUrl,
      if (iconUri != null) 'icon_uri': iconUri,
      if (badge != null) 'badge': badge,
      if (count != null) 'count': count,
      if (meta != null) 'meta': meta,
    };
  }
}

class PluginAssetPickerTab {
  final String id;
  final String label;
  final bool searchable;

  const PluginAssetPickerTab({
    required this.id,
    required this.label,
    this.searchable = false,
  });

  factory PluginAssetPickerTab.fromJson(Map<String, dynamic> json) {
    final id = json['id'] as String? ?? '';
    final rawLabel = json['label'] as String?;
    return PluginAssetPickerTab(
      id: id,
      label: (rawLabel != null && rawLabel.trim().isNotEmpty)
          ? rawLabel.trim()
          : id,
      searchable: json['searchable'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'label': label,
      if (searchable) 'searchable': searchable,
    };
  }
}

class PluginInputFieldSpec {
  final String key;
  final String kind;
  final String title;
  final String? description;
  final String? placeholder;
  final bool required;
  final dynamic defaultValue;
  final PluginFieldVisibilityRule? visibleWhen;
  final List<PluginOptionItem> options;
  final String? optionsSource;
  final bool searchable;
  final String? emptyMessage;
  final String? assetShape;
  final List<PluginAssetPickerTab> tabs;
  final bool allowUpload;
  final bool perEntryOverride;
  final String? overrideKey;
  final String? overrideLabel;

  const PluginInputFieldSpec({
    required this.key,
    required this.kind,
    required this.title,
    this.description,
    this.placeholder,
    this.required = false,
    this.defaultValue,
    this.visibleWhen,
    this.options = const [],
    this.optionsSource,
    this.searchable = false,
    this.emptyMessage,
    this.assetShape,
    this.tabs = const [],
    this.allowUpload = false,
    this.perEntryOverride = false,
    this.overrideKey,
    this.overrideLabel,
  });

  bool isVisible(Map<String, dynamic> formValues) {
    if (visibleWhen == null) return true;
    return visibleWhen!.evaluate(formValues);
  }

  factory PluginInputFieldSpec.fromJson(Map<String, dynamic> json) {
    final key = json['key'] as String? ?? '';
    final kind = json['kind'] as String? ?? 'text';
    final rawTitle = json['title'] as String?;
    final title = (rawTitle != null && rawTitle.trim().isNotEmpty)
        ? rawTitle.trim()
        : key;
    final description = json['description'] as String?;
    final placeholder = json['placeholder'] as String?;
    final required = json['required'] as bool? ?? false;
    final defaultValue = json['default_value'];
    final visibleWhen = json['visible_when'] is Map
        ? PluginFieldVisibilityRule.fromJson(
            Map<String, dynamic>.from(json['visible_when'] as Map),
          )
        : null;
    final options = (json['options'] as List<dynamic>? ?? const [])
        .whereType<Map>()
        .map((e) => PluginOptionItem.fromJson(Map<String, dynamic>.from(e)))
        .toList();
    final optionsSource = json['options_source'] as String?;
    final searchable = json['searchable'] as bool? ?? false;
    final emptyMessage = json['empty_message'] as String?;
    final assetShape = json['asset_shape'] as String?;
    final tabs = (json['tabs'] as List<dynamic>? ?? const [])
        .whereType<Map>()
        .map((e) => PluginAssetPickerTab.fromJson(Map<String, dynamic>.from(e)))
        .toList();
    final allowUpload = json['allow_upload'] as bool? ?? false;
    final perEntryOverride = json['per_entry_override'] as bool? ?? false;
    final overrideKey = json['override_key'] as String?;
    final overrideLabel = json['override_label'] as String?;

    switch (kind) {
      case 'boolean':
        return BooleanFieldSpec(
          key: key,
          title: title,
          description: description,
          required: required,
          defaultValue: defaultValue is bool ? defaultValue : false,
          visibleWhen: visibleWhen,
        );
      case 'select':
        return SelectFieldSpec(
          key: key,
          title: title,
          description: description,
          required: required,
          defaultValue: defaultValue,
          visibleWhen: visibleWhen,
          options: options,
        );
      case 'segmented':
        return SegmentedFieldSpec(
          key: key,
          title: title,
          description: description,
          required: required,
          defaultValue: defaultValue,
          visibleWhen: visibleWhen,
          options: options,
        );
      case 'remote_select':
        return RemoteSelectFieldSpec(
          key: key,
          title: title,
          description: description,
          placeholder: placeholder,
          required: required,
          defaultValue: defaultValue,
          visibleWhen: visibleWhen,
          options: options,
          optionsSource: optionsSource,
          searchable: searchable,
          emptyMessage: emptyMessage,
        );
      case 'asset_picker':
        return AssetPickerFieldSpec(
          key: key,
          title: title,
          description: description,
          required: required,
          defaultValue: defaultValue,
          visibleWhen: visibleWhen,
          options: options,
          optionsSource: optionsSource,
          searchable: searchable,
          emptyMessage: emptyMessage,
          assetShape: assetShape,
          tabs: tabs,
          allowUpload: allowUpload,
          perEntryOverride: perEntryOverride,
          overrideKey: overrideKey,
          overrideLabel: overrideLabel,
        );
      case 'text':
      default:
        return TextFieldSpec(
          key: key,
          kind: kind,
          title: title,
          description: description,
          placeholder: placeholder,
          required: required,
          defaultValue: defaultValue,
          visibleWhen: visibleWhen,
        );
    }
  }
}

class TextFieldSpec extends PluginInputFieldSpec {
  const TextFieldSpec({
    required super.key,
    super.kind = 'text',
    required super.title,
    super.description,
    super.placeholder,
    super.required,
    super.defaultValue,
    super.visibleWhen,
  });
}

class BooleanFieldSpec extends PluginInputFieldSpec {
  const BooleanFieldSpec({
    required super.key,
    required super.title,
    super.description,
    super.required,
    super.defaultValue = false,
    super.visibleWhen,
  }) : super(kind: 'boolean');
}

class SelectFieldSpec extends PluginInputFieldSpec {
  const SelectFieldSpec({
    required super.key,
    required super.title,
    super.description,
    super.required,
    super.defaultValue,
    super.visibleWhen,
    super.options,
  }) : super(kind: 'select');
}

class SegmentedFieldSpec extends PluginInputFieldSpec {
  const SegmentedFieldSpec({
    required super.key,
    required super.title,
    super.description,
    super.required,
    super.defaultValue,
    super.visibleWhen,
    super.options,
  }) : super(kind: 'segmented');
}

class RemoteSelectFieldSpec extends PluginInputFieldSpec {
  const RemoteSelectFieldSpec({
    required super.key,
    required super.title,
    super.description,
    super.placeholder,
    super.required,
    super.defaultValue,
    super.visibleWhen,
    super.options,
    super.optionsSource,
    super.searchable,
    super.emptyMessage,
  }) : super(kind: 'remote_select');
}

class AssetPickerFieldSpec extends PluginInputFieldSpec {
  const AssetPickerFieldSpec({
    required super.key,
    required super.title,
    super.description,
    super.required,
    super.defaultValue,
    super.visibleWhen,
    super.options,
    super.optionsSource,
    super.searchable,
    super.emptyMessage,
    super.assetShape,
    super.tabs,
    super.allowUpload,
    super.perEntryOverride,
    super.overrideKey,
    super.overrideLabel,
  }) : super(kind: 'asset_picker');
}

class PluginActionFormSpec {
  final String title;
  final String? description;
  final String? submitLabel;
  final bool authRequired;
  final String? authMessage;
  final List<PluginFormEntry> entries;
  final List<PluginInputFieldSpec> fields;

  const PluginActionFormSpec({
    required this.title,
    this.description,
    this.submitLabel,
    this.authRequired = false,
    this.authMessage,
    this.entries = const [],
    this.fields = const [],
  });

  factory PluginActionFormSpec.fromJson(Map<String, dynamic> json) {
    return PluginActionFormSpec(
      title: json['title'] as String? ?? 'Plugin Action',
      description: json['description'] as String?,
      submitLabel: json['submit_label'] as String?,
      authRequired: json['auth_required'] as bool? ?? false,
      authMessage: json['auth_message'] as String?,
      entries: (json['entries'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => PluginFormEntry.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      fields: (json['fields'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => PluginInputFieldSpec.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
    );
  }
}

class PluginActionOptionsResult {
  final List<PluginOptionItem> items;
  final String? nextCursor;

  const PluginActionOptionsResult({
    this.items = const [],
    this.nextCursor,
  });

  factory PluginActionOptionsResult.fromJson(Map<String, dynamic> json) {
    final rawCursor = json['next_cursor'] as String?;
    return PluginActionOptionsResult(
      items: (json['items'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => PluginOptionItem.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      nextCursor: (rawCursor != null && rawCursor.isNotEmpty) ? rawCursor : null,
    );
  }
}

class PluginActionResult {
  final String status;
  final String? message;
  final String? externalUrl;
  final String? openUrl;
  final String? downloadApiPath;
  final String? fileName;
  final String? mimeType;
  final String? jobId;
  final PluginJob? job;
  final Map<String, dynamic>? data;

  const PluginActionResult({
    required this.status,
    this.message,
    this.externalUrl,
    this.openUrl,
    this.downloadApiPath,
    this.fileName,
    this.mimeType,
    this.jobId,
    this.job,
    this.data,
  });

  bool get isCompleted => status == 'completed';
  bool get isClientHandoff => status == 'client_handoff';
  bool get isJobStarted => status == 'job_started';

  factory PluginActionResult.fromJson(Map<String, dynamic> json) {
    return PluginActionResult(
      status: json['status'] as String? ?? 'completed',
      message: json['message'] as String?,
      externalUrl: json['external_url'] as String?,
      openUrl: json['open_url'] as String?,
      downloadApiPath: json['download_api_path'] as String?,
      fileName: json['file_name'] as String?,
      mimeType: json['mime_type'] as String?,
      jobId: json['job_id'] as String?,
      job: json['job'] is Map
          ? PluginJob.fromJson(Map<String, dynamic>.from(json['job'] as Map))
          : null,
      data: json['data'] is Map
          ? Map<String, dynamic>.from(json['data'] as Map)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'status': status,
      if (message != null) 'message': message,
      if (externalUrl != null) 'external_url': externalUrl,
      if (openUrl != null) 'open_url': openUrl,
      if (downloadApiPath != null) 'download_api_path': downloadApiPath,
      if (fileName != null) 'file_name': fileName,
      if (mimeType != null) 'mime_type': mimeType,
      if (jobId != null) 'job_id': jobId,
      if (data != null) 'data': data,
    };
  }
}

class PluginJob {
  final String id;
  final String userId;
  final String plugin;
  final String action;
  final String title;
  final String targetType;
  final String targetId;
  final List<String> externalIds;
  final String status;
  final int current;
  final int total;
  final String? itemTitle;
  final double itemProgressPct;
  final String? message;
  final String? error;
  final PluginActionResult? result;
  final DateTime? startedAt;
  final DateTime? updatedAt;

  const PluginJob({
    required this.id,
    this.userId = '',
    required this.plugin,
    required this.action,
    required this.title,
    this.targetType = 'book',
    this.targetId = '',
    this.externalIds = const [],
    this.status = 'running',
    this.current = 0,
    this.total = 1,
    this.itemTitle,
    this.itemProgressPct = 0.0,
    this.message,
    this.error,
    this.result,
    this.startedAt,
    this.updatedAt,
  });

  bool get isActive => status == 'running' || status == 'queued';
  bool get isCompleted => status == 'completed';
  bool get isFailed => status == 'failed';

  double get progressFraction {
    if (isCompleted) return 1.0;
    if (itemProgressPct > 0) {
      return (itemProgressPct / 100.0).clamp(0.0, 1.0);
    }
    if (total > 0) {
      return (current / total).clamp(0.0, 1.0);
    }
    return 0.0;
  }

  factory PluginJob.fromJson(Map<String, dynamic> json) {
    return PluginJob(
      id: json['id'] as String? ?? '',
      userId: json['user_id'] as String? ?? '',
      plugin: json['plugin'] as String? ?? '',
      action: json['action'] as String? ?? '',
      title: json['title'] as String? ?? '',
      targetType: json['target_type'] as String? ?? 'book',
      targetId: json['target_id'] as String? ?? '',
      externalIds: (json['external_ids'] as List<dynamic>? ?? const [])
          .map((e) => e.toString())
          .where((e) => e.isNotEmpty)
          .toList(),
      status: json['status'] as String? ?? 'running',
      current: (json['current'] as num?)?.toInt() ?? 0,
      total: (json['total'] as num?)?.toInt() ?? 1,
      itemTitle: json['item_title'] as String?,
      itemProgressPct: (json['item_progress_pct'] as num?)?.toDouble() ?? 0.0,
      message: json['message'] as String?,
      error: json['error'] as String?,
      result: json['result'] is Map
          ? PluginActionResult.fromJson(
              Map<String, dynamic>.from(json['result'] as Map),
            )
          : null,
      startedAt: json['started_at'] is String
          ? DateTime.tryParse(json['started_at'] as String)
          : null,
      updatedAt: json['updated_at'] is String
          ? DateTime.tryParse(json['updated_at'] as String)
          : null,
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
  final PluginAuthSpec? auth;
  final List<PluginCommandDef> commands;
  final PluginCatalogDescriptor? catalog;
  final List<PluginActionDef> actions;
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
    this.auth,
    this.commands = const [],
    this.catalog,
    this.actions = const [],
    this.configured = true,
  });

  bool get hasCatalog =>
      capabilities.any((c) => c.toLowerCase() == 'catalog');

  bool get hasActions => actions.isNotEmpty;

  bool get hasAuth => auth != null && auth!.methods.isNotEmpty;

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
      auth: json['auth'] is Map
          ? PluginAuthSpec.fromJson(
              Map<String, dynamic>.from(json['auth'] as Map),
            )
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
      actions: (json['actions'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => PluginActionDef.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      configured: json['configured'] as bool? ?? true,
    );
  }
}

class PluginInstallResult {
  final PluginSummary plugin;
  final bool updated;
  final String? previousVersion;

  const PluginInstallResult({
    required this.plugin,
    this.updated = false,
    this.previousVersion,
  });

  factory PluginInstallResult.fromJson(Map<String, dynamic> json) {
    final rawPrev = json['previous_version'] as String?;
    return PluginInstallResult(
      plugin: PluginSummary.fromJson(json),
      updated: json['updated'] as bool? ?? false,
      previousVersion:
          (rawPrev != null && rawPrev.trim().isNotEmpty) ? rawPrev.trim() : null,
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
  final bool cached;
  final DateTime? cachedAt;

  const CatalogPage({
    this.items = const [],
    this.groups = const [],
    this.nextCursor,
    this.totalCount,
    this.cached = false,
    this.cachedAt,
  });

  bool get hasMore => nextCursor != null && nextCursor!.isNotEmpty;

  factory CatalogPage.fromJson(Map<String, dynamic> json) {
    final rawNext = json['next_cursor'] as String?;
    final rawCachedAt = json['cached_at'] as String?;
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
      cached: json['cached'] as bool? ?? false,
      cachedAt: (rawCachedAt != null && rawCachedAt.isNotEmpty)
          ? DateTime.tryParse(rawCachedAt)
          : null,
    );
  }
}

class CatalogStageItemRequest {
  final String externalId;
  final String? title;
  final String? preferredFormat;

  const CatalogStageItemRequest({
    required this.externalId,
    this.title,
    this.preferredFormat,
  });

  Map<String, dynamic> toJson() {
    return {
      'external_id': externalId,
      if (title != null && title!.isNotEmpty) 'title': title,
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
  final String? jobId;
  final PluginJob? job;
  final List<CatalogStagedEntry> results;
  final String? message;

  const CatalogStageResult({
    this.batchId,
    this.isAsync = false,
    this.jobId,
    this.job,
    this.results = const [],
    this.message,
  });

  factory CatalogStageResult.fromJson(Map<String, dynamic> json) {
    return CatalogStageResult(
      batchId: json['batch_id'] as String?,
      isAsync: json['async'] as bool? ?? false,
      jobId: json['job_id'] as String?,
      job: json['job'] is Map
          ? PluginJob.fromJson(Map<String, dynamic>.from(json['job'] as Map))
          : null,
      results: (json['results'] as List<dynamic>? ?? const [])
          .whereType<Map<String, dynamic>>()
          .map(CatalogStagedEntry.fromJson)
          .toList(),
      message: json['message'] as String?,
    );
  }
}
