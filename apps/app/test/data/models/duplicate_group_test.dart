import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/duplicate_group.dart';

void main() {
  group('DuplicateGroup and DuplicateScanResult Models', () {
    test('DuplicateGroup fromJson & toJson', () {
      final json = {
        'id': 'dup_123',
        'match_reason': 'Matching Identifier (ISBN)',
        'confidence': 1.0,
        'books': [
          {
            'id': 'book_1',
            'title': 'Dune',
            'file_size_bytes': 1024000,
            'authors': [{'id': 'a1', 'name': 'Frank Herbert'}],
          },
          {
            'id': 'book_2',
            'title': 'Dune (Special Edition)',
            'file_size_bytes': 1048576,
            'authors': [{'id': 'a1', 'name': 'Frank Herbert'}],
          }
        ],
      };

      final group = DuplicateGroup.fromJson(json);
      expect(group.id, 'dup_123');
      expect(group.matchReason, 'Matching Identifier (ISBN)');
      expect(group.confidence, 1.0);
      expect(group.books.length, 2);
      expect(group.books[0].title, 'Dune');
      expect(group.books[1].title, 'Dune (Special Edition)');

      final outJson = group.toJson();
      expect(outJson['id'], 'dup_123');
      expect(outJson['match_reason'], 'Matching Identifier (ISBN)');
      expect(outJson['confidence'], 1.0);
      expect((outJson['books'] as List).length, 2);
    });

    test('DuplicateScanResult fromJson & toJson', () {
      final json = {
        'total_groups': 1,
        'total_duplicate_books': 2,
        'groups': [
          {
            'id': 'dup_1',
            'match_reason': 'Identical Title & Author',
            'confidence': 0.95,
            'books': [
              {
                'id': 'b1',
                'title': 'Foundation',
                'authors': [{'id': 'a1', 'name': 'Isaac Asimov'}],
              },
              {
                'id': 'b2',
                'title': 'Foundation',
                'authors': [{'id': 'a1', 'name': 'Isaac Asimov'}],
              }
            ],
          }
        ],
      };

      final result = DuplicateScanResult.fromJson(json);
      expect(result.totalGroups, 1);
      expect(result.totalDuplicateBooks, 2);
      expect(result.groups.length, 1);
      expect(result.groups[0].books.length, 2);

      final out = result.toJson();
      expect(out['total_groups'], 1);
      expect(out['total_duplicate_books'], 2);
      expect((out['groups'] as List).length, 1);
    });

    test('MergeOptions toJson returns correct boolean flags', () {
      const options = MergeOptions(
        transferBookmarks: true,
        transferHighlights: true,
        mergeMetadata: true,
        deleteFiles: true,
      );

      final json = options.toJson();
      expect(json['transfer_bookmarks'], true);
      expect(json['transfer_highlights'], true);
      expect(json['merge_metadata'], true);
      expect(json['delete_files'], true);
    });
  });
}
