import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/data/models/connect_info.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/search_result.dart';
import 'package:shelf/data/models/series.dart';
import 'package:shelf/data/models/user.dart';

void main() {
  group('Data Models Serialization Tests', () {
    test('User fromJson & toJson', () {
      final json = {
        'id': 'usr-123',
        'username': 'benforce',
        'role': 'admin',
      };
      final user = User.fromJson(json);
      expect(user.id, 'usr-123');
      expect(user.username, 'benforce');
      expect(user.role, 'admin');
      expect(user.toJson(), json);
    });

    test('ServerConnectInfo fromJson & toJson', () {
      final json = {
        'server_name': 'Shelfd Homelab',
        'version': '0.1.0',
        'base_url': 'http://192.168.1.100:8080',
        'capabilities': ['mcp', 'semantic_search'],
      };
      final info = ServerConnectInfo.fromJson(json);
      expect(info.serverName, 'Shelfd Homelab');
      expect(info.version, '0.1.0');
      expect(info.baseUrl, 'http://192.168.1.100:8080');
      expect(info.capabilities, contains('mcp'));
      expect(info.capabilities, contains('semantic_search'));
      expect(info.toJson(), json);
    });

    test('Author, Genre, Series fromJson & toJson', () {
      final authorJson = {'id': 'auth-1', 'name': 'Ursula K. Le Guin', 'sort_name': 'Le Guin, Ursula K.'};
      final author = Author.fromJson(authorJson);
      expect(author.id, 'auth-1');
      expect(author.name, 'Ursula K. Le Guin');
      expect(author.sortName, 'Le Guin, Ursula K.');
      expect(author.toJson(), authorJson);

      final genreJson = {'id': 'gen-1', 'name': 'Science Fiction'};
      final genre = Genre.fromJson(genreJson);
      expect(genre.id, 'gen-1');
      expect(genre.name, 'Science Fiction');
      expect(genre.toJson(), genreJson);

      final seriesJson = {'id': 'ser-1', 'name': 'Hainish Cycle', 'book_count': 7};
      final series = Series.fromJson(seriesJson);
      expect(series.id, 'ser-1');
      expect(series.name, 'Hainish Cycle');
      expect(series.bookCount, 7);
      expect(series.toJson(), seriesJson);
    });

    test('Book fromJson with nested authors and series', () {
      final bookJson = {
        'id': 'book-42',
        'title': 'The Left Hand of Darkness',
        'synopsis': 'A classic sci-fi novel about Gethen.',
        'cover_url': '/api/v1/books/book-42/cover',
        'authors': [
          {'id': 'auth-1', 'name': 'Ursula K. Le Guin'}
        ],
        'genres': [
          {'id': 'gen-1', 'name': 'Sci-Fi'}
        ],
        'series': {'id': 'ser-1', 'name': 'Hainish Cycle'},
        'series_sequence': 4.0,
      };

      final book = Book.fromJson(bookJson);
      expect(book.id, 'book-42');
      expect(book.title, 'The Left Hand of Darkness');
      expect(book.synopsis, 'A classic sci-fi novel about Gethen.');
      expect(book.coverUrl, '/api/v1/books/book-42/cover');
      expect(book.authors.length, 1);
      expect(book.authors.first.name, 'Ursula K. Le Guin');
      expect(book.genres.first.name, 'Sci-Fi');
      expect(book.series?.name, 'Hainish Cycle');
      expect(book.seriesSequence, 4.0);
      expect(book.authorDisplay, 'Ursula K. Le Guin');
    });

    test('Chapter fromJson & toJson', () {
      final json = {
        'id': 'chap-1',
        'book_id': 'book-42',
        'chapter_index': 4,
        'title': 'The Archimedes Principle',
        'summary': 'Estraven reflects on the political situation.',
        'content': 'We walked across the snowfields in silence.',
      };
      final chapter = Chapter.fromJson(json);
      expect(chapter.id, 'chap-1');
      expect(chapter.bookId, 'book-42');
      expect(chapter.chapterIndex, 4);
      expect(chapter.title, 'The Archimedes Principle');
      expect(chapter.summary, 'Estraven reflects on the political situation.');
      expect(chapter.content, 'We walked across the snowfields in silence.');
      expect(chapter.toJson(), json);
    });

    test('SemanticSearchHit fromJson & toJson', () {
      final json = {
        'book_id': 'book-42',
        'book_title': 'The Left Hand of Darkness',
        'author_name': 'Ursula K. Le Guin',
        'chapter_index': 14,
        'chapter_title': 'Across the Gobrin Ice',
        'summary': 'Genly Ai and Estraven haul their sledges across the glacier.',
        'score': 0.94,
      };
      final hit = SemanticSearchHit.fromJson(json);
      expect(hit.bookId, 'book-42');
      expect(hit.bookTitle, 'The Left Hand of Darkness');
      expect(hit.authorName, 'Ursula K. Le Guin');
      expect(hit.chapterIndex, 14);
      expect(hit.chapterTitle, 'Across the Gobrin Ice');
      expect(hit.score, 0.94);
      expect(hit.matchPercentage, 94);
      expect(hit.toJson(), json);
    });
  });
}
