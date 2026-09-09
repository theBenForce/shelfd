class StagedMetadata {
  final String title;
  final List<String> authors;
  final String? series;
  final double? sequenceNumber;
  final String? description;
  final String? publisher;
  final String? language;
  final List<String> genres;

  const StagedMetadata({
    required this.title,
    this.authors = const [],
    this.series,
    this.sequenceNumber,
    this.description,
    this.publisher,
    this.language,
    this.genres = const [],
  });

  String get primaryAuthor {
    if (authors.isEmpty || authors.first.trim().isEmpty) {
      return 'Unknown';
    }
    return authors.first.trim();
  }

  static String sanitizePathSegment(String name) {
    final sanitized = name.replaceAll(RegExp(r'[/\\:*?"<>|]'), '_').trim();
    return sanitized.isEmpty ? 'Unknown' : sanitized;
  }

  String get destinationPathPreview {
    final authorSegment = sanitizePathSegment(primaryAuthor);
    final titleSegment = sanitizePathSegment(title.trim().isEmpty ? 'Untitled' : title.trim());
    return '/library/$authorSegment/$titleSegment/$titleSegment.epub';
  }

  StagedMetadata copyWith({
    String? title,
    List<String>? authors,
    String? series,
    double? sequenceNumber,
    String? description,
    String? publisher,
    String? language,
    List<String>? genres,
  }) {
    return StagedMetadata(
      title: title ?? this.title,
      authors: authors ?? this.authors,
      series: series ?? this.series,
      sequenceNumber: sequenceNumber ?? this.sequenceNumber,
      description: description ?? this.description,
      publisher: publisher ?? this.publisher,
      language: language ?? this.language,
      genres: genres ?? this.genres,
    );
  }

  factory StagedMetadata.fromJson(Map<String, dynamic> json) {
    final rawAuthors = json['authors'] as List<dynamic>? ?? [];
    final rawGenres = json['genres'] as List<dynamic>? ?? [];

    List<String> parsedAuthors = rawAuthors.map((a) => a.toString()).toList();
    if (parsedAuthors.isEmpty && json['author'] != null && json['author'].toString().trim().isNotEmpty) {
      parsedAuthors = [json['author'].toString().trim()];
    }
    if (parsedAuthors.isEmpty) {
      parsedAuthors = ['Unknown'];
    }

    return StagedMetadata(
      title: json['title'] as String? ?? 'Untitled',
      authors: parsedAuthors,
      series: json['series'] as String?,
      sequenceNumber: (json['sequence_number'] as num?)?.toDouble(),
      description: json['description'] as String?,
      publisher: json['publisher'] as String?,
      language: json['language'] as String?,
      genres: rawGenres.map((g) => g.toString()).toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'title': title.trim().isEmpty ? 'Untitled' : title.trim(),
      'authors': authors.isEmpty ? ['Unknown'] : authors,
      'author': primaryAuthor,
      if (series != null && series!.trim().isNotEmpty) 'series': series!.trim(),
      if (sequenceNumber != null) 'sequence_number': sequenceNumber,
      if (description != null && description!.trim().isNotEmpty) 'description': description!.trim(),
      if (publisher != null && publisher!.trim().isNotEmpty) 'publisher': publisher!.trim(),
      if (language != null && language!.trim().isNotEmpty) 'language': language!.trim(),
      if (genres.isNotEmpty) 'genres': genres,
    };
  }
}

class StagedUploadJob {
  final String jobId;
  final String status;
  final String filename;
  final StagedMetadata metadata;
  final bool hasCover;
  final List<String> warnings;
  final DateTime? createdAt;

  const StagedUploadJob({
    required this.jobId,
    required this.status,
    required this.filename,
    required this.metadata,
    this.hasCover = false,
    this.warnings = const [],
    this.createdAt,
  });

  factory StagedUploadJob.fromJson(Map<String, dynamic> json) {
    final rawWarnings = json['warnings'] as List<dynamic>? ?? [];
    final metaJson = (json['metadata'] as Map<String, dynamic>?) ?? {};

    DateTime? created;
    if (json['created_at'] != null) {
      created = DateTime.tryParse(json['created_at'].toString());
    }

    return StagedUploadJob(
      jobId: json['job_id'] as String? ?? json['id'] as String? ?? '',
      status: json['status'] as String? ?? 'staged',
      filename: json['filename'] as String? ?? '',
      metadata: StagedMetadata.fromJson(metaJson),
      hasCover: json['has_cover'] as bool? ?? false,
      warnings: rawWarnings.map((w) => w.toString()).toList(),
      createdAt: created,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'job_id': jobId,
      'status': status,
      'filename': filename,
      'metadata': metadata.toJson(),
      'has_cover': hasCover,
      'warnings': warnings,
      if (createdAt != null) 'created_at': createdAt!.toIso8601String(),
    };
  }
}
