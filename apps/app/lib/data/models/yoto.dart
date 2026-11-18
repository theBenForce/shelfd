class YotoStatus {
  final bool isLinked;
  final String? clientId;
  final String? defaultClientId;
  final String authBaseUrl;
  final DateTime? expiresAt;

  const YotoStatus({
    this.isLinked = false,
    this.clientId,
    this.defaultClientId,
    this.authBaseUrl = 'https://login.yotoplay.com',
    this.expiresAt,
  });

  factory YotoStatus.fromJson(Map<String, dynamic> json) {
    final expiresAtStr = json['expires_at'] as String?;
    return YotoStatus(
      isLinked: json['is_linked'] as bool? ?? false,
      clientId: json['client_id'] as String?,
      defaultClientId: json['default_client_id'] as String?,
      authBaseUrl: (json['auth_base_url'] as String?)?.isNotEmpty == true
          ? json['auth_base_url'] as String
          : 'https://login.yotoplay.com',
      expiresAt: expiresAtStr != null ? DateTime.tryParse(expiresAtStr) : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'is_linked': isLinked,
      if (clientId != null) 'client_id': clientId,
      if (defaultClientId != null) 'default_client_id': defaultClientId,
      'auth_base_url': authBaseUrl,
      if (expiresAt != null) 'expires_at': expiresAt!.toIso8601String(),
    };
  }
}

class YotoPlaylist {
  final String cardId;
  final String title;
  final String? coverUrl;
  final int chapterCount;
  final String? updatedAt;

  const YotoPlaylist({
    required this.cardId,
    required this.title,
    this.coverUrl,
    this.chapterCount = 0,
    this.updatedAt,
  });

  factory YotoPlaylist.fromJson(Map<String, dynamic> json) {
    return YotoPlaylist(
      cardId: (json['card_id'] ?? json['cardId'] ?? '') as String,
      title: (json['title'] ?? 'Untitled Playlist') as String,
      coverUrl: (json['cover_url'] ?? json['coverUrl']) as String?,
      chapterCount: (json['chapter_count'] as num?)?.toInt() ?? 0,
      updatedAt: (json['updated_at'] ?? json['updatedAt']) as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'card_id': cardId,
      'title': title,
      if (coverUrl != null) 'cover_url': coverUrl,
      'chapter_count': chapterCount,
      if (updatedAt != null) 'updated_at': updatedAt,
    };
  }
}

class YotoDisplayIcon {
  final String displayIconId;
  final String mediaId;
  final String title;
  final String url;
  final List<String> publicTags;
  final bool isPublic;

  const YotoDisplayIcon({
    required this.displayIconId,
    required this.mediaId,
    this.title = '',
    this.url = '',
    this.publicTags = const [],
    this.isPublic = true,
  });

  /// Returns the canonical `yoto:#<mediaId>` URI used in `display.icon16x16`.
  String get yotoUri {
    final trimmed = mediaId.trim();
    if (trimmed.isEmpty) {
      return '';
    }
    if (trimmed.startsWith('yoto:#')) {
      return trimmed;
    }
    if (trimmed.startsWith('yoto:')) {
      return 'yoto:#${trimmed.substring(5)}';
    }
    if (trimmed.startsWith('#')) {
      return 'yoto:$trimmed';
    }
    return 'yoto:#$trimmed';
  }

  factory YotoDisplayIcon.fromJson(Map<String, dynamic> json) {
    final tagsRaw = json['publicTags'] ?? json['public_tags'];
    final tags = tagsRaw is List
        ? tagsRaw.map((e) => e.toString()).toList()
        : const <String>[];
    final rawMediaId = (json['mediaId'] ?? json['media_id'] ?? '') as String;
    final cleanMediaId = rawMediaId
        .trim()
        .replaceFirst(RegExp(r'^yoto:#?'), '')
        .replaceFirst(RegExp(r'^#'), '');
    return YotoDisplayIcon(
      displayIconId: (json['displayIconId'] ?? json['display_icon_id'] ?? cleanMediaId) as String,
      mediaId: cleanMediaId,
      title: (json['title'] ?? '') as String,
      url: (json['url'] ?? '') as String,
      publicTags: tags,
      isPublic: json['public'] as bool? ?? true,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'displayIconId': displayIconId,
      'mediaId': mediaId,
      'title': title,
      'url': url,
      'publicTags': publicTags,
      'public': isPublic,
    };
  }
}

class YotoIconsCatalog {
  final List<YotoDisplayIcon> publicIcons;
  final List<YotoDisplayIcon> userIcons;
  final String defaultIconUri;

  const YotoIconsCatalog({
    this.publicIcons = const [],
    this.userIcons = const [],
    this.defaultIconUri = 'yoto:#gTMbacpoeSMYqc9fNLJnxPjylraNG6jIrYEWevyzYbA',
  });

