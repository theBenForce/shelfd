class Author {
  final String id;
  final String name;
  final String? sortName;

  const Author({
    required this.id,
    required this.name,
    this.sortName,
  });

  factory Author.fromJson(Map<String, dynamic> json) {
    return Author(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      sortName: json['sort_name'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      if (sortName != null) 'sort_name': sortName,
    };
  }
}
