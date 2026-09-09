class Series {
  final String id;
  final String name;
  final int bookCount;
  final String? coverBookId;
  final String? coverUrl;

  const Series({
    required this.id,
    required this.name,
    this.bookCount = 0,
    this.coverBookId,
    this.coverUrl,
  });

  factory Series.fromJson(Map<String, dynamic> json, {String? baseUrl}) {
    final coverBookId = json['cover_book_id'] as String?;
    String? cover;
    if (coverBookId != null && coverBookId.isNotEmpty) {
      final cleanBase = (baseUrl != null && baseUrl.isNotEmpty)
          ? (baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl)
          : '';
      cover = cleanBase.isNotEmpty ? '$cleanBase/api/v1/books/$coverBookId/cover' : '/api/v1/books/$coverBookId/cover';
    }
    return Series(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      bookCount: (json['book_count'] as num?)?.toInt() ?? 0,
      coverBookId: coverBookId,
      coverUrl: cover,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'book_count': bookCount,
      if (coverBookId != null) 'cover_book_id': coverBookId,
      if (coverUrl != null) 'cover_url': coverUrl,
    };
  }
}
