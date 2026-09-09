class Author {
  final String id;
  final String name;
  final String? sortName;
  final String? photoUrl;
  final int bookCount;

  const Author({
    required this.id,
    required this.name,
    this.sortName,
    this.photoUrl,
    this.bookCount = 0,
  });

  factory Author.fromJson(Map<String, dynamic> json, {String? baseUrl}) {
    String? photo = json['photo_url'] as String?;
    if (photo != null && baseUrl != null && photo.startsWith('/')) {
      photo = '$baseUrl$photo';
    }

    return Author(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      sortName: json['sort_name'] as String?,
      photoUrl: photo,
      bookCount: (json['book_count'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      if (sortName != null) 'sort_name': sortName,
      if (photoUrl != null) 'photo_url': photoUrl,
      if (bookCount > 0) 'book_count': bookCount,
    };
  }
}
