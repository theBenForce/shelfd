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

  const QueueStatus({
    required this.totalChapters,
    required this.indexedChapters,
    required this.pendingChapters,
    required this.pendingUploads,
    this.stagedUploads = 0,
    required this.progressPercent,
    required this.isActive,
    this.currentBook,
    this.currentChapter,
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
    };
  }
}
