class MetadataSearchResult {
  final String id;
  final String title;
  final String author;
  final String? description;
  final String? publisher;
  final int? publishedYear;
  final String? isbn;
  final String? language;
  final List<String> genres;
  final String? series;
  final String? seriesSequence;
  final String? coverUrl;
  final String provider;

  const MetadataSearchResult({
    required this.id,
    required this.title,
    required this.author,
    this.description,
    this.publisher,
    this.publishedYear,
    this.isbn,
    this.language,
    this.genres = const [],
    this.series,
    this.seriesSequence,
    this.coverUrl,
    required this.provider,
  });

  factory MetadataSearchResult.fromJson(Map<String, dynamic> json) {
    return MetadataSearchResult(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      author: json['author'] as String? ?? '',
      description: json['description'] as String?,
      publisher: json['publisher'] as String?,
      publishedYear: json['published_year'] as int?,
      isbn: json['isbn'] as String?,
      language: json['language'] as String?,
      genres: (json['genres'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
      series: json['series'] as String?,
      seriesSequence: json['series_sequence'] as String?,
      coverUrl: json['cover_url'] as String?,
      provider: json['provider'] as String? ?? 'unknown',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'author': author,
      'description': description,
      'publisher': publisher,
      'published_year': publishedYear,
      'isbn': isbn,
      'language': language,
      'genres': genres,
      'series': series,
      'series_sequence': seriesSequence,
      'cover_url': coverUrl,
      'provider': provider,
    };
  }
}
