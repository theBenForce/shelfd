class AudioChapter {
  final String id;
  final String bookId;
  final int chapterIndex;
  final String title;
  final double startOffsetSec;
  final double durationSec;

  const AudioChapter({
    required this.id,
    required this.bookId,
    required this.chapterIndex,
    required this.title,
    required this.startOffsetSec,
    required this.durationSec,
  });

  factory AudioChapter.fromJson(Map<String, dynamic> json) {
    return AudioChapter(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      title: json['title'] as String? ?? '',
      startOffsetSec: (json['start_offset_sec'] as num?)?.toDouble() ?? 0.0,
      durationSec: (json['duration_sec'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      'chapter_index': chapterIndex,
      'title': title,
      'start_offset_sec': startOffsetSec,
      'duration_sec': durationSec,
    };
  }
}

class BookFile {
  final String id;
  final String bookId;
  final String fileType;
  final String filePath;
  final int? fileSizeBytes;
  final double? durationSeconds;
  final int? bitrateKbps;
  final int? pageCount;
  final String? mimeType;
  final DateTime? fileModifiedAt;
  final DateTime? createdAt;

  const BookFile({
    required this.id,
    required this.bookId,
    required this.fileType,
    required this.filePath,
    this.fileSizeBytes,
    this.durationSeconds,
    this.bitrateKbps,
    this.pageCount,
    this.mimeType,
    this.fileModifiedAt,
    this.createdAt,
  });

  factory BookFile.fromJson(Map<String, dynamic> json) {
    return BookFile(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      fileType: json['file_type'] as String? ?? 'ebook',
      filePath: json['file_path'] as String? ?? '',
      fileSizeBytes: (json['file_size_bytes'] as num?)?.toInt(),
      durationSeconds: (json['duration_seconds'] as num?)?.toDouble(),
      bitrateKbps: (json['bitrate_kbps'] as num?)?.toInt(),
      pageCount: (json['page_count'] as num?)?.toInt(),
      mimeType: json['mime_type'] as String?,
      fileModifiedAt: json['file_modified_at'] != null
          ? DateTime.tryParse(json['file_modified_at'] as String)
          : null,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      'file_type': fileType,
      'file_path': filePath,
      if (fileSizeBytes != null) 'file_size_bytes': fileSizeBytes,
      if (durationSeconds != null) 'duration_seconds': durationSeconds,
      if (bitrateKbps != null) 'bitrate_kbps': bitrateKbps,
      if (pageCount != null) 'page_count': pageCount,
      if (mimeType != null) 'mime_type': mimeType,
      if (fileModifiedAt != null)
        'file_modified_at': fileModifiedAt!.toIso8601String(),
      if (createdAt != null) 'created_at': createdAt!.toIso8601String(),
    };
  }
}

class AudiobookProgress {
  final String id;
  final String bookId;
  final String userId;
  final double positionSeconds;
  final double speed;
  final bool isCompleted;
  final DateTime? updatedAt;

  const AudiobookProgress({
    this.id = '',
    required this.bookId,
    this.userId = '',
    required this.positionSeconds,
    this.speed = 1.0,
    this.isCompleted = false,
    this.updatedAt,
  });

  factory AudiobookProgress.fromJson(Map<String, dynamic> json) {
    return AudiobookProgress(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      userId: json['user_id'] as String? ?? '',
      positionSeconds: (json['position_seconds'] as num?)?.toDouble() ?? 0.0,
      speed: (json['speed'] as num?)?.toDouble() ?? 1.0,
      isCompleted: json['is_completed'] as bool? ?? false,
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      'user_id': userId,
      'position_seconds': positionSeconds,
      'speed': speed,
      'is_completed': isCompleted,
      if (updatedAt != null) 'updated_at': updatedAt!.toIso8601String(),
    };
  }
}
