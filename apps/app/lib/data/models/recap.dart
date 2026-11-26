class BookRecap {
  final String bookId;
  final String bookTitle;
  final int chapterIndex;
  final int paragraphOffset;
  final String recap;
  final bool cached;
  final int chaptersIncluded;
  final DateTime? createdAt;

  const BookRecap({
    required this.bookId,
    required this.bookTitle,
    required this.chapterIndex,
    this.paragraphOffset = 0,
    required this.recap,
    this.cached = false,
    this.chaptersIncluded = 0,
    this.createdAt,
  });

  factory BookRecap.fromJson(Map<String, dynamic> json) {
    final rawCreatedAt = json['created_at'] as String?;
    return BookRecap(
      bookId: (json['book_id'] as String?) ?? '',
      bookTitle: (json['book_title'] as String?) ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      paragraphOffset: (json['paragraph_offset'] as num?)?.toInt() ?? 0,
      recap: (json['recap'] as String?) ?? '',
      cached: (json['cached'] as bool?) ?? false,
      chaptersIncluded: (json['chapters_included'] as num?)?.toInt() ?? 0,
      createdAt: rawCreatedAt != null ? DateTime.tryParse(rawCreatedAt) : null,
    );
  }
}