  List<YotoDisplayIcon> get allIcons => [...userIcons, ...publicIcons];

  YotoDisplayIcon? findByUriOrMediaId(String? uriOrId) {
    if (uriOrId == null || uriOrId.isEmpty) return null;
    final clean = uriOrId
        .trim()
        .replaceFirst(RegExp(r'^yoto:#?'), '')
        .replaceFirst(RegExp(r'^#'), '');
    for (final icon in allIcons) {
      if (icon.mediaId == clean || icon.yotoUri == uriOrId || icon.displayIconId == clean) {
        return icon;
      }
    }
    return null;
  }

  factory YotoIconsCatalog.fromJson(Map<String, dynamic> json) {
    final pubList = json['public_icons'] as List<dynamic>? ?? const [];
    final usrList = json['user_icons'] as List<dynamic>? ?? const [];
    return YotoIconsCatalog(
      publicIcons: pubList
          .whereType<Map<String, dynamic>>()
          .map(YotoDisplayIcon.fromJson)
          .toList(),
      userIcons: usrList
          .whereType<Map<String, dynamic>>()
          .map(YotoDisplayIcon.fromJson)
          .toList(),
      defaultIconUri: (json['default_icon_uri'] as String?) ??
          'yoto:#gTMbacpoeSMYqc9fNLJnxPjylraNG6jIrYEWevyzYbA',
    );
  }
}

class SendToYotoRequest {
  final String mode; // 'append', 'replace', 'existing', or 'new'
  final String? cardId;
  final String? newPlaylistTitle;
  final bool includeCover;
  final bool useCoverAsIcon;
  final String? defaultIconId;
  final Map<int, String>? chapterIcons;
  final List<String>? bookIds;

  const SendToYotoRequest({
    required this.mode,
    this.cardId,
    this.newPlaylistTitle,
    this.includeCover = true,
    this.useCoverAsIcon = false,
    this.defaultIconId,
    this.chapterIcons,
    this.bookIds,
  });

  Map<String, dynamic> toJson() {
    return {
      'mode': mode,
      if (cardId != null && cardId!.isNotEmpty) 'card_id': cardId,
      if (newPlaylistTitle != null && newPlaylistTitle!.isNotEmpty)
        'new_playlist_title': newPlaylistTitle,
      'include_cover': includeCover,
      'use_cover_as_icon': useCoverAsIcon,
      if (defaultIconId != null && defaultIconId!.isNotEmpty)
        'default_icon_id': defaultIconId,
      if (chapterIcons != null && chapterIcons!.isNotEmpty)
        'chapter_icons': chapterIcons!.map((k, v) => MapEntry(k.toString(), v)),
      if (bookIds != null && bookIds!.isNotEmpty) 'book_ids': bookIds,
    };
  }
}

class SendToYotoResult {
  final String status;
  final String cardId;
  final String title;
  final int chapterCount;
  final String message;

  const SendToYotoResult({
    required this.status,
    required this.cardId,
    required this.title,
    required this.chapterCount,
    required this.message,
  });

  factory SendToYotoResult.fromJson(Map<String, dynamic> json) {
    return SendToYotoResult(
      status: (json['status'] ?? 'sent') as String,
      cardId: (json['card_id'] ?? '') as String,
      title: (json['title'] ?? '') as String,
      chapterCount: (json['chapter_count'] as num?)?.toInt() ?? 0,
      message: (json['message'] ?? 'Sent to Yoto') as String,
    );
  }
}

class YotoServerUploadProgress {
  final bool isActive;
  final String? title;
  final String? currentItem;
  final String? stage;
  final double currentProgress;
  final double overallProgress;
  final int completedItems;
  final int totalItems;

  const YotoServerUploadProgress({
    this.isActive = false,
    this.title,
    this.currentItem,
    this.stage,
    this.currentProgress = 0.0,
    this.overallProgress = 0.0,
    this.completedItems = 0,
    this.totalItems = 0,
  });

  factory YotoServerUploadProgress.fromJson(Map<String, dynamic> json) {
    return YotoServerUploadProgress(
      isActive: json['is_active'] as bool? ?? false,
      title: json['title'] as String?,
      currentItem: json['current_item'] as String?,
      stage: json['stage'] as String?,
      currentProgress: ((json['current_progress'] as num?)?.toDouble() ?? 0.0).clamp(0.0, 1.0),
      overallProgress: ((json['overall_progress'] as num?)?.toDouble() ?? 0.0).clamp(0.0, 1.0),
      completedItems: (json['completed_items'] as num?)?.toInt() ?? 0,
      totalItems: (json['total_items'] as num?)?.toInt() ?? 0,
    );
  }
}
