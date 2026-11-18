import 'audiobook.dart';
import 'author.dart';
import 'bookmark.dart';
import 'genre.dart';
import 'highlight.dart';
import 'series.dart';
import 'topic.dart';

export 'audiobook.dart';

class Book {
  final String id;
  final String title;
  final String synopsis;
  final String? coverUrl;
  final List<Author> authors;
  final List<Genre> genres;
  final List<Topic> topics;
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
  final String bookType;
  final double? durationSeconds;
  final bool hasReadalong;
  final int? bitrateKbps;
  final List<BookFile> files;
  final List<AudioChapter> audioChapters;

  const Book({
    required this.id,
    required this.title,
    this.synopsis = '',
    this.coverUrl,
    this.authors = const [],
    this.genres = const [],
    this.topics = const [],
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
    this.bookType = 'ebook',
    this.durationSeconds,
    this.hasReadalong = false,
    this.bitrateKbps,
    this.files = const [],
    this.audioChapters = const [],
  });

  bool get isFixedLayout => layout == 'pre-paginated';
  bool get isAudiobook => bookType == 'audiobook';
  bool get canReadAlong => hasReadalong || (!isAudiobook && (durationSeconds ?? 0) > 0);
  bool get hasAudio =>
      isAudiobook ||
      hasReadalong ||
      (durationSeconds != null && durationSeconds! > 0) ||
      files.any((f) =>
          f.fileType == 'audiobook' ||
          f.fileType == 'audio' ||
          f.durationSeconds != null ||
          f.filePath.toLowerCase().endsWith('.m4b') ||
          f.filePath.toLowerCase().endsWith('.m4a') ||
          f.filePath.toLowerCase().endsWith('.mp3'));

  List<SpineItem> get namedSpine => spine.where((s) => s.hasNamedChapter).toList();

  String get authorDisplay {
    if (authors.isEmpty) return 'Unknown Author';
    return authors.map((a) => a.name).join(', ');
  }

