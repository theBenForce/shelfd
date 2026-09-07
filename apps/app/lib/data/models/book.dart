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

  factory Book.fromJson(Map<String, dynamic> json, {String? baseUrl}) {
    final rawAuthors = json['authors'] as List<dynamic>? ?? [];
    final rawGenres = json['genres'] as List<dynamic>? ?? [];

    Series? seriesObj;
    double? seqNum;
    if (json['series'] is List && (json['series'] as List).isNotEmpty) {
      final firstSeries = (json['series'] as List).first;
      if (firstSeries is Map<String, dynamic>) {
        seriesObj = Series.fromJson(firstSeries);
        seqNum = (firstSeries['sequence_number'] as num?)?.toDouble();
      }
    } else if (json['series'] is Map<String, dynamic>) {
      seriesObj = Series.fromJson(json['series'] as Map<String, dynamic>);
      seqNum = (json['series_sequence'] as num?)?.toDouble();
    }

    final id = json['id'] as String? ?? '';
    String? cover = json['cover_url'] as String?;
    if (cover == null && json['cover_path'] != null) {
      final cleanBase = (baseUrl != null && baseUrl.isNotEmpty)
          ? (baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl)
          : '';
      cover = cleanBase.isNotEmpty ? '$cleanBase/api/v1/books/$id/cover' : '/api/v1/books/$id/cover';
    }

    return Book(
      id: id,
      title: json['title'] as String? ?? '',
      synopsis: json['synopsis'] as String? ?? json['description'] as String? ?? '',
      coverUrl: cover,
      authors: rawAuthors
          .whereType<Map<String, dynamic>>()
          .map((a) => Author.fromJson(a))
          .toList(),
      genres: rawGenres
          .whereType<Map<String, dynamic>>()
          .map((g) => Genre.fromJson(g))
          .toList(),
      series: seriesObj,
      seriesSequence: seqNum ?? (json['series_sequence'] as num?)?.toDouble(),
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
