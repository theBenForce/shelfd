import 'plugin.dart';

class BackgroundJobKind {
  static const String plugin = 'plugin';
  static const String libraryScan = 'library_scan';
  static const String aiIndexing = 'ai_indexing';
  static const String uploadIngest = 'upload_ingest';
}

class BackgroundJob {
  final String id;
  final String kind;
  final String userId;
  final String title;
  final String? subtitle;
  final String status; // 'queued' | 'running' | 'completed' | 'failed'
  final int current;
  final int total;
  final String? itemTitle;
  final double itemProgressPct;
  final String? message;
  final String? error;
  final Map<String, dynamic>? metadata;
  final DateTime? startedAt;
  final DateTime? updatedAt;
  final DateTime? completedAt;

  const BackgroundJob({
    required this.id,
    required this.kind,
    this.userId = '',
    required this.title,
    this.subtitle,
    this.status = 'running',
    this.current = 0,
    this.total = 0,
    this.itemTitle,
    this.itemProgressPct = 0,
    this.message,
    this.error,
    this.metadata,
    this.startedAt,
    this.updatedAt,
    this.completedAt,
  });

  bool get isQueued => status == 'queued';
  bool get isRunning => status == 'running';
  bool get isActive => isQueued || isRunning;
  bool get isCompleted => status == 'completed';
  bool get isFailed => status == 'failed';
  bool get isFinished => isCompleted || isFailed;

  bool get isIndeterminate =>
      isActive && total <= 0 && itemProgressPct <= 0;

  double get progressFraction {
    if (isCompleted) return 1.0;
    if (total > 1) {
      if (itemProgressPct > 0 && kind == BackgroundJobKind.plugin) {
        final completedItems = (current > 0 ? current - 1 : 0).clamp(0, total);
        final itemFrac = (itemProgressPct / 100.0).clamp(0.0, 1.0);
        return ((completedItems + itemFrac) / total).clamp(0.0, 1.0);
      }
      return (current / total).clamp(0.0, 1.0);
    }
    if (itemProgressPct > 0) {
      return (itemProgressPct / 100.0).clamp(0.0, 1.0);
    }
    if (total == 1 && current >= 1) return 1.0;
    return 0.0;
  }

  String get kindLabel {
    switch (kind) {
      case BackgroundJobKind.plugin:
        final pluginName = metadata?['plugin']?.toString();
        if (pluginName != null && pluginName.trim().isNotEmpty) {
          return pluginName.trim();
        }
        return 'Plugin';
      case BackgroundJobKind.libraryScan:
        return 'Library Scan';
      case BackgroundJobKind.aiIndexing:
        return 'AI Indexing';
      case BackgroundJobKind.uploadIngest:
        return 'Upload';
      default:
        return 'Task';
    }
  }

  String get displayTitle {
    if (title.trim().isNotEmpty) return title.trim();
    return kindLabel;
  }

  String get displayStatusMessage {
    if (isFailed) {
      if (error != null && error!.trim().isNotEmpty) return error!.trim();
      if (message != null && message!.trim().isNotEmpty) return message!.trim();
      return 'Task failed';
    }
    if (message != null && message!.trim().isNotEmpty) {
      return message!.trim();
    }
    if (isCompleted) return 'Completed';
    if (isQueued) return 'Queued';
    return 'Processing...';
  }

  factory BackgroundJob.fromJson(Map<String, dynamic> json) {
    return BackgroundJob(
      id: json['id']?.toString() ?? '',
      kind: json['kind']?.toString() ?? BackgroundJobKind.plugin,
      userId: json['user_id']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      subtitle: json['subtitle'] as String?,
      status: json['status']?.toString() ?? 'running',
      current: (json['current'] as num?)?.toInt() ?? 0,
      total: (json['total'] as num?)?.toInt() ?? 0,
      itemTitle: json['item_title'] as String?,
      itemProgressPct: (json['item_progress_pct'] as num?)?.toDouble() ?? 0,
      message: json['message'] as String?,
      error: json['error'] as String?,
      metadata: json['metadata'] is Map
          ? Map<String, dynamic>.from(json['metadata'] as Map)
          : null,
      startedAt: json['started_at'] != null
          ? DateTime.tryParse(json['started_at'].toString())
          : null,
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString())
          : null,
      completedAt: json['completed_at'] != null
          ? DateTime.tryParse(json['completed_at'].toString())
          : null,
    );
  }

  factory BackgroundJob.fromPluginJob(PluginJob job) {
    final subtitle = job.action.isNotEmpty
        ? '${job.plugin} • ${job.action}'
        : job.plugin;
    return BackgroundJob(
      id: job.id,
      kind: BackgroundJobKind.plugin,
      userId: job.userId,
      title: job.title.isNotEmpty ? job.title : job.action,
      subtitle: subtitle,
      status: job.status,
      current: job.current,
      total: job.total,
      itemTitle: job.itemTitle,
      itemProgressPct: job.itemProgressPct,
      message: job.message,
      error: job.error,
      metadata: {
        'plugin': job.plugin,
        'action': job.action,
        'target_type': job.targetType,
        'target_id': job.targetId,
      },
      startedAt: job.startedAt,
      updatedAt: job.updatedAt,
      completedAt: (job.isCompleted || job.isFailed) ? job.updatedAt : null,
    );
  }
}
