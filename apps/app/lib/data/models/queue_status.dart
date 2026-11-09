class QueueStatus {
  final int totalChapters;
  final int indexedChapters;
  final int pendingChapters;
  final int pendingUploads;
  final int stagedUploads;
  final double progressPercent;
  final bool isActive;
  final String? currentBook;
  final String? currentChapter;
  final String? currentTask;

  const QueueStatus({
    this.totalChapters = 0,
    this.indexedChapters = 0,
    this.pendingChapters = 0,
    this.pendingUploads = 0,
    this.stagedUploads = 0,
    this.progressPercent = 0.0,
    this.isActive = false,
    this.currentBook,
    this.currentChapter,
    this.currentTask,
  });

  factory QueueStatus.fromJson(Map<String, dynamic> json) {
    return QueueStatus(
      totalChapters: (json['total_chapters'] as num?)?.toInt() ?? 0,
      indexedChapters: (json['indexed_chapters'] as num?)?.toInt() ?? 0,
      pendingChapters: (json['pending_chapters'] as num?)?.toInt() ?? 0,
      pendingUploads: (json['pending_uploads'] as num?)?.toInt() ?? 0,
      stagedUploads: (json['staged_uploads'] as num?)?.toInt() ?? 0,
      progressPercent: (json['progress_percent'] as num?)?.toDouble() ?? 0.0,
      isActive: json['is_active'] as bool? ?? false,
      currentBook: json['current_book'] as String?,
      currentChapter: json['current_chapter'] as String?,
      currentTask: json['current_task'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'total_chapters': totalChapters,
      'indexed_chapters': indexedChapters,
      'pending_chapters': pendingChapters,
      'pending_uploads': pendingUploads,
      'staged_uploads': stagedUploads,
      'progress_percent': progressPercent,
      'is_active': isActive,
      if (currentBook != null) 'current_book': currentBook,
      if (currentChapter != null) 'current_chapter': currentChapter,
      if (currentTask != null) 'current_task': currentTask,
    };
  }
}
