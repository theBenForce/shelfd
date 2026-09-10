class Chapter {
  final String id;
  final String bookId;
  final int chapterIndex;
  final String title;
  final String summary;
  final String? content;
  final String? href;
  final double? pageWidth;
  final double? pageHeight;
  final String? pageSpread;

  const Chapter({
    required this.id,
    required this.bookId,
    required this.chapterIndex,
    required this.title,
    this.summary = '',
    this.content,
    this.href,
    this.pageWidth,
    this.pageHeight,
    this.pageSpread,
  });

  factory Chapter.fromJson(Map<String, dynamic> json) {
    return Chapter(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      content: (json['content_plain'] ?? json['content']) as String?,
      href: json['href'] as String?,
      pageWidth: (json['page_width'] as num?)?.toDouble(),
      pageHeight: (json['page_height'] as num?)?.toDouble(),
      pageSpread: json['page_spread'] as String?,
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
      if (href != null) 'href': href,
      if (pageWidth != null) 'page_width': pageWidth,
      if (pageHeight != null) 'page_height': pageHeight,
      if (pageSpread != null) 'page_spread': pageSpread,
    };
  }
}
