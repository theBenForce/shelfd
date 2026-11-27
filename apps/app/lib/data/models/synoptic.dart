class SynopticCitation {
  final String bookId;
  final String bookTitle;
  final String authorName;
  final String chapterId;
  final String chapterTitle;
  final int chapterIndex;
  final int startParagraph;
  final int endParagraph;
  final String excerpt;
  final double? cosineDistance;

  const SynopticCitation({
    required this.bookId,
    required this.bookTitle,
    this.authorName = '',
    this.chapterId = '',
    this.chapterTitle = '',
    required this.chapterIndex,
    this.startParagraph = 0,
    this.endParagraph = 0,
    this.excerpt = '',
    this.cosineDistance,
  });

  String get chipLabel => '$bookTitle · Ch. $chapterIndex';

  factory SynopticCitation.fromJson(Map<String, dynamic> json) {
    return SynopticCitation(
      bookId: json['book_id'] as String? ?? '',
      bookTitle: json['book_title'] as String? ?? '',
      authorName: json['author_name'] as String? ?? '',
      chapterId: json['chapter_id'] as String? ?? '',
      chapterTitle: json['chapter_title'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      startParagraph: (json['start_paragraph'] as num?)?.toInt() ?? 0,
      endParagraph: (json['end_paragraph'] as num?)?.toInt() ?? 0,
      excerpt: json['excerpt'] as String? ?? '',
      cosineDistance: (json['cosine_distance'] as num?)?.toDouble(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'book_id': bookId,
      'book_title': bookTitle,
      if (authorName.isNotEmpty) 'author_name': authorName,
      'chapter_id': chapterId,
      if (chapterTitle.isNotEmpty) 'chapter_title': chapterTitle,
      'chapter_index': chapterIndex,
      'start_paragraph': startParagraph,
      'end_paragraph': endParagraph,
      'excerpt': excerpt,
      if (cosineDistance != null) 'cosine_distance': cosineDistance,
    };
  }
}

class SynopticPoint {
  final String topic;
  final String description;
  final List<int> citationIndices;

  const SynopticPoint({
    required this.topic,
    required this.description,
    this.citationIndices = const [],
  });

  factory SynopticPoint.fromJson(Map<String, dynamic> json) {
    final rawIndices = json['citation_indices'] as List<dynamic>? ?? const [];
    return SynopticPoint(
      topic: json['topic'] as String? ?? '',
      description: json['description'] as String? ?? '',
      citationIndices: rawIndices
          .whereType<num>()
          .map((e) => e.toInt())
          .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'topic': topic,
      'description': description,
      'citation_indices': citationIndices,
    };
  }
}

class SynopticResult {
  final String summary;
  final List<SynopticPoint> consensusPoints;
  final List<SynopticPoint> divergencePoints;
  final List<SynopticCitation> citations;
  final int booksCovered;

  const SynopticResult({
    required this.summary,
    this.consensusPoints = const [],
    this.divergencePoints = const [],
    this.citations = const [],
    this.booksCovered = 0,
  });

  factory SynopticResult.fromJson(Map<String, dynamic> json) {
    return SynopticResult(
      summary: json['summary'] as String? ?? '',
      consensusPoints: (json['consensus_points'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => SynopticPoint.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      divergencePoints: (json['divergence_points'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => SynopticPoint.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      citations: (json['citations'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => SynopticCitation.fromJson(Map<String, dynamic>.from(e)))
          .toList(),
      booksCovered: (json['books_covered'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'summary': summary,
      'consensus_points': consensusPoints.map((p) => p.toJson()).toList(),
      'divergence_points': divergencePoints.map((p) => p.toJson()).toList(),
      'citations': citations.map((c) => c.toJson()).toList(),
      'books_covered': booksCovered,
    };
  }
}