  factory Book.fromJson(Map<String, dynamic> json, {String? baseUrl}) {
    final rawAuthors = json['authors'] as List<dynamic>? ?? [];
    final rawGenres = json['genres'] as List<dynamic>? ?? [];
    final rawTopics = json['topics'] as List<dynamic>? ?? [];
    final rawBookmarks = json['bookmarks'] as List<dynamic>? ?? [];
    final rawHighlights = json['highlights'] as List<dynamic>? ?? [];
    final rawFiles = json['files'] as List<dynamic>? ?? [];
    final rawAudioChapters = json['audio_chapters'] as List<dynamic>? ?? [];

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

    final durationSec = (json['duration_seconds'] as num?)?.toDouble();
    final parsedFiles = rawFiles
        .whereType<Map<String, dynamic>>()
        .map((f) => BookFile.fromJson(f))
        .toList();
    final parsedBookType = json['book_type'] as String? ?? 'ebook';
    final explicitReadalong = json['has_readalong'] as bool?;
    final inferredReadalong = explicitReadalong ??
        (parsedBookType != 'audiobook' &&
            ((durationSec != null && durationSec > 0) ||
                parsedFiles.any((f) => f.fileType == 'audiobook' || f.fileType == 'audio')));

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
      topics: rawTopics
          .whereType<Map<String, dynamic>>()
          .map((t) => Topic.fromJson(t))
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
      bookType: parsedBookType,
      durationSeconds: durationSec,
      hasReadalong: inferredReadalong,
      bitrateKbps: (json['bitrate_kbps'] as num?)?.toInt(),
      files: parsedFiles,
      audioChapters: rawAudioChapters
          .whereType<Map<String, dynamic>>()
          .map((c) => AudioChapter.fromJson(c))
          .toList(),
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
      'topics': topics.map((t) => t.toJson()).toList(),
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
      'book_type': bookType,
      if (durationSeconds != null) 'duration_seconds': durationSeconds,
      'has_readalong': hasReadalong,
      if (bitrateKbps != null) 'bitrate_kbps': bitrateKbps,
      if (files.isNotEmpty) 'files': files.map((f) => f.toJson()).toList(),
      if (audioChapters.isNotEmpty) 'audio_chapters': audioChapters.map((c) => c.toJson()).toList(),
    };
  }

  Book copyWith({
    String? id,
    String? title,
    String? synopsis,
    String? coverUrl,
    List<Author>? authors,
    List<Genre>? genres,
    List<Topic>? topics,
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
    String? bookType,
    double? durationSeconds,
    bool? hasReadalong,
    int? bitrateKbps,
    List<BookFile>? files,
    List<AudioChapter>? audioChapters,
  }) {
    return Book(
      id: id ?? this.id,
      title: title ?? this.title,
      synopsis: synopsis ?? this.synopsis,
      coverUrl: coverUrl ?? this.coverUrl,
      authors: authors ?? this.authors,
      genres: genres ?? this.genres,
      topics: topics ?? this.topics,
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
      bookType: bookType ?? this.bookType,
      durationSeconds: durationSeconds ?? this.durationSeconds,
      hasReadalong: hasReadalong ?? this.hasReadalong,
      bitrateKbps: bitrateKbps ?? this.bitrateKbps,
      files: files ?? this.files,
      audioChapters: audioChapters ?? this.audioChapters,
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

  bool get hasNamedChapter {
    final trimmed = title.trim();
    if (trimmed.isEmpty) return false;
    final lower = trimmed.toLowerCase();
    if (lower.startsWith('page ') || lower.startsWith('page\u00a0')) {
      final rest = lower.substring(5).trim();
      if (int.tryParse(rest) != null) return false;
    }
    return true;
  }

  factory SpineItem.fromJson(Map<String, dynamic> json) {
    return SpineItem(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      title: json['title'] as String? ?? '',
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

class ReadalongSegment {
  final int chapterIndex;
  final String chapterHref;
  final String overlayId;
  final String parId;
  final String fragmentId;
  final String text;
  final String audioSrc;
  final double clipBegin;
  final double clipEnd;
  final double globalBegin;
  final double globalEnd;

  const ReadalongSegment({
    required this.chapterIndex,
    this.chapterHref = '',
    this.overlayId = '',
    this.parId = '',
    this.fragmentId = '',
    this.text = '',
    this.audioSrc = '',
    required this.clipBegin,
    required this.clipEnd,
    required this.globalBegin,
    required this.globalEnd,
  });

  factory ReadalongSegment.fromJson(Map<String, dynamic> json) {
    final clipBegin = (json['clip_begin'] as num?)?.toDouble() ?? 0.0;
    final clipEnd = (json['clip_end'] as num?)?.toDouble() ?? 0.0;
    return ReadalongSegment(
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 1,
      chapterHref: json['chapter_href'] as String? ?? '',
      overlayId: json['overlay_id'] as String? ?? '',
      parId: json['par_id'] as String? ?? '',
      fragmentId: json['fragment_id'] as String? ?? '',
      text: json['text'] as String? ?? '',
      audioSrc: json['audio_src'] as String? ?? '',
      clipBegin: clipBegin,
      clipEnd: clipEnd,
      globalBegin: (json['global_begin'] as num?)?.toDouble() ?? clipBegin,
      globalEnd: (json['global_end'] as num?)?.toDouble() ?? clipEnd,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'chapter_index': chapterIndex,
      if (chapterHref.isNotEmpty) 'chapter_href': chapterHref,
      if (overlayId.isNotEmpty) 'overlay_id': overlayId,
      if (parId.isNotEmpty) 'par_id': parId,
      if (fragmentId.isNotEmpty) 'fragment_id': fragmentId,
      if (text.isNotEmpty) 'text': text,
      if (audioSrc.isNotEmpty) 'audio_src': audioSrc,
      'clip_begin': clipBegin,
      'clip_end': clipEnd,
      'global_begin': globalBegin,
      'global_end': globalEnd,
    };
  }
}

class ReadalongManifest {
  final String bookId;
  final bool hasReadalong;
  final bool hasMediaOverlays;
  final String activeClass;
  final String playbackActiveClass;
  final String? narrator;
  final double? durationSeconds;
  final String audioStreamUrl;
  final List<ReadalongSegment> segments;

  const ReadalongManifest({
    required this.bookId,
    this.hasReadalong = false,
    this.hasMediaOverlays = false,
    this.activeClass = '-epub-media-overlay-active',
    this.playbackActiveClass = '-epub-media-overlay-playing',
    this.narrator,
    this.durationSeconds,
    this.audioStreamUrl = '',
    this.segments = const [],
  });

  ReadalongSegment? segmentAt(double positionSeconds) {
    if (segments.isEmpty) return null;
    for (final seg in segments) {
      if (positionSeconds >= seg.globalBegin && positionSeconds < seg.globalEnd) {
        return seg;
      }
    }
    if (positionSeconds >= segments.last.globalEnd &&
        (positionSeconds - segments.last.globalEnd) < 0.5) {
      return segments.last;
    }
    return null;
  }

  ReadalongSegment? firstSegmentForChapter(int chapterIndex) {
    for (final seg in segments) {
      if (seg.chapterIndex == chapterIndex) {
        return seg;
      }
    }
    return null;
  }

  factory ReadalongManifest.fromJson(Map<String, dynamic> json) {
    final rawSegments = json['segments'] as List<dynamic>? ?? const [];
    final activeCls = (json['active_class'] as String?)?.trim();
    final playbackCls = (json['playback_active_class'] as String?)?.trim();
    return ReadalongManifest(
      bookId: json['book_id'] as String? ?? '',
      hasReadalong: json['has_readalong'] as bool? ?? false,
      hasMediaOverlays: json['has_media_overlays'] as bool? ?? false,
      activeClass: (activeCls != null && activeCls.isNotEmpty)
          ? activeCls
          : '-epub-media-overlay-active',
      playbackActiveClass: (playbackCls != null && playbackCls.isNotEmpty)
          ? playbackCls
          : '-epub-media-overlay-playing',
      narrator: json['narrator'] as String?,
      durationSeconds: (json['duration_seconds'] as num?)?.toDouble(),
      audioStreamUrl: json['audio_stream_url'] as String? ?? '',
      segments: rawSegments
          .whereType<Map<String, dynamic>>()
          .map(ReadalongSegment.fromJson)
          .toList(),
    );
  }
}


