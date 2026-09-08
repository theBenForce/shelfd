class Chapter {
  final String id;
  final String bookId;
  final int chapterIndex;
  final String title;
  final String summary;
  final String? content;

  const Chapter({
    required this.id,
    required this.bookId,
    required this.chapterIndex,
    required this.title,
    this.summary = '',
    this.content,
  });

  factory Chapter.fromJson(Map<String, dynamic> json) {
    return Chapter(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      content: (json['content_plain'] ?? json['content']) as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      'chapter_index': chapterIndex,
      'title': title,
      'summary': summary,
      if (content != null) 'content': content,
    };
  }
}
