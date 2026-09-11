import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/topic.dart';
import 'package:shelf/ui/features/library/library_query_parser.dart';

void main() {
  group('ParsedLibraryQuery parsing', () {
    test('parses plain title query', () {
      final parsed = ParsedLibraryQuery.parse('The Bullet Journal Method');
      expect(parsed.titleQuery, 'The Bullet Journal Method');
      expect(parsed.authorQuery, isNull);
      expect(parsed.genreQuery, isNull);
      expect(parsed.topicQuery, isNull);
      expect(parsed.hasFilter, isTrue);
    });

    test('parses Author:"Ryder Carrol" with quotes', () {
      final parsed = ParsedLibraryQuery.parse('Author:"Ryder Carrol"');
      expect(parsed.titleQuery, '');
      expect(parsed.authorQuery, 'Ryder Carrol');
      expect(parsed.hasFilter, isTrue);
    });

    test('parses author without quotes and lowercase', () {
      final parsed = ParsedLibraryQuery.parse('author:Herbert');
      expect(parsed.titleQuery, '');
      expect(parsed.authorQuery, 'Herbert');
    });

    test('parses author with spaces after colon', () {
      final parsed = ParsedLibraryQuery.parse('Author: "Ryder Carrol"');
      expect(parsed.authorQuery, 'Ryder Carrol');
      expect(parsed.titleQuery, '');
    });

    test('parses combined title and author', () {
      final parsed = ParsedLibraryQuery.parse('Bullet Journal Author:"Ryder Carrol"');
      expect(parsed.titleQuery, 'Bullet Journal');
      expect(parsed.authorQuery, 'Ryder Carrol');
    });

    test('parses author, genre, and topic combined', () {
      final parsed = ParsedLibraryQuery.parse(
        'Dune Author:"Frank Herbert" Genre:"Science Fiction" Topic:"Ecology"',
      );
      expect(parsed.titleQuery, 'Dune');
      expect(parsed.authorQuery, 'Frank Herbert');
      expect(parsed.genreQuery, 'Science Fiction');
      expect(parsed.topicQuery, 'Ecology');
    });

    test('handles empty and whitespace-only queries', () {
      final parsed = ParsedLibraryQuery.parse('   ');
      expect(parsed.titleQuery, '');
      expect(parsed.authorQuery, isNull);
      expect(parsed.hasFilter, isFalse);
    });
  });

  group('ParsedLibraryQuery.matchesBook', () {
    const book = Book(
      id: 'b1',
      title: 'The Bullet Journal Method',
      synopsis: 'Track the past, order the present, design the future.',
      authors: [Author(id: 'a1', name: 'Ryder Carroll')],
      genres: [Genre(id: 'g1', name: 'Self-Help')],
      topics: [Topic(id: 't1', name: 'Productivity')],
    );

    test('matches title substring', () {
      final query = ParsedLibraryQuery.parse('Bullet');
      expect(query.matchesBook(book), isTrue);

      final nonMatch = ParsedLibraryQuery.parse('Neuromancer');
      expect(nonMatch.matchesBook(book), isFalse);
    });

    test('matches author name including slight variations like Carrol -> Carroll', () {
      final query = ParsedLibraryQuery.parse('Author:"Ryder Carrol"');
      expect(query.matchesBook(book), isTrue);

      final exact = ParsedLibraryQuery.parse('author:"Ryder Carroll"');
      expect(exact.matchesBook(book), isTrue);

      final nonMatch = ParsedLibraryQuery.parse('Author:"William Gibson"');
      expect(nonMatch.matchesBook(book), isFalse);
    });

    test('matches genre and topic', () {
      final genreQuery = ParsedLibraryQuery.parse('Genre:"Self-Help"');
      expect(genreQuery.matchesBook(book), isTrue);

      final topicQuery = ParsedLibraryQuery.parse('Topic:"Productivity"');
      expect(topicQuery.matchesBook(book), isTrue);

      final combined = ParsedLibraryQuery.parse(
        'Bullet Author:"Ryder" Genre:"Self-Help" Topic:"Productivity"',
      );
      expect(combined.matchesBook(book), isTrue);

      final mismatchedTopic = ParsedLibraryQuery.parse('Topic:"Cooking"');
      expect(mismatchedTopic.matchesBook(book), isFalse);
    });
  });

  group('detectAutocompletePrefix', () {
    test('detects author: at end of input', () {
      final match = ParsedLibraryQuery.detectAutocompletePrefix('author:', 7);
      expect(match, isNotNull);
      expect(match!.tag, 'author');
      expect(match.prefix, '');
    });

    test('detects author:Ry with partial prefix', () {
      final match = ParsedLibraryQuery.detectAutocompletePrefix('author:Ry', 9);
      expect(match, isNotNull);
      expect(match!.tag, 'author');
      expect(match.prefix, 'Ry');
    });

    test('detects Author:"Ryder with opening quote', () {
      final match = ParsedLibraryQuery.detectAutocompletePrefix('Author:"Ryder', 13);
      expect(match, isNotNull);
      expect(match!.tag, 'author');
      expect(match.prefix, 'Ryder');
    });

    test('detects genre: and topic:', () {
      final genreMatch = ParsedLibraryQuery.detectAutocompletePrefix('genre:sci', 9);
      expect(genreMatch, isNotNull);
      expect(genreMatch!.tag, 'genre');
      expect(genreMatch.prefix, 'sci');

      final topicMatch = ParsedLibraryQuery.detectAutocompletePrefix('topic:prod', 10);
      expect(topicMatch, isNotNull);
      expect(topicMatch!.tag, 'topic');
      expect(topicMatch.prefix, 'prod');
    });

    test('returns null when not typing a tag', () {
      final match = ParsedLibraryQuery.detectAutocompletePrefix('Dune Frank Herbert', 18);
      expect(match, isNull);
    });
  });
}
