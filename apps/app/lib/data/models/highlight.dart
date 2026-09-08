class Highlight {
  final String id;
  final String bookId;
  final String? chapterId;
  final String selectedText;
  final String? note;
  final String color;
  final DateTime? createdAt;

  const Highlight({
    required this.id,
    required this.bookId,
    this.chapterId,
    required this.selectedText,
    this.note,
    this.color = 'yellow',
    this.createdAt,
  });

  factory Highlight.fromJson(Map<String, dynamic> json) {
    return Highlight(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterId: json['chapter_id'] as String?,
      selectedText: json['selected_text'] as String? ?? '',
      note: json['note'] as String?,
      color: json['color'] as String? ?? 'yellow',
      createdAt: json['created_at'] != null ? DateTime.tryParse(json['created_at'] as String) : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      if (chapterId != null) 'chapter_id': chapterId,
      'selected_text': selectedText,
      if (note != null) 'note': note,
      'color': color,
      if (createdAt != null) 'created_at': createdAt!.toIso8601String(),
    };
  }

  Highlight copyWith({
    String? id,
    String? bookId,
    String? chapterId,
    String? selectedText,
    String? note,
    String? color,
    DateTime? createdAt,
  }) {
    return Highlight(
      id: id ?? this.id,
      bookId: bookId ?? this.bookId,
      chapterId: chapterId ?? this.chapterId,
      selectedText: selectedText ?? this.selectedText,
      note: note ?? this.note,
      color: color ?? this.color,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
