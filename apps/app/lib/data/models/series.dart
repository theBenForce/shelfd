class Series {
  final String id;
  final String name;
  final int bookCount;

  const Series({
    required this.id,
    required this.name,
    this.bookCount = 0,
  });

  factory Series.fromJson(Map<String, dynamic> json) {
    return Series(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      bookCount: (json['book_count'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'book_count': bookCount,
    };
  }
}
