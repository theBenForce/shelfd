import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/book_chat.dart';

void main() {
  group('BookCitation Tests', () {
    test('fromJson and toJson with paragraph range and excerpt', () {
      final json = {
        'chapter_id': 'chap-42',
        'chapter_index': 3,
        'chapter_title': 'The Spouter-Inn',
        'start_paragraph': 5,
        'end_paragraph': 12,
        'excerpt': 'Entering that gable-ended Spouter-Inn, you found yourself in a wide, low, straggling entry...',
        'summary': 'Ishmael visits the Spouter-Inn in New Bedford.',
      };

      final citation = BookCitation.fromJson(json);

      expect(citation.chapterId, 'chap-42');
      expect(citation.chapterIndex, 3);
      expect(citation.chapterTitle, 'The Spouter-Inn');
      expect(citation.startParagraph, 5);
      expect(citation.endParagraph, 12);
      expect(citation.excerpt, startsWith('Entering that gable-ended'));
      expect(citation.summary, 'Ishmael visits the Spouter-Inn in New Bedford.');

      final outputJson = citation.toJson();
      expect(outputJson['chapter_id'], 'chap-42');
      expect(outputJson['chapter_index'], 3);
      expect(outputJson['chapter_title'], 'The Spouter-Inn');
      expect(outputJson['start_paragraph'], 5);
      expect(outputJson['end_paragraph'], 12);
      expect(outputJson['excerpt'], startsWith('Entering that gable-ended'));
      expect(outputJson['summary'], 'Ishmael visits the Spouter-Inn in New Bedford.');
    });

    test('fromJson fallback excerpt to summary when summary is missing', () {
      final json = {
        'chapter_id': 'chap-1',
        'chapter_index': 1,
        'excerpt': 'Call me Ishmael.',
      };

      final citation = BookCitation.fromJson(json);
      expect(citation.summary, 'Call me Ishmael.');
      expect(citation.excerpt, 'Call me Ishmael.');
      expect(citation.startParagraph, isNull);
      expect(citation.endParagraph, isNull);
    });

    test('copyWith creates modified citation', () {
      const citation = BookCitation(
        chapterId: 'chap-1',
        chapterIndex: 1,
        chapterTitle: 'Loomings',
        startParagraph: 1,
        endParagraph: 3,
        excerpt: 'Call me Ishmael.',
        summary: 'Opening chapter.',
      );

      final updated = citation.copyWith(
        startParagraph: 2,
        endParagraph: 5,
        chapterTitle: 'Chapter 1: Loomings',
      );

      expect(updated.chapterId, 'chap-1');
      expect(updated.chapterIndex, 1);
      expect(updated.chapterTitle, 'Chapter 1: Loomings');
      expect(updated.startParagraph, 2);
      expect(updated.endParagraph, 5);
      expect(updated.excerpt, 'Call me Ishmael.');
      expect(updated.summary, 'Opening chapter.');
    });
  });

  group('BookChatResponse Tests', () {
    test('fromJson parses reply field', () {
      final json = {
        'reply': 'Ishmael travels to Nantucket.',
        'citations': [
          {
            'chapter_id': 'chap-2',
            'chapter_index': 2,
            'summary': 'The Carpet-Bag',
          }
        ],
      };

      final response = BookChatResponse.fromJson(json);
      expect(response.reply, 'Ishmael travels to Nantucket.');
      expect(response.citations.length, 1);
      expect(response.citations.first.chapterId, 'chap-2');
    });

    test('fromJson fallback to legacy response key', () {
      final json = {
        'response': 'Legacy response text.',
        'citations': [],
      };

      final response = BookChatResponse.fromJson(json);
      expect(response.reply, 'Legacy response text.');
      expect(response.citations, isEmpty);
    });
  });

  group('BookChatMessage Tests', () {
    test('toJson and fromJson preserve message attributes', () {
      final now = DateTime.now();
      final msg = BookChatMessage(
        role: 'assistant',
        content: 'Here is the answer.',
        citations: [
          const BookCitation(
            chapterId: 'chap-3',
            chapterIndex: 3,
            startParagraph: 1,
            endParagraph: 2,
            summary: 'Summary text',
          ),
        ],
        timestamp: now,
      );

      final json = msg.toJson();
      expect(json['role'], 'assistant');
      expect(json['content'], 'Here is the answer.');
      expect(json['citations'], hasLength(1));

      final restored = BookChatMessage.fromJson(json);
      expect(restored.role, 'assistant');
      expect(restored.content, 'Here is the answer.');
      expect(restored.citations.length, 1);
      expect(restored.citations.first.startParagraph, 1);
      expect(restored.citations.first.endParagraph, 2);
    });
  });
}
