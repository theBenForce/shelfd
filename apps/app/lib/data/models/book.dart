import 'author.dart';
import 'bookmark.dart';
import 'genre.dart';
import 'highlight.dart';
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
  final List<SpineItem> spine;
  final List<Bookmark> bookmarks;
  final List<Highlight> highlights;
  final String? publisher;
  final String? publishedDate;
  final String? language;
  final int? fileSizeBytes;
  final String layout;
  final String? renditionSpread;
  final String? renditionOrientation;
  final String? pageProgressionDirection;

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
    this.spine = const [],
    this.bookmarks = const [],
    this.highlights = const [],
    this.publisher,
    this.publishedDate,
    this.language,
    this.fileSizeBytes,
    this.layout = 'reflowable',
    this.renditionSpread,
    this.renditionOrientation,
    this.pageProgressionDirection,
  });

  bool get isFixedLayout => layout == 'pre-paginated';

  String get authorDisplay {
    if (authors.isEmpty) return 'Unknown Author';
    return authors.map((a) => a.name).join(', ');
  }

  factory Book.fromJson(Map<String, dynamic> json, {String? baseUrl}) {
    final rawAuthors = json['authors'] as List<dynamic>? ?? [];
    final rawGenres = json['genres'] as List<dynamic>? ?? [];
    final rawBookmarks = json['bookmarks'] as List<dynamic>? ?? [];
    final rawHighlights = json['highlights'] as List<dynamic>? ?? [];

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
      spine: (json['spine'] as List<dynamic>? ?? [])
          .whereType<Map<String, dynamic>>()
          .map((s) => SpineItem.fromJson(s))
          .toList(),
      bookmarks: rawBookmarks
          .whereType<Map<String, dynamic>>()
          .map((b) => Bookmark.fromJson(b))
          .toList(),
      highlights: rawHighlights
          .whereType<Map<String, dynamic>>()
          .map((h) => Highlight.fromJson(h))
          .toList(),
      publisher: json['publisher'] as String?,
      publishedDate: json['published_date'] as String?,
      language: json['language'] as String?,
      fileSizeBytes: (json['file_size_bytes'] as num?)?.toInt(),
      layout: json['layout'] as String? ?? 'reflowable',
      renditionSpread: json['rendition_spread'] as String?,
      renditionOrientation: json['rendition_orientation'] as String?,
      pageProgressionDirection: json['page_progression_direction'] as String?,
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
      if (spine.isNotEmpty) 'spine': spine.map((s) => s.toJson()).toList(),
      if (bookmarks.isNotEmpty) 'bookmarks': bookmarks.map((b) => b.toJson()).toList(),
      if (highlights.isNotEmpty) 'highlights': highlights.map((h) => h.toJson()).toList(),
      if (publisher != null) 'publisher': publisher,
      if (publishedDate != null) 'published_date': publishedDate,
      if (language != null) 'language': language,
      if (fileSizeBytes != null) 'file_size_bytes': fileSizeBytes,
      'layout': layout,
      if (renditionSpread != null) 'rendition_spread': renditionSpread,
      if (renditionOrientation != null) 'rendition_orientation': renditionOrientation,
      if (pageProgressionDirection != null) 'page_progression_direction': pageProgressionDirection,
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
    List<SpineItem>? spine,
    List<Bookmark>? bookmarks,
    List<Highlight>? highlights,
    String? publisher,
    String? publishedDate,
    String? language,
    int? fileSizeBytes,
    String? layout,
    String? renditionSpread,
    String? renditionOrientation,
    String? pageProgressionDirection,
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
      spine: spine ?? this.spine,
      bookmarks: bookmarks ?? this.bookmarks,
      highlights: highlights ?? this.highlights,
      publisher: publisher ?? this.publisher,
      publishedDate: publishedDate ?? this.publishedDate,
      language: language ?? this.language,
      fileSizeBytes: fileSizeBytes ?? this.fileSizeBytes,
      layout: layout ?? this.layout,
      renditionSpread: renditionSpread ?? this.renditionSpread,
      renditionOrientation: renditionOrientation ?? this.renditionOrientation,
      pageProgressionDirection: pageProgressionDirection ?? this.pageProgressionDirection,
    );
  }
}

class SpineItem {
  final String id;
  final String bookId;
  final int chapterIndex;
  final String title;
  final String summary;
  final String? href;
  final double? pageWidth;
  final double? pageHeight;
  final String? pageSpread;

  const SpineItem({
    required this.id,
    required this.bookId,
    required this.chapterIndex,
    required this.title,
    this.summary = '',
    this.href,
    this.pageWidth,
    this.pageHeight,
    this.pageSpread,
  });

  bool get isGatefold =>
      (pageWidth != null && pageHeight != null && pageHeight! > 0 && (pageWidth! / pageHeight! > 1.2)) ||
      pageSpread == 'center';

  factory SpineItem.fromJson(Map<String, dynamic> json) {
    return SpineItem(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      title: json['title'] as String? ?? 'Chapter ${json['chapter_index'] ?? 0}',
      summary: json['summary'] as String? ?? '',
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
      if (summary.isNotEmpty) 'summary': summary,
      if (href != null) 'href': href,
      if (pageWidth != null) 'page_width': pageWidth,
      if (pageHeight != null) 'page_height': pageHeight,
      if (pageSpread != null) 'page_spread': pageSpread,
    };
  }
}

