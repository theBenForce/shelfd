import 'book.dart';

class DuplicateGroup {
  final String id;
  final String matchReason;
  final double confidence;
  final List<Book> books;

  const DuplicateGroup({
    required this.id,
    required this.matchReason,
    required this.confidence,
    required this.books,
  });

  factory DuplicateGroup.fromJson(Map<String, dynamic> json) {
    final rawBooks = json['books'] as List<dynamic>? ?? [];
    return DuplicateGroup(
      id: json['id'] as String? ?? '',
      matchReason: json['match_reason'] as String? ?? 'Duplicate detected',
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0.8,
      books: rawBooks
          .map((b) => Book.fromJson(b as Map<String, dynamic>))
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'match_reason': matchReason,
        'confidence': confidence,
        'books': books.map((b) => b.toJson()).toList(),
      };
}

class DuplicateScanResult {
  final List<DuplicateGroup> groups;
  final int totalGroups;
  final int totalDuplicateBooks;

  const DuplicateScanResult({
    required this.groups,
    required this.totalGroups,
    required this.totalDuplicateBooks,
  });

  factory DuplicateScanResult.fromJson(Map<String, dynamic> json) {
    final rawGroups = json['groups'] as List<dynamic>? ?? [];
    return DuplicateScanResult(
      groups: rawGroups
          .map((g) => DuplicateGroup.fromJson(g as Map<String, dynamic>))
          .toList(),
      totalGroups: json['total_groups'] as int? ?? rawGroups.length,
      totalDuplicateBooks: json['total_duplicate_books'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
        'groups': groups.map((g) => g.toJson()).toList(),
        'total_groups': totalGroups,
        'total_duplicate_books': totalDuplicateBooks,
      };
}

class MergeOptions {
  final bool transferBookmarks;
  final bool transferHighlights;
  final bool mergeMetadata;
  final bool deleteFiles;

  const MergeOptions({
    this.transferBookmarks = true,
    this.transferHighlights = true,
    this.mergeMetadata = true,
    this.deleteFiles = true,
  });

  Map<String, dynamic> toJson() => {
        'transfer_bookmarks': transferBookmarks,
        'transfer_highlights': transferHighlights,
        'merge_metadata': mergeMetadata,
        'delete_files': deleteFiles,
      };
}
