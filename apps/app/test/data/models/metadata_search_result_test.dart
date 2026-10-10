import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/metadata_search_result.dart';

void main() {
  group('MetadataSearchResult Tests', () {
    test('fromJson and toJson round-trip', () {
      final jsonMap = {
        'provider': 'openlibrary',
        'title': 'Dune',
        'author': 'Frank Herbert',
        'series': 'Dune Chronicles',
        'series_sequence': '1',
        'description': 'A science fiction masterpiece.',
        'cover_url': 'https://covers.openlibrary.org/b/id/12345-L.jpg',
        'publisher': 'Chilton Books',
        'published_year': 1965,
        'isbn': '9780441172719',
        'genres': ['Science Fiction', 'Space Opera'],
      };

      final result = MetadataSearchResult.fromJson(jsonMap);

      expect(result.provider, 'openlibrary');
      expect(result.title, 'Dune');
      expect(result.author, 'Frank Herbert');
      expect(result.series, 'Dune Chronicles');
      expect(result.seriesSequence, '1');
      expect(result.description, 'A science fiction masterpiece.');
      expect(result.coverUrl, 'https://covers.openlibrary.org/b/id/12345-L.jpg');
      expect(result.publisher, 'Chilton Books');
      expect(result.publishedYear, 1965);
      expect(result.isbn, '9780441172719');
      expect(result.genres, ['Science Fiction', 'Space Opera']);

      final serialized = result.toJson();
      expect(serialized['provider'], 'openlibrary');
      expect(serialized['title'], 'Dune');
      expect(serialized['published_year'], 1965);
      expect(serialized['genres'], ['Science Fiction', 'Space Opera']);
    });

    test('fromJson handles minimal and null values gracefully', () {
      final jsonMap = {
        'provider': 'googlebooks',
        'title': 'Minimal Book',
      };

      final result = MetadataSearchResult.fromJson(jsonMap);

      expect(result.provider, 'googlebooks');
      expect(result.title, 'Minimal Book');
      expect(result.author, '');
      expect(result.series, isNull);
      expect(result.seriesSequence, isNull);
      expect(result.description, isNull);
      expect(result.coverUrl, isNull);
      expect(result.publishedYear, isNull);
      expect(result.genres, isEmpty);
    });
  });
}
