class SemanticSearchHit {
  final String bookId;
  final String bookTitle;
  final String authorName;
  final String? chapterId;
  final int chapterIndex;
  final String chapterTitle;
  final String summary;
  final double score;

  const SemanticSearchHit({
    required this.bookId,
    required this.bookTitle,
    required this.authorName,
    this.chapterId,
    required this.chapterIndex,
    required this.chapterTitle,
    required this.summary,
    required this.score,
  });

  int get matchPercentage => (score * 100).round();

  factory SemanticSearchHit.fromJson(Map<String, dynamic> json) {
    return SemanticSearchHit(
      bookId: json['book_id'] as String? ?? '',
      bookTitle: json['book_title'] as String? ?? '',
      authorName: json['author_name'] as String? ?? '',
      chapterId: json['chapter_id'] as String?,
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      chapterTitle: json['chapter_title'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      score: (json['score'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'book_id': bookId,
      'book_title': bookTitle,
      'author_name': authorName,
      if (chapterId != null) 'chapter_id': chapterId,
      'chapter_index': chapterIndex,
      'chapter_title': chapterTitle,
      'summary': summary,
      'score': score,
    };
  }
}
