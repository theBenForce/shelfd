import 'author.dart';
import 'genre.dart';
import 'series.dart';

class Book {
  final String id;
  final String title;
  final String synopsis;
  final String? coverUrl;
  final List<Author> authors;
  final List<Genre> genres;
  final Series? series;
  final double? seriesSequence;
  final double readingProgress;

  const Book({
    required this.id,
    required this.title,
    this.synopsis = '',
    this.coverUrl,
    this.authors = const [],
    this.genres = const [],
    this.series,
    this.seriesSequence,
    this.readingProgress = 0.0,
  });

  String get authorDisplay {
    if (authors.isEmpty) return 'Unknown Author';
    return authors.map((a) => a.name).join(', ');
  }

  factory Book.fromJson(Map<String, dynamic> json) {
    final rawAuthors = json['authors'] as List<dynamic>? ?? [];
    final rawGenres = json['genres'] as List<dynamic>? ?? [];
    final rawSeries = json['series'] as Map<String, dynamic>?;

    return Book(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      synopsis: json['synopsis'] as String? ?? '',
      coverUrl: json['cover_url'] as String?,
      authors: rawAuthors
          .map((a) => Author.fromJson(a as Map<String, dynamic>))
          .toList(),
      genres: rawGenres
          .map((g) => Genre.fromJson(g as Map<String, dynamic>))
          .toList(),
      series: rawSeries != null ? Series.fromJson(rawSeries) : null,
      seriesSequence: (json['series_sequence'] as num?)?.toDouble(),
      readingProgress: (json['reading_progress'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'synopsis': synopsis,
      if (coverUrl != null) 'cover_url': coverUrl,
      'authors': authors.map((a) => a.toJson()).toList(),
      'genres': genres.map((g) => g.toJson()).toList(),
      if (series != null) 'series': series!.toJson(),
      if (seriesSequence != null) 'series_sequence': seriesSequence,
      'reading_progress': readingProgress,
    };
  }

  Book copyWith({
    String? id,
    String? title,
    String? synopsis,
    String? coverUrl,
    List<Author>? authors,
    List<Genre>? genres,
    Series? series,
    double? seriesSequence,
    double? readingProgress,
  }) {
    return Book(
      id: id ?? this.id,
      title: title ?? this.title,
      synopsis: synopsis ?? this.synopsis,
      coverUrl: coverUrl ?? this.coverUrl,
      authors: authors ?? this.authors,
      genres: genres ?? this.genres,
      series: series ?? this.series,
      seriesSequence: seriesSequence ?? this.seriesSequence,
      readingProgress: readingProgress ?? this.readingProgress,
    );
  }
}
