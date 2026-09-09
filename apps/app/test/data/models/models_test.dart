import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/book_chat.dart';
import 'package:shelf/data/models/bookmark.dart';
import 'package:shelf/data/models/chapter.dart';
import 'package:shelf/data/models/connect_info.dart';
import 'package:shelf/data/models/genre.dart';
import 'package:shelf/data/models/highlight.dart';
import 'package:shelf/data/models/queue_status.dart';
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
        'spine': [
          {'id': '01J7SPINE1', 'book_id': 'book-42', 'chapter_index': 1, 'title': 'Prologue'}
        ],
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
      expect(book.spine.length, 1);
      expect(book.spine.first.id, '01J7SPINE1');
      expect(book.spine.first.title, 'Prologue');
    });

    test('SpineItem fromJson & toJson', () {
      final json = {
        'id': '01J7SPINE1',
        'book_id': 'book-42',
        'chapter_index': 1,
        'title': 'Prologue',
      };
      final item = SpineItem.fromJson(json);
      expect(item.id, '01J7SPINE1');
      expect(item.bookId, 'book-42');
      expect(item.chapterIndex, 1);
      expect(item.title, 'Prologue');
      expect(item.toJson(), json);
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

    test('Chapter fromJson with content_plain key', () {
      final json = {
        'id': 'chap-2',
        'book_id': 'book-42',
        'chapter_index': 5,
        'title': 'Chapter 5',
        'summary': '',
        'content_plain': 'Plain text content from server API.',
      };
      final chapter = Chapter.fromJson(json);
      expect(chapter.id, 'chap-2');
      expect(chapter.content, 'Plain text content from server API.');
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

    test('QueueStatus fromJson & toJson', () {
      final json = {
        'total_chapters': 100,
        'indexed_chapters': 40,
        'pending_chapters': 60,
        'pending_uploads': 2,
        'progress_percent': 40.0,
        'is_active': true,
        'current_book': 'Hyperion',
        'current_chapter': 'The Priest\'s Tale',
      };
      final status = QueueStatus.fromJson(json);
      expect(status.totalChapters, 100);
      expect(status.indexedChapters, 40);
      expect(status.pendingChapters, 60);
      expect(status.pendingUploads, 2);
      expect(status.progressPercent, 40.0);
      expect(status.isActive, isTrue);
      expect(status.currentBook, 'Hyperion');
      expect(status.currentChapter, 'The Priest\'s Tale');
      expect(status.toJson(), json);
    });

    test('Bookmark fromJson & toJson', () {
      final json = {
        'id': 'bm-1',
        'book_id': 'book-42',
        'chapter_id': 'chap-1',
        'title': 'Chapter 4',
        'progress': 0.35,
        'created_at': '2026-09-08T12:00:00.000Z',
      };
      final bm = Bookmark.fromJson(json);
      expect(bm.id, 'bm-1');
      expect(bm.bookId, 'book-42');
      expect(bm.chapterId, 'chap-1');
      expect(bm.title, 'Chapter 4');
      expect(bm.progress, 0.35);
      expect(bm.toJson()['id'], 'bm-1');
      expect(bm.toJson()['progress'], 0.35);
    });

    test('Highlight fromJson & toJson', () {
      final json = {
        'id': 'hl-1',
        'book_id': 'book-42',
        'chapter_id': 'chap-2',
        'selected_text': 'To be or not to be.',
        'note': 'Hamlet soliloquy',
        'color': 'blue',
        'start_offset': 100,
        'end_offset': 120,
        'start_paragraph': 2,
        'end_paragraph': 3,
        'location': 'chap-2:100-120',
        'created_at': '2026-09-08T12:00:00.000Z',
      };
      final hl = Highlight.fromJson(json);
      expect(hl.id, 'hl-1');
      expect(hl.bookId, 'book-42');
      expect(hl.selectedText, 'To be or not to be.');
      expect(hl.note, 'Hamlet soliloquy');
      expect(hl.color, 'blue');
      expect(hl.startOffset, 100);
      expect(hl.endOffset, 120);
      expect(hl.startParagraph, 2);
      expect(hl.endParagraph, 3);
      expect(hl.location, 'chap-2:100-120');
      expect(hl.highlightColor, KindleHighlightColor.blue);
      expect(hl.toJson()['selected_text'], 'To be or not to be.');
      expect(hl.toJson()['color'], 'blue');
      expect(hl.toJson()['start_offset'], 100);
      expect(hl.toJson()['start_paragraph'], 2);
      expect(hl.toJson()['end_paragraph'], 3);
    });

    test('BookCitation, BookChatMessage, BookChatResponse fromJson & toJson', () {
      final citationJson = {
        'chapter_id': 'chap-1',
        'chapter_index': 1,
        'chapter_title': 'Loomings',
        'summary': 'Ishmael travels to New Bedford.',
      };
      final citation = BookCitation.fromJson(citationJson);
      expect(citation.chapterId, 'chap-1');
      expect(citation.chapterIndex, 1);
      expect(citation.chapterTitle, 'Loomings');
      expect(citation.summary, 'Ishmael travels to New Bedford.');
      expect(citation.toJson(), citationJson);

      final msg = BookChatMessage(
        role: 'assistant',
        content: 'Ishmael goes to sea because he feels restless.',
        citations: [citation],
      );
      expect(msg.role, 'assistant');
      expect(msg.citations.length, 1);
      expect(msg.toJson()['role'], 'assistant');
      expect(msg.toJson()['citations'], isNotEmpty);

      final chatRes = BookChatResponse.fromJson({
        'reply': 'This is the reply.',
        'citations': [citationJson],
      });
      expect(chatRes.reply, 'This is the reply.');
      expect(chatRes.citations.length, 1);
    });

    test('Book fromJson with bookmarks and highlights', () {
      final bookJson = {
        'id': 'book-42',
        'title': 'The Left Hand of Darkness',
        'bookmarks': [
          {'id': 'bm-1', 'book_id': 'book-42', 'title': 'My Mark', 'progress': 0.5}
        ],
        'highlights': [
          {'id': 'hl-1', 'book_id': 'book-42', 'selected_text': 'Great quote', 'color': 'yellow'}
        ],
        'publisher': 'Ace Books',
        'published_date': '1969',
        'language': 'en',
        'file_size_bytes': 1024000,
      };

      final book = Book.fromJson(bookJson);
      expect(book.bookmarks.length, 1);
      expect(book.bookmarks.first.title, 'My Mark');
      expect(book.highlights.length, 1);
      expect(book.highlights.first.selectedText, 'Great quote');
      expect(book.publisher, 'Ace Books');
      expect(book.publishedDate, '1969');
      expect(book.language, 'en');
      expect(book.fileSizeBytes, 1024000);
      expect(book.toJson()['publisher'], 'Ace Books');
    });
  });
}
