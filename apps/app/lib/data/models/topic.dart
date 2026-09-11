class Topic {
  final String id;
  final String name;
  final int bookCount;

  const Topic({
    required this.id,
    required this.name,
    this.bookCount = 0,
  });

  factory Topic.fromJson(Map<String, dynamic> json) {
    return Topic(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      bookCount: (json['book_count'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      if (bookCount > 0) 'book_count': bookCount,
    };
  }
}
