class Bookmark {
  final String id;
  final String bookId;
  final String? chapterId;
  final String title;
  final double progress;
  final DateTime? createdAt;

  const Bookmark({
    required this.id,
    required this.bookId,
    this.chapterId,
    required this.title,
    this.progress = 0.0,
    this.createdAt,
  });

  factory Bookmark.fromJson(Map<String, dynamic> json) {
    return Bookmark(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterId: json['chapter_id'] as String?,
      title: json['title'] as String? ?? 'Bookmark',
      progress: (json['progress'] as num?)?.toDouble() ?? 0.0,
      createdAt: json['created_at'] != null ? DateTime.tryParse(json['created_at'] as String) : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      if (chapterId != null) 'chapter_id': chapterId,
      'title': title,
      'progress': progress,
      if (createdAt != null) 'created_at': createdAt!.toIso8601String(),
    };
  }

  Bookmark copyWith({
    String? id,
    String? bookId,
    String? chapterId,
    String? title,
    double? progress,
    DateTime? createdAt,
  }) {
    return Bookmark(
      id: id ?? this.id,
      bookId: bookId ?? this.bookId,
      chapterId: chapterId ?? this.chapterId,
      title: title ?? this.title,
      progress: progress ?? this.progress,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
